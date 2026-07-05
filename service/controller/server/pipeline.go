package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/redis/go-redis/v9"
	"trpc.group/trpc-go/trpc-go/client"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/entity"
	"echat/sdk/message"
	"echat/sdk/mysql"
	gwpb "echat/service/gateway/stub"
)

// Pipeline 消息处理流水线。
type Pipeline struct {
	idGen       *IDGen
	redis       *redis.Client
	polaris     api.ConsumerAPI
	msgStore    *mysql.MessageStore
	authChecker *mysql.AuthChecker
}

// Process 在 ants worker 内执行完整流水线。
func (p *Pipeline) Process(msg *message.Message) {
	if err := p.authorize(msg); err != nil {
		p.sendACK(msg, &message.ACKResult{Err: err})
		p.tryDelRequestID(msg)
		return
	}

	msg.MsgID = p.idGen.Generate()

	// ServerTime 必须在 saveMessage 之前赋值，saveMessage 内部缓存 ACK 时需要此值
	msg.ServerTime = time.Now().UnixMilli()

	// 持久化失败则中止，ACK 不缓存，客户端靠超时重试
	if !p.saveMessage(msg) {
		p.sendACK(msg, &message.ACKResult{Err: fmt.Errorf("消息持久化失败")})
		p.tryDelRequestID(msg)
		return
	}

	p.sendACK(msg, &message.ACKResult{MsgID: msg.MsgID, SeqID: msg.SeqID, ServerTime: msg.ServerTime})

	p.routeAndForward(msg)
	p.tryDelRequestID(msg)
}

func (p *Pipeline) authorize(msg *message.Message) error {
	switch msg.ChatType {
	case string(message.ChatTypeSingle), "":
		if msg.ToUID == "" {
			return fmt.Errorf("缺少接收者")
		}
		return p.authChecker.CheckFriend(msg.Ctx, msg.FromUID, msg.ToUID)
	case string(message.ChatTypeGroup):
		if msg.GroupID == "" {
			return fmt.Errorf("缺少群 ID")
		}
		return p.authChecker.CheckGroupMember(msg.Ctx, msg.FromUID, msg.GroupID)
	case string(message.ChatTypeAck):
		return nil
	}
	return nil
}

// saveMessage 持久化到 MySQL（返回 false 表示失败，ACK 不缓存）。
func (p *Pipeline) saveMessage(msg *message.Message) bool {
	switch msg.ChatType {
	case string(message.ChatTypeSingle), "":
		pid, err := p.authChecker.EnsurePrivateChat(msg.Ctx, msg.FromUID, msg.ToUID)
		if err != nil {
			log.Errorf("[Controller] EnsurePrivateChat 失败: %v", err)
			return false
		}
		if err := p.msgStore.SavePrivateMessage(msg.Ctx, &entity.PrivateMessage{
			MsgID: msg.MsgID, PID: pid, SeqID: msg.SeqID,
			Content: string(msg.Content), SenderUID: msg.FromUID, Type: messageToEntityType(msg),
		}); err != nil {
			log.Errorf("[Controller] SavePrivateMessage 失败: %v", err)
			return false
		}
	case string(message.ChatTypeGroup):
		if err := p.msgStore.SaveGroupMessage(msg.Ctx, &entity.GroupMessage{
			MsgID: msg.MsgID, GID: msg.GroupID, SeqID: msg.SeqID,
			Content: string(msg.Content), SenderUID: msg.FromUID, Type: messageToEntityType(msg),
		}); err != nil {
			log.Errorf("[Controller] SaveGroupMessage 失败: %v", err)
			return false
		}
	case string(message.ChatTypeAck):
	}

	ackData, err := json.Marshal(map[string]interface{}{
		"msg_id": msg.MsgID, "seq_id": msg.SeqID, "server_time": msg.ServerTime, "success": true,
	})
	if err != nil {
		log.Errorf("[Controller] Marshal ACK 失败: %v", err)
		return false
	}
	p.redis.Set(context.Background(), "ack:"+msg.RequestID, ackData, 60*time.Second)
	return true
}

func (p *Pipeline) routeAndForward(msg *message.Message) {
	var targets []string
	switch msg.ChatType {
	case string(message.ChatTypeSingle), "":
		targets = []string{msg.ToUID}
	case string(message.ChatTypeGroup):
		members, err := p.authChecker.GetGroupMemberUIDs(msg.Ctx, msg.GroupID)
		if err != nil {
			log.Errorf("[Controller] 查群成员失败: gid=%s, err=%v", msg.GroupID, err)
			return
		}
		for _, m := range members {
			if m != msg.FromUID {
				targets = append(targets, m)
			}
		}
	default:
		return
	}
	if len(targets) == 0 {
		return
	}

	gatewayUsers := make(map[string][]string)
	for _, uid := range targets {
		result, err := p.redis.HGetAll(context.Background(), "user_sessions:"+uid).Result()
		if err != nil || len(result) == 0 {
			continue
		}
		for _, raw := range result {
			var info struct{ GatewayID string `json:"gateway_id"` }
			if json.Unmarshal([]byte(raw), &info) == nil {
				gatewayUsers[info.GatewayID] = append(gatewayUsers[info.GatewayID], uid)
			}
		}
	}
	if len(gatewayUsers) == 0 {
		return
	}

	instResp, err := p.polaris.GetAllInstances(&api.GetAllInstancesRequest{
		GetAllInstancesRequest: model.GetAllInstancesRequest{
			Namespace: "default",
			Service:   "echat.gateway.GatewayInternal",
		},
	})
	if err != nil || len(instResp.Instances) == 0 {
		log.Errorf("[Controller] Polaris GetAllInstances 失败: err=%v", err)
		return
	}

	gatewayAddr := make(map[string]string)
	var fallback []string
	for _, inst := range instResp.Instances {
		if !inst.IsHealthy() {
			continue
		}
		addr := fmt.Sprintf("ip://%s:%d", inst.GetHost(), inst.GetPort())
		if gwID := inst.GetMetadata()["gateway_id"]; gwID != "" {
			gatewayAddr[gwID] = addr
		} else {
			fallback = append(fallback, addr)
		}
	}

	for gwID, uids := range gatewayUsers {
		addr, ok := gatewayAddr[gwID]
		if !ok {
			if len(fallback) > 0 {
				log.Warnf("[Controller] gateway_id=%s 无 metadata，退化广播", gwID)
				for _, fb := range fallback {
					p.pushAsync(msg, uids, fb)
				}
			}
			continue
		}
		p.pushAsync(msg, uids, addr)
	}
}

var pushSem = make(chan struct{}, 256)

func (p *Pipeline) pushAsync(msg *message.Message, uids []string, addr string) {
	pushSem <- struct{}{}
	go func() {
		defer func() { <-pushSem }()
		gwCli := gwpb.NewGatewayInternalClientProxy(client.WithTarget(addr))
		_, err := gwCli.PushToUser(context.Background(), &gwpb.PushToUserRequest{
			FromUserId: msg.FromUID, MsgId: msg.MsgID, SeqId: msg.SeqID,
			ServerTime: msg.ServerTime, Content: string(msg.Content), ToUserIds: uids,
		})
		if err != nil {
			log.Errorf("[Controller] PushToUser 失败: addr=%s, err=%v", addr, err)
		}
	}()
}

// sendACK 非阻塞发送 ACK 到 RespCh（Entry 超时后 RespCh 无人读取）。
func (p *Pipeline) sendACK(msg *message.Message, ack *message.ACKResult) {
	select {
	case msg.RespCh <- ack:
	default:
		// Entry 已超时退出，丢弃 ACK
	}
}

// tryDelRequestID 仅在 ctx 未取消时删除 request key。
// ctx 已取消说明 Entry 已超时返回，保留 request key 防止 Gateway 重试时重复落库。
func (p *Pipeline) tryDelRequestID(msg *message.Message) {
	if msg.Ctx.Err() != nil {
		return
	}
	p.redis.Del(context.Background(), "request:"+msg.RequestID)
}

func messageToEntityType(msg *message.Message) entity.MsgType {
	if ct, ok := msg.Ext["content_type"]; ok {
		switch ct {
		case "image":
			return entity.MsgTypeImage
		case "file":
			return entity.MsgTypeFile
		case "audio":
			return entity.MsgTypeVoice
		case "video":
			return entity.MsgTypeVideo
		}
	}
	return entity.MsgTypeText
}

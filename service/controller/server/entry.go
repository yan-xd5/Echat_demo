package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/message"
	"echat/sdk/route"
	ctrlpb "echat/service/controller/stub"
)

// Entry 消息入口：SETNX 去重 + parseMessage + Submit 到协程池。
type Entry struct {
	redis *redis.Client
	pool  *Pool
}

// Handle 处理 Gateway 发来的 RouteMessage 请求。
func (e *Entry) Handle(ctx context.Context, req *ctrlpb.RouteMessageRequest) *ctrlpb.RouteMessageResponse {
	// ① SETNX 去重
	ok, err := e.redis.SetNX(ctx, "request:"+req.RequestId, "processing", 60*time.Second).Result()
	if err != nil {
		log.ErrorContextf(ctx, "[Controller] SETNX 失败: %v", err)
		return &ctrlpb.RouteMessageResponse{Success: false, Reason: "去重检查失败"}
	}
	if !ok {
		cached, _ := e.redis.Get(ctx, "ack:"+req.RequestId).Result()
		if cached != "" {
			var resp ctrlpb.RouteMessageResponse
			json.Unmarshal([]byte(cached), &resp)
			return &resp
		}
		return &ctrlpb.RouteMessageResponse{Success: true, Reason: "重复请求已处理"}
	}

	// ② parseMessage：从 proto 填充 Message
	msg := e.parseMessage(ctx, req)

	// ③ 提取 sessionID + Submit 到协程池
	sessionID := route.ExtractSessionID(req.RawBody, req.FromUserId)
	msg.SessionID = sessionID

	if err := e.pool.Submit(sessionID, msg); err != nil {
		e.redis.Del(context.Background(), "request:"+req.RequestId)
		return &ctrlpb.RouteMessageResponse{Success: false, Reason: err.Error()}
	}

	// ④ 等协程池返回 ACK
	select {
	case ack := <-msg.RespCh:
		if ack.Err != nil {
			return &ctrlpb.RouteMessageResponse{Success: false, Reason: ack.Err.Error()}
		}
		return &ctrlpb.RouteMessageResponse{
			Success:    true,
			MsgId:      ack.MsgID,
			ServerTime: ack.ServerTime,
			Reason:     "消息处理成功",
		}
	case <-ctx.Done():
		// 超时不删除 request key，Gateway 重试时 SETNX 仍然命中，防止重复落库
		return &ctrlpb.RouteMessageResponse{Success: false, Reason: "请求超时"}
	}
}

// parseMessage 从 proto + RawBody 填充 Message。
func (e *Entry) parseMessage(ctx context.Context, req *ctrlpb.RouteMessageRequest) *message.Message {
	msg := &message.Message{
		MsgType:   req.MsgType,
		FromUID:   req.FromUserId,
		SeqID:     req.SeqId,
		RawBody:   req.RawBody,
		RequestID: req.RequestId,
		Ctx:       ctx,
		RespCh:    make(chan *message.ACKResult, 1),
		Ext:       make(map[string]string),
	}

	var raw struct {
		To          string `json:"to"`
		GroupID     string `json:"group_id"`
		Content     string `json:"content"`
		ContentType string `json:"content_type"`
	}
	if json.Unmarshal(req.RawBody, &raw) == nil {
		msg.ToUID = raw.To
		msg.GroupID = raw.GroupID
		msg.Content = []byte(raw.Content)
		if raw.ContentType != "" {
			msg.Ext["content_type"] = raw.ContentType
		}
	}

	switch req.MsgType {
	case "chat", "typing":
		msg.ChatType = string(message.ChatTypeSingle)
		msg.ToUID = raw.To
	case "group_chat":
		msg.ChatType = string(message.ChatTypeGroup)
		msg.GroupID = raw.GroupID
	case "read_receipt", "delivery_ack":
		msg.ChatType = string(message.ChatTypeAck)
	default:
		msg.ChatType = string(message.ChatTypeSingle)
	}

	return msg
}

package main

import (
	"context"
	"encoding/json"

	"trpc.group/trpc-go/trpc-go/log"

	gwpb "echat/service/gateway/stub"
)

// GatewayPushService 实现 GatewayInternal 接口，供 Controller 调用。
type GatewayPushService struct {
	gwpb.UnimplementedGatewayInternal
	gateway *Gateway
}

// PushToUser Controller 推送消息给指定用户。
func (s *GatewayPushService) PushToUser(ctx context.Context, req *gwpb.PushToUserRequest) (*gwpb.PushToUserResponse, error) {
	// 构建下行帧
	frame, _ := json.Marshal(WSResponse{
		Type:       "push",
		From:       req.FromUserId,
		Content:    req.Content,
		MsgID:      req.MsgId,
		SeqID:      req.SeqId,
		ServerTime: req.ServerTime,
	})

	// 确定目标用户列表（兼容旧版单播字段）
	targets := req.ToUserIds
	if len(targets) == 0 && req.UserId != "" {
		targets = []string{req.UserId}
	}

	// 遍历目标用户，非阻塞写入所有设备
	delivered := 0
	for _, uid := range targets {
		chs := s.gateway.connMgr.LookupWriteChs(uid)
		if len(chs) == 0 {
			continue // 用户不在本 Gateway
		}
		for _, ch := range chs {
			select {
			case ch <- frame:
				delivered++
			default:
				log.Warnf("[Gateway] WriteCh 满，消息丢弃: uid=%s", uid)
			}
		}
	}

	if delivered == 0 {
		return &gwpb.PushToUserResponse{Delivered: false, Reason: "用户不在线或缓冲区满"}, nil
	}

	log.InfoContextf(ctx, "[Gateway] 推送: from=%s, targets=%d, delivered=%d", req.FromUserId, len(targets), delivered)
	return &gwpb.PushToUserResponse{Delivered: true, Reason: "投递成功"}, nil
}

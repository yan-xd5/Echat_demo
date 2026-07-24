package handler

import (
	"context"
	"encoding/json"

	"trpc.group/trpc-go/trpc-go/log"

	gwpb "echat/service/gateway/stub"
	"echat/service/gateway/internal/session"
)

type GatewayPushService struct {
	Gateway *session.Gateway
}

func (s *GatewayPushService) PushToUser(ctx context.Context, req *gwpb.PushToUserRequest) (*gwpb.PushToUserResponse, error) {
	// 构造下行推送帧：type="message" 表示服务端推送的新消息
	frame, _ := json.Marshal(session.WSResponse{
		Type:       "message",
		MsgID:      req.MsgId,
		SeqID:      req.SeqId,
		ServerTime: req.ServerTime,
		Content:    req.Content,
		From:       req.FromUserId,
	})

	// 合并单播 + 批量目标用户，去重
	seen := make(map[string]bool)
	var uids []string
	for _, uid := range req.ToUserIds {
		if !seen[uid] {
			seen[uid] = true
			uids = append(uids, uid)
		}
	}
	if req.UserId != "" && !seen[req.UserId] {
		uids = append(uids, req.UserId)
	}

	// 非阻塞写入各用户的所有在线设备
	var delivered int
	for _, uid := range uids {
		for _, ch := range s.Gateway.ConnMgr.LookupWriteChs(uid) {
			select {
			case ch <- frame:
				delivered++
			default:
				// WriteCh 满，丢弃（不阻塞 PushToUser 协程）
			}
		}
	}

	log.Infof("[Gateway] PushToUser: from=%s, targets=%d, delivered=%d", req.FromUserId, len(uids), delivered)
	return &gwpb.PushToUserResponse{Delivered: delivered > 0}, nil
}

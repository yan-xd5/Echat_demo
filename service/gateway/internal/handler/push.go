package handler

import (
	"context"

	"trpc.group/trpc-go/trpc-go/log"

	gwpb "echat/service/gateway/stub"
	"echat/service/gateway/internal/session"
)

type GatewayPushService struct {
	Gateway *session.Gateway
}

func (s *GatewayPushService) PushToUser(ctx context.Context, req *gwpb.PushToUserRequest) (*gwpb.PushToUserResponse, error) {
	for _, uid := range req.ToUserIds {
		for _, ch := range s.Gateway.ConnMgr.LookupWriteChs(uid) {
			_ = ch // TODO: send push frame
		}
	}
	log.Infof("[Gateway] PushToUser: from=%s, targets=%d", req.FromUserId, len(req.ToUserIds))
	return &gwpb.PushToUserResponse{}, nil
}

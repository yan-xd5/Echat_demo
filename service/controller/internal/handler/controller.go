package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"trpc.group/trpc-go/trpc-go/log"

	ctrlpb "echat/service/controller/stub"
	commonpb "echat/proto/common"
	"echat/sdk/usecase/auth"
)

// Impl 实现 ControllerService 接口。
type Impl struct {
	ctrlpb.UnimplementedControllerService
	Entry interface{ Handle(context.Context, *ctrlpb.RouteMessageRequest) *ctrlpb.RouteMessageResponse }
	Redis *redis.Client
}

func New(entry interface{ Handle(context.Context, *ctrlpb.RouteMessageRequest) *ctrlpb.RouteMessageResponse }, rdb *redis.Client) *Impl {
	return &Impl{Entry: entry, Redis: rdb}
}

func (s *Impl) AuthCheck(ctx context.Context, req *ctrlpb.AuthCheckRequest) (*ctrlpb.AuthCheckResponse, error) {
	uid, platform, err := auth.ValidateTicket(req.Token)
	if err != nil {
		return &ctrlpb.AuthCheckResponse{Valid: false, Reason: "Token 无效"}, nil
	}
	log.InfoContextf(ctx, "[Controller] Token 校验通过: uid=%s, platform=%s", uid, platform)
	return &ctrlpb.AuthCheckResponse{Valid: true, UserId: uid, Reason: "校验通过"}, nil
}

func (s *Impl) RouteMessage(ctx context.Context, req *ctrlpb.RouteMessageRequest) (*ctrlpb.RouteMessageResponse, error) {
	return s.Entry.Handle(ctx, req), nil
}

func (s *Impl) UpdateStatus(ctx context.Context, req *ctrlpb.UpdateStatusRequest) (*ctrlpb.UpdateStatusResponse, error) {
	key := fmt.Sprintf("user_status:%s", req.UserId)

	switch req.Status {
	case commonpb.OnlineStatus_ONLINE:
		// 在线状态设 5 分钟 TTL，Gateway 心跳会刷新
		s.Redis.Set(ctx, key, "online", 5*time.Minute)
	case commonpb.OnlineStatus_OFFLINE:
		s.Redis.Del(ctx, key)
	}

	log.InfoContextf(ctx, "[Controller] 状态变更: uid=%s, status=%v, gateway=%s", req.UserId, req.Status, req.GatewayId)
	return &ctrlpb.UpdateStatusResponse{}, nil
}

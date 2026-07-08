package handler

import (
	"context"

	ctrlpb "echat/service/controller/stub"
	"echat/sdk/usecase/auth"
	"trpc.group/trpc-go/trpc-go/log"
)

// Impl 实现 ControllerService 接口。
type Impl struct {
	ctrlpb.UnimplementedControllerService
	Entry interface{ Handle(context.Context, *ctrlpb.RouteMessageRequest) *ctrlpb.RouteMessageResponse }
}

func New(entry interface{ Handle(context.Context, *ctrlpb.RouteMessageRequest) *ctrlpb.RouteMessageResponse }) *Impl {
	return &Impl{Entry: entry}
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
	log.InfoContextf(ctx, "[Controller] 状态变更: uid=%s, status=%v, gateway=%s", req.UserId, req.Status, req.GatewayId)
	return &ctrlpb.UpdateStatusResponse{}, nil
}

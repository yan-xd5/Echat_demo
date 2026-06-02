package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/client"
	"trpc.group/trpc-go/trpc-go/filter"
	"trpc.group/trpc-go/trpc-go/log"

	ctrlpb "echat/service/controller/stub"
	apipb "echat/service/api/stub"
)

// controllerImpl 实现 ControllerService 接口
type controllerImpl struct {
	ctrlpb.UnimplementedControllerService
	apiClient apipb.UserServiceClientProxy // ★ 注入 API 客户端
}

// AuthCheck Token 校验 → ★ 真实调用 API 服务验证用户
func (s *controllerImpl) AuthCheck(ctx context.Context, req *ctrlpb.AuthCheckRequest) (*ctrlpb.AuthCheckResponse, error) {
	log.InfoContextf(ctx, "[中控服务] 收到 Token 校验请求")

	// ★ 调用 API 服务获取用户信息（模拟 Token 解析后得到 uid = "user_001"）
	userResp, err := s.apiClient.GetUserInfo(ctx, &apipb.GetUserInfoRequest{
		Uid: "user_001",
	})
	if err != nil {
		log.ErrorContextf(ctx, "[中控服务] 调用 API 查询用户失败: %v", err)
		return &ctrlpb.AuthCheckResponse{
			Valid:  false,
			Reason: "Token 无效：用户不存在",
		}, nil
	}

	log.InfoContextf(ctx, "[中控服务] API 返回用户: uid=%s, username=%s, account=%s",
		userResp.User.Uid, userResp.User.Username, userResp.User.Account)

	return &ctrlpb.AuthCheckResponse{
		Valid:  true,
		UserId: userResp.User.Uid,
		Reason: "校验通过（API 确认用户存在）",
	}, nil
}

// RouteMessage 消息路由
func (s *controllerImpl) RouteMessage(ctx context.Context, req *ctrlpb.RouteMessageRequest) (*ctrlpb.RouteMessageResponse, error) {
	nickname := GetNickname(ctx)
	log.InfoContextf(ctx, "[中控服务] ========== 开始消息路由 ==========")
	log.InfoContextf(ctx, "[中控服务] 当前用户: %s，发送者: %s → 接收者: %s",
		nickname, req.FromUserId, req.ToUserId)
	log.InfoContextf(ctx, "[中控服务] 消息内容: %s", req.Content)

	// ★ 调用 API 校验接收者是否存在
	_, err := s.apiClient.GetUserInfo(ctx, &apipb.GetUserInfoRequest{
		Uid: req.ToUserId,
	})
	if err != nil {
		log.ErrorContextf(ctx, "[中控服务] 接收者不存在: %s", req.ToUserId)
		return nil, err
	}
	log.InfoContextf(ctx, "[中控服务] 接收者校验通过")

	// 模拟：路由步骤
	log.InfoContextf(ctx, "[中控服务] ① 校验好友关系...通过")
	log.InfoContextf(ctx, "[中控服务] ② 检查黑名单...未拉黑")
	log.InfoContextf(ctx, "[中控服务] ③ 查询目标用户所在 Gateway...gateway-02")
	log.InfoContextf(ctx, "[中控服务] ④ 推送到 Gateway(gateway-02)...投递成功")
	log.InfoContextf(ctx, "[中控服务] ⑤ 离线消息存储检查...目标在线，跳过")
	log.InfoContextf(ctx, "[中控服务] ========== 消息路由完成 ==========")

	return &ctrlpb.RouteMessageResponse{
		Success:    true,
		MsgId:      "MSG_" + req.MsgId,
		ServerTime: time.Now().UnixMilli(),
		Reason:     "消息转发成功",
	}, nil
}

// UpdateStatus 在线状态变更
func (s *controllerImpl) UpdateStatus(ctx context.Context, req *ctrlpb.UpdateStatusRequest) (*ctrlpb.UpdateStatusResponse, error) {
	userID := GetUserID(ctx)
	log.InfoContextf(ctx, "[中控服务] ctx 中用户=%s，请求用户=%s，状态变更为 %v (Gateway: %s)",
		userID, req.UserId, req.Status, req.GatewayId)
	return &ctrlpb.UpdateStatusResponse{}, nil
}

func init() {
	filter.Register("myServerFilter", serverFilter, nil)
}

func main() {
	// ★ 创建 API 客户端
	apiClient := apipb.NewUserServiceClientProxy(
		client.WithTarget("ip://127.0.0.1:8001"),
	)

	s := trpc.NewServer()
	ctrlpb.RegisterControllerServiceService(s, &controllerImpl{
		apiClient: apiClient,
	})

	// 优雅关机
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[中控服务] 收到信号 %v，正在优雅关机...", <-ch)
		s.Close(nil)
	}()

	log.Info("[中控服务] Controller 服务启动中...(Ctrl+C 停止)")
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[中控服务] 已停止")
}

package filter

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-go/filter"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/usecase/auth"
	pb "echat/service/controller/stub"
)

func init() {
	filter.Register("myServerFilter", serverFilter, nil)
}

type ctxKey string

const (
	keyUserID   ctxKey = "user_id"
	keyNickname ctxKey = "nickname"
)

// GetUserID 从 ctx 中提取 user_id。
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(keyUserID).(string); ok {
		return v
	}
	return ""
}

// GetNickname 从 ctx 中提取昵称。
func GetNickname(ctx context.Context) string {
	if v, ok := ctx.Value(keyNickname).(string); ok {
		return v
	}
	return ""
}

func serverFilter(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
	start := time.Now()
	log.InfoContextf(ctx, "[Filter] ========== 拦截请求 ==========")

	switch r := req.(type) {

	case *pb.AuthCheckRequest:
		log.InfoContextf(ctx, "[Filter] 请求类型: AuthCheck（Token 校验）")
		if r.Token == "" {
			log.ErrorContextf(ctx, "[Filter] Token 为空，拒绝请求")
			return nil, fmt.Errorf("token 不能为空")
		}
		claims, err := auth.ParseToken(r.Token)
		if err != nil {
			log.ErrorContextf(ctx, "[Filter] Token 解析失败: %v", err)
			return nil, fmt.Errorf("token 解析失败: %w", err)
		}
		ctx = context.WithValue(ctx, keyUserID, claims.UID)
		ctx = context.WithValue(ctx, keyNickname, claims.Account)
		log.InfoContextf(ctx, "[Filter] Token 解析成功，user_id=%s", claims.UID)

	case *pb.RouteMessageRequest:
		log.InfoContextf(ctx, "[Filter] 请求类型: RouteMessage（消息路由）")
		log.InfoContextf(ctx, "[Filter] 发送者: %s", r.FromUserId)
		if r.FromUserId == "" {
			return nil, fmt.Errorf("发送者不能为空")
		}
		log.InfoContextf(ctx, "[Filter] 发送者身份校验通过")

	case *pb.UpdateStatusRequest:
		log.InfoContextf(ctx, "[Filter] 请求类型: UpdateStatus（状态变更）")
		log.InfoContextf(ctx, "[Filter] 用户: %s，状态: %v，Gateway: %s",
			r.UserId, r.Status, r.GatewayId)
		ctx = context.WithValue(ctx, keyUserID, r.UserId)

	default:
		log.InfoContextf(ctx, "[Filter] 请求类型: 未知")
	}

	rsp, err := next(ctx, req)

	elapsed := time.Since(start)
	if err != nil {
		log.ErrorContextf(ctx, "[Filter] ========== 请求失败，耗时=%v，错误=%v ==========", elapsed, err)
	} else {
		log.InfoContextf(ctx, "[Filter] ========== 请求完成，耗时=%v ==========", elapsed)
	}

	return rsp, err
}

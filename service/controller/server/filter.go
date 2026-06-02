package main

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-go/filter"
	"trpc.group/trpc-go/trpc-go/log"

	pb "echat/service/controller/stub"
)

// ctxKey 是 context 的 key 类型（Go 推荐用自定义类型避免冲突）
type ctxKey string

const (
	keyUserID   ctxKey = "user_id"   // 当前请求的 user_id
	keyNickname ctxKey = "nickname"  // 当前请求的用户昵称
)

// GetUserID 从 ctx 中提取 user_id（供业务方法调用）
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(keyUserID).(string); ok {
		return v
	}
	return ""
}

// GetNickname 从 ctx 中提取昵称（供业务方法调用）
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
		log.InfoContextf(ctx, "[Filter] Token: %s", r.Token)

		if r.Token == "" {
			log.ErrorContextf(ctx, "[Filter] Token 为空，拒绝请求")
			return nil, fmt.Errorf("token 不能为空")
		}

		// ★ 模拟解析 Token，提取用户信息，写入 ctx
		//    真实场景：调用 jwt.Parse() 解析出 user_id、nickname
		userID := "user_001"
		nickname := "张三"
		ctx = context.WithValue(ctx, keyUserID, userID)
		ctx = context.WithValue(ctx, keyNickname, nickname)
		log.InfoContextf(ctx, "[Filter] Token 解析成功，user_id=%s，已写入 ctx", userID)

	case *pb.RouteMessageRequest:
		log.InfoContextf(ctx, "[Filter] 请求类型: RouteMessage（消息路由）")
		log.InfoContextf(ctx, "[Filter] 发送者: %s → 接收者: %s", r.FromUserId, r.ToUserId)
		log.InfoContextf(ctx, "[Filter] 消息内容: %s", r.Content)

		if r.FromUserId == "" {
			return nil, fmt.Errorf("发送者不能为空")
		}

		// ★ 校验：发送者必须与 Token 中的 user_id 一致
		tokenUserID := GetUserID(ctx)
		if tokenUserID != "" && r.FromUserId != tokenUserID {
			log.ErrorContextf(ctx, "[Filter] 身份不匹配！Token 用户=%s，发送者=%s", tokenUserID, r.FromUserId)
			return nil, fmt.Errorf("发送者身份与 Token 不一致")
		}
		log.InfoContextf(ctx, "[Filter] 发送者身份校验通过")

	case *pb.UpdateStatusRequest:
		log.InfoContextf(ctx, "[Filter] 请求类型: UpdateStatus（状态变更）")
		log.InfoContextf(ctx, "[Filter] 用户: %s，状态: %v，Gateway: %s",
			r.UserId, r.Status, r.GatewayId)

		// ★ 把状态变更信息写入 ctx，供后续使用
		ctx = context.WithValue(ctx, keyUserID, r.UserId)

	default:
		log.InfoContextf(ctx, "[Filter] 请求类型: 未知")
	}

	// 把带有用户信息的 ctx 传给业务方法
	rsp, err := next(ctx, req)

	elapsed := time.Since(start)
	if err != nil {
		log.ErrorContextf(ctx, "[Filter] ========== 请求失败，耗时=%v，错误=%v ==========", elapsed, err)
	} else {
		log.InfoContextf(ctx, "[Filter] ========== 请求完成，耗时=%v ==========", elapsed)
	}

	return rsp, err
}

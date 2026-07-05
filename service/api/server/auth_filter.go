package main

import (
	"context"
	"fmt"
	"strings"

	"trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/filter"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/auth"
)

func init() {
	filter.Register("apiAuth", apiAuthFilter, nil)
}

// apiAuthFilter 从 HTTP Authorization header 解析 JWT，注入 uid 到 ctx。
func apiAuthFilter(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
	msg := trpc.Message(ctx)
	if msg == nil {
		return next(ctx, req)
	}

	// 跳过注册和登录（无需认证）
	rpcName := msg.ServerRPCName()
	if strings.Contains(rpcName, "Register") || strings.Contains(rpcName, "Login") || strings.Contains(rpcName, "GetPublicKey") {
		return next(ctx, req)
	}

	// 从 HTTP metadata 提取 Authorization header
	md := msg.ServerMetaData()
	if md != nil {
		for k, v := range md {
			if strings.EqualFold(k, "authorization") {
				token := string(v)
				token = strings.TrimPrefix(token, "Bearer ")
				claims, err := auth.ParseToken(token)
				if err != nil {
					log.Warnf("[API] JWT 验证失败: %v", err)
					return nil, fmt.Errorf("认证失败: %w", err)
				}
				ctx = context.WithValue(ctx, uidKey{}, claims.UID)
				break
			}
		}
	}

	return next(ctx, req)
}

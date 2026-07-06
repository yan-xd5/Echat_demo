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

// skipAuth 返回 true 表示该路由无需认证（注册、登录、公钥分发）。
func skipAuth(rpcName string) bool {
	for _, s := range []string{"Register", "Login", "GetPublicKey"} {
		if strings.Contains(rpcName, s) {
			return true
		}
	}
	return false
}

// apiAuthFilter 从 HTTP Authorization header 解析 JWT，注入 uid 到 ctx。
// 非公开路由强制要求认证，未认证直接拒绝。
func apiAuthFilter(ctx context.Context, req interface{}, next filter.ServerHandleFunc) (interface{}, error) {
	msg := trpc.Message(ctx)
	if msg == nil {
		return next(ctx, req)
	}

	rpcName := msg.ServerRPCName()

	// 从 HTTP metadata 提取 Authorization header
	var uid string
	md := msg.ServerMetaData()
	if md != nil {
		for k, v := range md {
			if strings.EqualFold(k, "authorization") {
				token := string(v)
				token = strings.TrimPrefix(token, "Bearer ")
				claims, err := auth.ParseToken(token)
				if err != nil {
					log.Warnf("[API] JWT 验签失败: method=%s, err=%v", rpcName, err)
					if !skipAuth(rpcName) {
						return nil, fmt.Errorf("认证失败: token 无效")
					}
					break
				}
				uid = claims.UID
				ctx = context.WithValue(ctx, uidKey{}, uid)
				break
			}
		}
	}

	// 非公开路由必须认证
	if !skipAuth(rpcName) && uid == "" {
		return nil, fmt.Errorf("未认证: 请提供 Authorization header")
	}

	return next(ctx, req)
}

package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/auth"
	"echat/sdk/route"
)

// Gateway 聚合所有子模块。
type Gateway struct {
	gatewayID     string
	connMgr       *ConnManager
	sessionRedis  *SessionRedis
	redis         *redis.Client
	sessionRouter *route.SessionRouter
}

// ============================================================
// WebSocket 升级 Handler
// ============================================================

// WSAuthHandler 处理 WebSocket 升级 + Ticket 验证。
type WSAuthHandler struct {
	gateway *Gateway
}

// ServeHTTP 处理 GET /ws?ticket=xxx
func (h *WSAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. 提取 ticket
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		http.Error(w, "缺少 ticket", http.StatusUnauthorized)
		return
	}

	// 2. SDK 本地验证 Ticket（签名+过期）
	uid, platform, err := auth.ValidateTicket(ticket)
	if err != nil {
		log.Errorf("[Gateway] Ticket 验证失败: %v", err)
		http.Error(w, "ticket 无效", http.StatusUnauthorized)
		return
	}

	// 2.1 Redis 黑名单检查（封号/踢人）
	blocked, err := auth.CheckBlacklist(r.Context(), h.gateway.redis, uid)
	if err != nil {
		log.Errorf("[Gateway] 黑名单检查失败: %v", err)
		http.Error(w, "服务不可用", http.StatusServiceUnavailable)
		return
	}
	if blocked {
		log.Warnf("[Gateway] 用户已被封禁，拒绝连接: uid=%s", uid)
		http.Error(w, "账号已被封禁", http.StatusForbidden)
		return
	}

	// 3. 升级 WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// 4. 生成 deviceID
	deviceID := generateDeviceID(h.gateway.gatewayID, ticket)

	// 5. 创建 Session + 顶号 + 注册
	session := NewSession(uid, deviceID, platform, conn)
	if err := KickOldAndRegister(h.gateway.connMgr, uid, deviceID, platform, session); err != nil {
		conn.Close()
		return
	}

	// 5.1 通知其他 Gateway 踢旧连接（跨网关顶号）
	publishKick(r.Context(), h.gateway.redis, h.gateway.gatewayID, uid, platform)

	// 6. 写 Redis 会话路由
	if err := h.gateway.sessionRedis.AddSession(r.Context(), uid, deviceID, &SessionInfo{
		GatewayID:  h.gateway.gatewayID,
		Platform:   platform,
		ConnTime:   time.Now().UnixMilli(),
		LastActive: time.Now().UnixMilli(),
	}); err != nil {
		log.Errorf("[Gateway] Redis AddSession 失败: uid=%s, err=%v", uid, err)
	}

	// 7. 启动 reader + writer
	session.Run(h.gateway)
}

// generateDeviceID 本地生成设备 ID。
func generateDeviceID(gatewayID, ticket string) string {
	hash := sha256.Sum256([]byte(ticket + time.Now().String()))
	return fmt.Sprintf("%s-%x", gatewayID, hash[:4])
}


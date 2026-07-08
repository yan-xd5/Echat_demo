package handler

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"

	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/usecase/auth"
	"echat/service/gateway/internal/session"
)

type WSAuthHandler struct {
	Gateway *session.Gateway
}

func (h *WSAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		http.Error(w, "缺少 ticket", http.StatusUnauthorized)
		return
	}
	uid, _, err := auth.ValidateTicket(ticket)
	if err != nil {
		log.Errorf("[Gateway] Ticket 验证失败: %v", err)
		http.Error(w, "ticket 无效", http.StatusUnauthorized)
		return
	}
	blocked, err := auth.CheckBlacklist(r.Context(), h.Gateway.Redis, uid)
	if err != nil {
		log.Errorf("[Gateway] 黑名单检查失败: %v", err)
		http.Error(w, "服务不可用", http.StatusServiceUnavailable)
		return
	}
	if blocked {
		http.Error(w, "账号已被封禁", http.StatusForbidden)
		return
	}
	conn, err := session.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	deviceID := generateDeviceID(h.Gateway.GatewayID, ticket)
	sess := session.NewSession(uid, deviceID, "web", conn)
	if err := session.KickOldAndRegister(h.Gateway.ConnMgr, uid, deviceID, "web", sess); err != nil {
		conn.Close()
		return
	}
	session.PublishKick(r.Context(), h.Gateway.Redis, h.Gateway.GatewayID, uid, "web")
	if err := h.Gateway.SessionRedis.AddSession(r.Context(), uid, deviceID, &session.SessionInfo{
		GatewayID: h.Gateway.GatewayID, Platform: "web",
		ConnTime: time.Now().UnixMilli(), LastActive: time.Now().UnixMilli(),
	}); err != nil {
		log.Errorf("[Gateway] Redis AddSession 失败: uid=%s, err=%v", uid, err)
		h.Gateway.ConnMgr.Unregister(uid, deviceID)
		conn.Close()
		return
	}
	sess.Run(h.Gateway)
}

func generateDeviceID(gatewayID, ticket string) string {
	hash := sha256.Sum256([]byte(ticket + time.Now().String()))
	return fmt.Sprintf("%s-%x", gatewayID, hash[:4])
}

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"trpc.group/trpc-go/trpc-go/log"

	ctrlpb "echat/service/controller/stub"
)

// ============================================================
// WebSocket 消息帧格式（客户端 ↔ Gateway）
// ============================================================

// WSMessage 客户端发来的消息
type WSMessage struct {
	Type    string `json:"type"`              // auth | chat | ping
	Token   string `json:"token,omitempty"`   // auth 时使用
	To      string `json:"to,omitempty"`      // chat 时的接收者 uid
	Content string `json:"content,omitempty"` // chat 时的消息内容
}

// WSResponse 推送给客户端的消息
type WSResponse struct {
	Type      string `json:"type"`                // auth_ack | msg_ack | push | pong | error
	Ok        bool   `json:"ok,omitempty"`        // auth 结果
	MsgID     string `json:"msg_id,omitempty"`    // 消息 ID
	From      string `json:"from,omitempty"`      // 发送者 uid
	Content   string `json:"content,omitempty"`   // 消息内容
	Error     string `json:"error,omitempty"`     // 错误信息
	ServerTime int64  `json:"server_time,omitempty"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 开发环境允许所有来源
}

// WSServer 处理 WebSocket 连接
type WSServer struct {
	connMgr  *ConnManager
	ctrlCli  ctrlpb.ControllerServiceClientProxy
	gatewayID string
}

// NewWSServer 创建 WebSocket 服务器
func NewWSServer(mgr *ConnManager, ctrlCli ctrlpb.ControllerServiceClientProxy, gwID string) *WSServer {
	return &WSServer{connMgr: mgr, ctrlCli: ctrlCli, gatewayID: gwID}
}

// ServeHTTP 处理 WebSocket 升级请求
func (s *WSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("[Gateway] WebSocket 升级失败: %v", err)
		return
	}
	log.Infof("[Gateway] 新连接接入: %s", r.RemoteAddr)

	go s.handleConn(conn)
}

// handleConn 处理单个 WebSocket 连接
func (s *WSServer) handleConn(conn *websocket.Conn) {
	defer conn.Close()

	var userID string
	lastActive := time.Now()

	// 设置读超时（比心跳间隔长）
	conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		lastActive = time.Now()
		return nil
	})

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Infof("[Gateway] 连接断开: user=%s, err=%v", userID, err)
			if userID != "" {
				s.connMgr.Remove(userID)
				// ★ 通知 Controller 该用户下线
				s.ctrlCli.UpdateStatus(context.Background(), &ctrlpb.UpdateStatusRequest{
					UserId: userID, GatewayId: s.gatewayID, Status: 0, // OFFLINE
				})
			}
			return
		}
		lastActive = time.Now()

		var msg WSMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			s.writeJSON(conn, WSResponse{Type: "error", Error: "消息格式错误"})
			continue
		}

		switch msg.Type {

		// —— 认证 ——
		case "auth":
			s.handleAuth(conn, &msg, &userID)

		// —— 单聊消息 ——
		case "chat":
			s.handleChat(conn, &msg, userID)

		// —— 心跳 ——
		case "ping":
			s.writeJSON(conn, WSResponse{Type: "pong", ServerTime: time.Now().UnixMilli()})

		default:
			s.writeJSON(conn, WSResponse{Type: "error", Error: "未知消息类型: " + msg.Type})
		}

		// 更新读超时
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		_ = lastActive
	}
}

// —— 处理认证 ——
func (s *WSServer) handleAuth(conn *websocket.Conn, msg *WSMessage, userID *string) {
	log.Infof("[Gateway] 收到认证请求: token=%s", msg.Token)

	// ★ 调用 Controller 校验 Token
	resp, err := s.ctrlCli.AuthCheck(context.Background(), &ctrlpb.AuthCheckRequest{
		Token: msg.Token,
	})
	if err != nil || !resp.Valid {
		log.Errorf("[Gateway] 认证失败: err=%v, valid=%v", err, resp.GetValid())
		s.writeJSON(conn, WSResponse{Type: "auth_ack", Ok: false, Error: "认证失败"})
		return
	}

	*userID = resp.UserId
	s.connMgr.Add(resp.UserId, conn)

	// ★ 通知 Controller 用户上线
	s.ctrlCli.UpdateStatus(context.Background(), &ctrlpb.UpdateStatusRequest{
		UserId:    resp.UserId,
		GatewayId: s.gatewayID,
		Status:    1, // ONLINE
	})

	log.Infof("[Gateway] 用户认证成功: user_id=%s, 当前在线=%d", resp.UserId, s.connMgr.OnlineUsers())
	s.writeJSON(conn, WSResponse{Type: "auth_ack", Ok: true})
}

// —— 处理单聊消息 ——
func (s *WSServer) handleChat(conn *websocket.Conn, msg *WSMessage, fromUser string) {
	if fromUser == "" {
		s.writeJSON(conn, WSResponse{Type: "error", Error: "请先认证"})
		return
	}

	log.Infof("[Gateway] 转发消息: from=%s → to=%s, content=%s", fromUser, msg.To, msg.Content)

	// ★ 调用 Controller 路由消息
	resp, err := s.ctrlCli.RouteMessage(context.Background(), &ctrlpb.RouteMessageRequest{
		FromUserId: fromUser,
		ToUserId:   msg.To,
		Content:    msg.Content,
		MsgId:      "tmp_" + time.Now().Format("150405"),
	})
	if err != nil {
		log.Errorf("[Gateway] 转发失败: %v", err)
		s.writeJSON(conn, WSResponse{Type: "error", Error: err.Error()})
		return
	}

	s.writeJSON(conn, WSResponse{
		Type:   "msg_ack",
		MsgID:  resp.MsgId,
		ServerTime: resp.ServerTime,
	})
	log.Infof("[Gateway] 消息已转发: msg_id=%s", resp.MsgId)
}

// —— 写 JSON 到 WebSocket ——
func (s *WSServer) writeJSON(conn *websocket.Conn, resp WSResponse) {
	data, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, data)
}

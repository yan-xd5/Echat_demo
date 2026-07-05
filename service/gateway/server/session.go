package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"trpc.group/trpc-go/trpc-go/client"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/idgen"
	"echat/sdk/route"
	ctrlpb "echat/service/controller/stub"
)

// UserSession 单个 WebSocket 连接抽象：2 协程 + 1 管道。
type UserSession struct {
	UID         string
	DeviceID    string
	Platform    string
	WriteCh     chan []byte
	Conn        *websocket.Conn
	ConnectTime time.Time
	Ctx         context.Context
	Cancel      context.CancelFunc
}

// NewSession 创建会话。
func NewSession(uid, deviceID, platform string, conn *websocket.Conn) *UserSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &UserSession{
		UID:         uid,
		DeviceID:    deviceID,
		Platform:    platform,
		WriteCh:     make(chan []byte, 256),
		Conn:        conn,
		ConnectTime: time.Now(),
		Ctx:         ctx,
		Cancel:      cancel,
	}
}

// Run 启动 reader + writer goroutine。
func (s *UserSession) Run(gateway *Gateway) {
	go s.runReader(gateway)
	go s.runWriter()
}

// ============================================================
// reader goroutine
// ============================================================

func (s *UserSession) runReader(gateway *Gateway) {
	defer func() {
		s.Cancel()
		s.Conn.Close()
		gateway.connMgr.Unregister(s.UID, s.DeviceID)
		// 用 background context，避免 ctx 已取消导致 Redis 清理失败
		gateway.sessionRedis.RemoveSession(context.Background(), s.UID, s.DeviceID)
		log.Infof("[Gateway] reader 退出: uid=%s, device=%s", s.UID, s.DeviceID)
	}()

	s.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	s.Conn.SetPongHandler(func(string) error {
		s.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		gateway.sessionRedis.RefreshTTL(s.Ctx, s.UID)
		return nil
	})

	for {
		select {
		case <-s.Ctx.Done():
			return
		default:
		}

		_, msgBytes, err := s.Conn.ReadMessage()
		if err != nil {
			return
		}
		s.Conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		s.handleMessage(msgBytes, gateway)
	}
}

// ============================================================
// writer goroutine
// ============================================================

func (s *UserSession) runWriter() {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-s.Ctx.Done():
			return
		case data, ok := <-s.WriteCh:
			if !ok {
				return
			}
			s.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-pingTicker.C:
			s.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// writeNonBlocking 非阻塞写入 WriteCh。
func (s *UserSession) writeNonBlocking(data []byte) {
	select {
	case s.WriteCh <- data:
	default:
	}
}

// ============================================================
// 协议转换
// ============================================================

// WSMessage 客户端上行帧元数据。
type WSMessage struct {
	Seq  int64  `json:"seq"`
	Type string `json:"type"`
}

// WSResponse 服务端下行帧。
type WSResponse struct {
	Seq        int64  `json:"seq,omitempty"`
	Type       string `json:"type"`
	MsgID      string `json:"msg_id,omitempty"`
	SeqID      int64  `json:"seq_id,omitempty"`
	ServerTime int64  `json:"server_time,omitempty"`
	Content    string `json:"content,omitempty"`
	From       string `json:"from,omitempty"`
	Error      string `json:"error,omitempty"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
	HandshakeTimeout: 10 * time.Second,
}

func (s *UserSession) handleMessage(msgBytes []byte, gateway *Gateway) {
	var msg WSMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		s.writeNonBlocking([]byte(`{"type":"error","error":"消息格式错误"}`))
		return
	}

	switch msg.Type {
	case "chat", "group_chat", "typing", "read_receipt", "delivery_ack":
		s.handleRouteMessage(&msg, msgBytes, gateway)
	case "ping":
		pong, _ := json.Marshal(WSResponse{Type: "pong", ServerTime: time.Now().UnixMilli()})
		s.writeNonBlocking(pong)
	default:
		errFrame, _ := json.Marshal(WSResponse{Seq: msg.Seq, Type: "error", Error: "未知消息类型: " + msg.Type})
		s.writeNonBlocking(errFrame)
	}
}

func (s *UserSession) handleRouteMessage(msg *WSMessage, rawBytes []byte, gateway *Gateway) {
	// 1. 提取 sessionID
	sessionID := route.ExtractSessionID(rawBytes, s.UID)

	// 2. 获取 seq_id（Redis 分布式锁 + INCR）
	seqID, err := route.GenSeqID(s.Ctx, gateway.redis, sessionID)
	if err != nil {
		errFrame, _ := json.Marshal(WSResponse{Seq: msg.Seq, Type: "error", Error: "序号生成失败"})
		s.writeNonBlocking(errFrame)
		return
	}

	// 3. 会话级路由选 Controller
	addr, err := gateway.sessionRouter.Resolve(sessionID)
	if err != nil {
		errFrame, _ := json.Marshal(WSResponse{Seq: msg.Seq, Type: "error", Error: "路由解析失败"})
		s.writeNonBlocking(errFrame)
		return
	}

	// 4. 生成 request_id + tRPC 调用
	requestID := idgen.GenerateRequestID(gateway.gatewayID)
	ctrlCli := ctrlpb.NewControllerServiceClientProxy(client.WithTarget(addr))
	resp, err := ctrlCli.RouteMessage(s.Ctx, &ctrlpb.RouteMessageRequest{
		FromUserId: s.UID,
		DeviceId:   s.DeviceID,
		Seq:        msg.Seq,
		SeqId:      seqID,
		MsgType:    msg.Type,
		RawBody:    rawBytes,
		RequestId:  requestID,
	})
	if err != nil {
		return // tRPC 失败不进 ACK，客户端靠超时重试
	}

	// 5. 返回 ACK（区分成功/失败）
	if !resp.Success {
		errFrame, _ := json.Marshal(WSResponse{Seq: msg.Seq, Type: "error", Error: resp.Reason})
		s.writeNonBlocking(errFrame)
		return
	}
	ack, _ := json.Marshal(WSResponse{
		Seq:        msg.Seq,
		Type:       "ack",
		MsgID:      resp.MsgId,
		SeqID:      seqID,
		ServerTime: resp.ServerTime,
	})
	s.writeNonBlocking(ack)
}

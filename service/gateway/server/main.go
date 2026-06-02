package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/client"
	"trpc.group/trpc-go/trpc-go/log"

	ctrlpb "echat/service/controller/stub"
	gwpb "echat/service/gateway/stub"
)

const gatewayID = "gateway-01"

// gatewayImpl 实现 GatewayInternal 接口（供 Controller 调用）
type gatewayImpl struct {
	gwpb.UnimplementedGatewayInternal
	connMgr *ConnManager
}

// PushToUser Controller 调用此接口向指定用户推送消息
func (s *gatewayImpl) PushToUser(ctx context.Context, req *gwpb.PushToUserRequest) (*gwpb.PushToUserResponse, error) {
	log.InfoContextf(ctx, "[Gateway] 收到推送: to=%s, from=%s, content=%s", req.UserId, req.FromUserId, req.Content)

	conn := s.connMgr.Get(req.UserId)
	if conn == nil {
		log.Infof("[Gateway] 用户不在线: %s", req.UserId)
		return &gwpb.PushToUserResponse{Delivered: false, Reason: "用户不在线"}, nil
	}

	// ★ 通过 WebSocket 推送给客户端
	wsServer := &WSServer{connMgr: s.connMgr}
	wsServer.writeJSON(conn, WSResponse{
		Type:    "push",
		MsgID:   req.MsgId,
		From:    req.FromUserId,
		Content: req.Content,
	})

	log.Infof("[Gateway] 推送成功: user=%s, msg_id=%s", req.UserId, req.MsgId)
	return &gwpb.PushToUserResponse{Delivered: true, Reason: "投递成功"}, nil
}

func main() {
	// ① 创建连接管理器
	connMgr := NewConnManager()

	// ② 创建 Controller 客户端（Gateway → Controller）
	ctrlCli := ctrlpb.NewControllerServiceClientProxy(
		client.WithTarget("ip://127.0.0.1:8002"),
	)

	// ③ 启动 WebSocket 服务器（goroutine，端口 9000）
	wsServer := NewWSServer(connMgr, ctrlCli, gatewayID)
	httpSrv := &http.Server{Addr: "0.0.0.0:9000", Handler: wsServer}
	go func() {
		log.Infof("[Gateway] WebSocket 服务启动: ws://0.0.0.0:9000/ws")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Gateway] WebSocket 启动失败: %v", err)
		}
	}()

	// ④ 启动 tRPC 服务器（供 Controller 调用 PushToUser，端口 8003）
	s := trpc.NewServer()
	gwpb.RegisterGatewayInternalService(s, &gatewayImpl{connMgr: connMgr})

	// ⑤ 优雅关机：Ctrl+C 同时关闭两个服务器
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		sig := <-ch
		log.Infof("[Gateway] 收到信号 %v，正在优雅关机...", sig)
		httpSrv.Shutdown(context.Background()) // 关闭 WebSocket
		s.Close(nil)                            // 关闭 tRPC
	}()

	log.Infof("[Gateway] tRPC 服务启动: 0.0.0.0:8003 (Ctrl+C 停止)")
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[Gateway] 已停止")
}

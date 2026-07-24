package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	trpc "trpc.group/trpc-go/trpc-go"
	"github.com/redis/go-redis/v9"
	"trpc.group/trpc-go/trpc-database/goredis"
	"trpc.group/trpc-go/trpc-go/log"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"

	"echat/sdk/infrastructure/observability"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"
	"echat/sdk/usecase/route"

	"echat/service/gateway/internal/handler"
	"echat/service/gateway/internal/session"
	gwpb "echat/service/gateway/stub"
)

func main() {
	s := trpc.NewServer()

	universalCli, err := goredis.New("echat.redis.service")
	if err != nil {
		log.Fatalf("[Gateway] Redis: %v", err)
	}
	redisCli := universalCli.(*redis.Client)

	sessionRedis := session.NewSessionRedis(redisCli)
	connMgr := session.NewConnManager(envInt("MAX_CONN", 100000))

	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName: "echat-gateway", ServiceVersion: "1.0.0",
		OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"), PprofPort: 6063,
	})
	if err != nil {
		log.Warnf("[Gateway] obs: %v", err)
	}
	if obsShutdown != nil {
		defer func() { obsShutdown(context.Background()) }()
	}
	observability.InitBusinessMetrics()

	gateway := &session.Gateway{
		GatewayID: env("GATEWAY_ID", "gw-01"), ConnMgr: connMgr, SessionRedis: sessionRedis,
		Redis: redisCli, SessionRouter: route.NewSessionRouter(route.NewTRPCDiscovery("polarismesh", "default")),
	}

	kickCtx, kickCancel := context.WithCancel(context.Background())
	session.StartKickListener(kickCtx, redisCli, connMgr, gateway.GatewayID)

	wsPort := env("WS_PORT", "9000")
	wsSrv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%s", wsPort),
		Handler: &handler.WSAuthHandler{Gateway: gateway},
	}
	go func() {
		log.Infof("[Gateway] WebSocket: ws://0.0.0.0:%s/ws", wsPort)
		if err := wsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("[Gateway] WebSocket 服务异常: %v", err)
		}
	}()

	gwpb.RegisterGatewayInternalService(s, &handler.GatewayPushService{Gateway: gateway})

	// 优雅关闭
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("[Gateway] 收到关闭信号，开始优雅退出...")

		// 1. 停止 WebSocket HTTP server — 拒绝新连接（5s 超时）
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := wsSrv.Shutdown(shutdownCtx); err != nil {
			log.Warnf("[Gateway] WebSocket 关停未完全: %v", err)
		} else {
			log.Info("[Gateway] WebSocket 服务已停止")
		}

		// 2. 断开所有客户端连接 — writer defer 链路清理
		log.Infof("[Gateway] 正在断开 %d 个连接...", connMgr.Len())
		connMgr.Shutdown()
		connMgrLen := connMgr.Len()

		// 3. 停止 KickListener 和哈希环刷新
		kickCancel()
		gateway.SessionRouter.Close()

		// 4. 等 writer goroutine 完成 defer 清理（Redis Unregister + RemoveSession）
		time.Sleep(500 * time.Millisecond)
		log.Infof("[Gateway] 连接已全部断开 (剩余 %d)", connMgrLen)

		// 5. 停止 tRPC server
		s.Close(nil)
	}()

	log.Infof("[Gateway] 启动 (gateway=%s, ws=%s)", gateway.GatewayID, wsPort)
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[Gateway] 已停止")
}

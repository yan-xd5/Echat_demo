package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	trpc "trpc.group/trpc-go/trpc-go"
	"github.com/redis/go-redis/v9"
	"trpc.group/trpc-go/trpc-database/goredis"
	"trpc.group/trpc-go/trpc-go/log"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"

	"echat/sdk/infrastructure/observability"
	"echat/sdk/usecase/route"

	"echat/service/gateway/internal/handler"
	"echat/service/gateway/internal/session"
	gwpb "echat/service/gateway/stub"
)

func main() {
	s := trpc.NewServer()

	universalCli, err := goredis.New("echat.redis.service")
	if err != nil { log.Fatalf("[Gateway] Redis: %v", err) }
	redisCli := universalCli.(*redis.Client) // go-redis v9 interface

	sessionRedis := session.NewSessionRedis(redisCli)
	connMgr := session.NewConnManager(envInt("MAX_CONN", 100000))

	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName: "echat-gateway", ServiceVersion: "1.0.0",
		OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"), PprofPort: 6063,
	})
	if err != nil { log.Warnf("[Gateway] obs: %v", err) }
	if obsShutdown != nil { defer func() { obsShutdown(context.Background()) }() }
	observability.InitBusinessMetrics()

	gateway := &session.Gateway{
		GatewayID: env("GATEWAY_ID", "gw-01"), ConnMgr: connMgr, SessionRedis: sessionRedis,
		Redis: redisCli, SessionRouter: route.NewSessionRouter(route.NewTRPCDiscovery("polarismesh", "default")),
	}
	session.StartKickListener(context.Background(), redisCli, connMgr, gateway.GatewayID)

	wsPort := env("WS_PORT", "9000")
	go func() {
		log.Infof("[Gateway] WebSocket: ws://0.0.0.0:%s/ws", wsPort)
		http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", wsPort), &handler.WSAuthHandler{Gateway: gateway})
	}()

	gwpb.RegisterGatewayInternalService(s, &handler.GatewayPushService{Gateway: gateway})

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[Gateway] 收到信号 %v", <-ch)
		connMgr.Shutdown()
		gateway.SessionRouter.Close()
		s.Close(nil)
	}()

	log.Infof("[Gateway] 启动 (gateway=%s, ws=%s)", gateway.GatewayID, wsPort)
	if err := s.Serve(); err != nil { log.Error(err) }
	log.Info("[Gateway] 已停止")
}

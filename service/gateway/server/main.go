package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/polarismesh/polaris-go/api"
	"github.com/redis/go-redis/v9"
	trpc "trpc.group/trpc-go/trpc-go"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	"trpc.group/trpc-go/trpc-go/log"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"

	"echat/sdk/route"
	gwpb "echat/service/gateway/stub"
)

func main() {
	// ① Redis
	redisCli := redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "127.0.0.1:6379")})
	sessionRedis := NewSessionRedis(redisCli)

	// ② ConnManager
	maxConn := envInt("MAX_CONN", 100000)
	connMgr := NewConnManager(maxConn)

	// ③ Polaris
	polarisConsumer, err := api.NewConsumerAPI()
	if err != nil {
		log.Fatalf("[Gateway] Polaris 初始化失败: %v", err)
	}
	sessionRouter := route.NewSessionRouter(polarisConsumer)

	// ④ Gateway
	gatewayID := env("GATEWAY_ID", "gw-01")
	gateway := &Gateway{
		gatewayID: gatewayID, connMgr: connMgr, sessionRedis: sessionRedis,
		redis: redisCli, sessionRouter: sessionRouter,
	}

	// ④.1 跨网关顶号监听
	startKickListener(context.Background(), redisCli, connMgr, gatewayID)

	// ⑤ WebSocket
	wsPort := env("WS_PORT", "9000")
	wsHandler := &WSAuthHandler{gateway: gateway}
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%s", wsPort)
		log.Infof("[Gateway] WebSocket: ws://%s/ws", addr)
		if err := http.ListenAndServe(addr, wsHandler); err != nil {
			log.Fatalf("[Gateway] WebSocket 失败: %v", err)
		}
	}()

	// ⑥ tRPC
	s := trpc.NewServer()
	gwpb.RegisterGatewayInternalService(s, &GatewayPushService{gateway: gateway})

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[Gateway] 收到信号 %v, 优雅关机...", <-ch)
		connMgr.Shutdown()
		s.Close(nil)
	}()

	log.Infof("[Gateway] 启动 (gateway=%s, ws=%s, redis=%s)", gatewayID, wsPort, env("REDIS_ADDR", "127.0.0.1:6379"))
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[Gateway] 已停止")
}

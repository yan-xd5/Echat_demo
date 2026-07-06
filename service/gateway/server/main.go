package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/redis/go-redis/v9"
	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-database/goredis"
	trpcdiscovery "trpc.group/trpc-go/trpc-go/naming/discovery"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	"trpc.group/trpc-go/trpc-go/log"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"

	"echat/sdk/observability"
	"echat/sdk/route"
	gwpb "echat/service/gateway/stub"
)

// tRPCDiscovery 适配 tRPC Polaris discovery → route.ServiceDiscovery。
// tRPC Polaris 插件将实例信息存在 Node.Metadata["service_instances"] 中（*model.InstancesResponse）。
type tRPCDiscovery struct{}

func (d *tRPCDiscovery) GetInstances(serviceName string) ([]route.ServiceInstance, error) {
	disc := trpcdiscovery.Get("polarismesh")
	if disc == nil {
		return nil, fmt.Errorf("discovery polarismesh 未注册")
	}
	nodes, err := disc.List(serviceName, trpcdiscovery.WithNamespace("default"))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", serviceName, err)
	}

	var instances []route.ServiceInstance
	for _, n := range nodes {
		// tRPC Polaris 插件把实例信息存在 Metadata
		if raw, ok := n.Metadata["service_instances"]; ok {
			if resp, ok := raw.(*model.InstancesResponse); ok {
				for _, inst := range resp.Instances {
					if !inst.IsHealthy() {
						continue
					}
					meta := make(map[string]string)
					for k, v := range inst.GetMetadata() {
						meta[k] = v
					}
					instances = append(instances, route.ServiceInstance{
						ID:       inst.GetId(),
						Address:  fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort()),
						Metadata: meta,
					})
				}
			}
		} else if n.Address != "" {
			// 兼容直接返回 Address 的 discovery 实现
			meta := make(map[string]string, len(n.Metadata))
			for k, v := range n.Metadata {
				if s, ok := v.(string); ok {
					meta[k] = s
				}
			}
			instances = append(instances, route.ServiceInstance{
				ID:       n.Address,
				Address:  n.Address,
				Metadata: meta,
			})
		}
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("服务 %s 无可用实例", serviceName)
	}
	return instances, nil
}

func main() {
	// ① tRPC 框架初始化（加载 trpc_go.yaml，后续 goredis.New 依赖此步骤）
	s := trpc.NewServer()

	// ② Redis（统一走 tRPC 框架）
	universalCli, err := goredis.New("echat.redis.service")
	if err != nil {
		log.Fatalf("[Gateway] Redis 初始化失败: %v", err)
	}
	redisCli, ok := universalCli.(*redis.Client)
	if !ok {
		log.Fatalf("[Gateway] Redis 客户端类型异常")
	}
	sessionRedis := NewSessionRedis(redisCli)

	// ③ ConnManager
	maxConn := envInt("MAX_CONN", 100000)
	connMgr := NewConnManager(maxConn)

	// ④ 观测: Metrics + Profiling
	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName:    "echat-gateway",
		ServiceVersion: "1.0.0",
		OTLPEndpoint:   env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"),
		PprofPort:      6063,
	})
	if err != nil {
		log.Warnf("[Gateway] 观测初始化失败 (metrics): %v", err)
	}
	if obsShutdown != nil {
		defer func() {
			if err := obsShutdown(context.Background()); err != nil {
				log.Warnf("[Gateway] 观测关闭失败: %v", err)
			}
		}()
	}
	if err := observability.InitBusinessMetrics(); err != nil {
		log.Warnf("[Gateway] 业务指标初始化失败: %v", err)
	}

	// ⑤ 会话路由（统一走 tRPC naming/discovery）
	sessionRouter := route.NewSessionRouter(&tRPCDiscovery{})

	// ⑥ Gateway
	gatewayID := env("GATEWAY_ID", "gw-01")
	gateway := &Gateway{
		gatewayID: gatewayID, connMgr: connMgr, sessionRedis: sessionRedis,
		redis: redisCli, sessionRouter: sessionRouter,
	}
	startKickListener(context.Background(), redisCli, connMgr, gatewayID)

	// ⑦ WebSocket
	wsPort := env("WS_PORT", "9000")
	wsHandler := &WSAuthHandler{gateway: gateway}
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%s", wsPort)
		log.Infof("[Gateway] WebSocket: ws://%s/ws", addr)
		if err := http.ListenAndServe(addr, wsHandler); err != nil {
			log.Fatalf("[Gateway] WebSocket 失败: %v", err)
		}
	}()

	// ⑧ 注册 tRPC 服务
	gwpb.RegisterGatewayInternalService(s, &GatewayPushService{gateway: gateway})

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[Gateway] 收到信号 %v, 优雅关机...", <-ch)
		connMgr.Shutdown()
		s.Close(nil)
	}()

	log.Infof("[Gateway] 启动 (gateway=%s, ws=%s)", gatewayID, wsPort)
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[Gateway] 已停止")
}

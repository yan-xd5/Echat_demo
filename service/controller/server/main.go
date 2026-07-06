package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/redis/go-redis/v9"
	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-database/goredis"
	"trpc.group/trpc-go/trpc-go/log"
	trpcdiscovery "trpc.group/trpc-go/trpc-go/naming/discovery"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"

	"echat/sdk/auth"
	"echat/sdk/mysql"
	"echat/sdk/observability"
	"echat/sdk/route"
	ctrlpb "echat/service/controller/stub"
)

// tRPCDiscovery 适配 tRPC naming/discovery → route.ServiceDiscovery。
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
			instances = append(instances, route.ServiceInstance{ID: n.Address, Address: n.Address})
		}
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("服务 %s 无可用实例", serviceName)
	}
	return instances, nil
}

func main() {
	// ① tRPC 框架初始化（加载 trpc_go.yaml，后续 goredis.New / discovery 依赖此步骤）
	s := trpc.NewServer()

	// ② Redis（统一走 tRPC 框架）
	universalCli, err := goredis.New("echat.redis.service")
	if err != nil {
		log.Fatalf("[Controller] Redis 初始化失败: %v", err)
	}
	redisCli, ok := universalCli.(*redis.Client)
	if !ok {
		log.Fatalf("[Controller] Redis 客户端类型异常")
	}

	// ③ MySQL
	dsn := env("MYSQL_DSN", "root:@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true")
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatalf("[Controller] MySQL 初始化失败: %v", err)
	}
	defer db.Close()

	// ④ Snowflake WorkerID
	workerID := envInt("SNOWFLAKE_WORKER_ID", 1)
	idGen, err := NewIDGen(workerID)
	if err != nil {
		log.Fatalf("[Controller] Snowflake 初始化失败: %v", err)
	}
	log.Infof("[Controller] WorkerID=%d", workerID)

	// ⑤ 观测: Metrics + Profiling
	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName:    "echat-controller",
		ServiceVersion: "1.0.0",
		OTLPEndpoint:   env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"),
		PprofPort:      6062,
	})
	if err != nil {
		log.Warnf("[Controller] 观测初始化失败 (metrics): %v", err)
	}
	if obsShutdown != nil {
		defer func() {
			if err := obsShutdown(context.Background()); err != nil {
				log.Warnf("[Controller] 观测关闭失败: %v", err)
			}
		}()
	}
	if err := observability.InitBusinessMetrics(); err != nil {
		log.Warnf("[Controller] 业务指标初始化失败: %v", err)
	}

	// ⑥ 服务发现 + Pipeline + 协程池
	svcDiscovery := &tRPCDiscovery{}
	msgStore := mysql.NewMessageStore(db)
	authChecker := mysql.NewAuthChecker(db)
	pipeline := &Pipeline{
		idGen:       idGen,
		redis:       redisCli,
		discovery:   svcDiscovery,
		msgStore:    msgStore,
		authChecker: authChecker,
	}
	pool, err := NewPool(pipeline)
	if err != nil {
		log.Fatalf("[Controller] 协程池初始化失败: %v", err)
	}

	// ⑦ 注册服务
	entry := &Entry{redis: redisCli, pool: pool}
	ctrlpb.RegisterControllerServiceService(s, &controllerImpl{entry: entry})

	// ⑧ 优雅关机
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[Controller] 收到信号 %v，正在优雅关机...", <-ch)
		pool.Shutdown()
		s.Close(nil)
	}()

	log.Info("[Controller] 服务启动中...(Ctrl+C 停止)")
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[Controller] 已停止")
}

type controllerImpl struct {
	ctrlpb.UnimplementedControllerService
	entry *Entry
}

func (s *controllerImpl) AuthCheck(ctx context.Context, req *ctrlpb.AuthCheckRequest) (*ctrlpb.AuthCheckResponse, error) {
	uid, platform, err := auth.ValidateTicket(req.Token)
	if err != nil {
		return &ctrlpb.AuthCheckResponse{Valid: false, Reason: "Token 无效"}, nil
	}
	log.InfoContextf(ctx, "[Controller] Token 校验通过: uid=%s, platform=%s", uid, platform)
	return &ctrlpb.AuthCheckResponse{Valid: true, UserId: uid, Reason: "校验通过"}, nil
}

func (s *controllerImpl) RouteMessage(ctx context.Context, req *ctrlpb.RouteMessageRequest) (*ctrlpb.RouteMessageResponse, error) {
	return s.entry.Handle(ctx, req), nil
}

func (s *controllerImpl) UpdateStatus(ctx context.Context, req *ctrlpb.UpdateStatusRequest) (*ctrlpb.UpdateStatusResponse, error) {
	log.InfoContextf(ctx, "[Controller] 状态变更: uid=%s, status=%v, gateway=%s", req.UserId, req.Status, req.GatewayId)
	return &ctrlpb.UpdateStatusResponse{}, nil
}

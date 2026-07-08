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
	trpcdiscovery "trpc.group/trpc-go/trpc-go/naming/discovery"
	"trpc.group/trpc-go/trpc-go/log"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"

	"echat/sdk/usecase/auth"
	"echat/sdk/repository/mysql"
	"echat/sdk/infrastructure/observability"
	"echat/sdk/usecase/route"

	"echat/service/controller/internal/pipeline"
	ctrlpb "echat/service/controller/stub"
)

func main() {
	s := trpc.NewServer()
	universalCli, err := goredis.New("echat.redis.service")
	if err != nil { log.Fatalf("[Controller] Redis: %v", err) }
	redisCli, _ := universalCli.(*redis.Client)

	dsn := env("MYSQL_DSN", "root:@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true")
	db, err := mysql.NewDB(dsn)
	if err != nil { log.Fatalf("[Controller] MySQL: %v", err) }
	defer db.Close()

	idGen, err := pipeline.NewIDGen(envInt("SNOWFLAKE_WORKER_ID", 1))
	if err != nil { log.Fatalf("[Controller] Snowflake: %v", err) }

	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName: "echat-controller", ServiceVersion: "1.0.0",
		OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"), PprofPort: 6062,
	})
	if err != nil { log.Warnf("[Controller] obs: %v", err) }
	if obsShutdown != nil { defer func() { obsShutdown(context.Background()) }() }
	observability.InitBusinessMetrics()

	pl := pipeline.New(pipeline.Config{
		IDGen: idGen, Redis: redisCli, Discovery: newTRPCDiscovery(),
		MsgStore: mysql.NewMessageStore(db), AuthChecker: mysql.NewAuthChecker(db),
	})
	pool, err := pipeline.NewPool(pl)
	if err != nil { log.Fatalf("[Controller] Pool: %v", err) }

	entry := pipeline.NewEntry(pipeline.EntryConfig{Redis: redisCli, Pool: pool})
	handler := newControllerImpl(entry)
	ctrlpb.RegisterControllerServiceService(s, handler)

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[Controller] 收到信号 %v", <-ch)
		pool.Shutdown()
		s.Close(nil)
	}()

	log.Info("[Controller] 启动中...")
	if err := s.Serve(); err != nil { log.Error(err) }
	log.Info("[Controller] 已停止")
}

// ─── discovery ───
func newTRPCDiscovery() route.ServiceDiscovery { return &trpcDiscoveryImpl{} }
type trpcDiscoveryImpl struct{}
func (d *trpcDiscoveryImpl) GetInstances(serviceName string) ([]route.ServiceInstance, error) {
	disc := trpcdiscovery.Get("polarismesh")
	if disc == nil { return nil, fmt.Errorf("discovery polarismesh 未注册") }
	nodes, err := disc.List(serviceName, trpcdiscovery.WithNamespace("default"))
	if err != nil { return nil, err }
	var instances []route.ServiceInstance
	for _, n := range nodes {
		if raw, ok := n.Metadata["service_instances"]; ok {
			if resp, ok := raw.(*model.InstancesResponse); ok {
				for _, inst := range resp.Instances {
					if !inst.IsHealthy() { continue }
					meta := make(map[string]string)
					for k, v := range inst.GetMetadata() { meta[k] = v }
					instances = append(instances, route.ServiceInstance{
						ID: inst.GetId(), Address: fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort()), Metadata: meta,
					})
				}
			}
		}
	}
	if len(instances) == 0 { return nil, fmt.Errorf("服务 %s 无可用实例", serviceName) }
	return instances, nil
}

// ─── handler ───
func newControllerImpl(entry *pipeline.Entry) *controllerImpl { return &controllerImpl{entry: entry} }
type controllerImpl struct {
	ctrlpb.UnimplementedControllerService
	entry interface{ Handle(context.Context, *ctrlpb.RouteMessageRequest) *ctrlpb.RouteMessageResponse }
}
func (s *controllerImpl) AuthCheck(ctx context.Context, req *ctrlpb.AuthCheckRequest) (*ctrlpb.AuthCheckResponse, error) {
	uid, _, err := auth.ValidateTicket(req.Token)
	if err != nil { return &ctrlpb.AuthCheckResponse{Valid: false, Reason: "Token 无效"}, nil }
	return &ctrlpb.AuthCheckResponse{Valid: true, UserId: uid, Reason: "校验通过"}, nil
}
func (s *controllerImpl) RouteMessage(ctx context.Context, req *ctrlpb.RouteMessageRequest) (*ctrlpb.RouteMessageResponse, error) {
	return s.entry.Handle(ctx, req), nil
}
func (s *controllerImpl) UpdateStatus(ctx context.Context, req *ctrlpb.UpdateStatusRequest) (*ctrlpb.UpdateStatusResponse, error) {
	return &ctrlpb.UpdateStatusResponse{}, nil
}

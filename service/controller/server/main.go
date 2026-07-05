package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/polarismesh/polaris-go/api"
	"github.com/redis/go-redis/v9"
	trpc "trpc.group/trpc-go/trpc-go"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"
	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/auth"
	"echat/sdk/mysql"
	ctrlpb "echat/service/controller/stub"
)

func main() {
	// ① Redis
	redisCli := redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "127.0.0.1:6379")})

	// ② MySQL
	dsn := env("MYSQL_DSN", "root:@tcp(127.0.0.1:3306)/echat?charset=utf8mb4&parseTime=true")
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatalf("[Controller] MySQL 初始化失败: %v", err)
	}
	defer db.Close()

	// ③ Snowflake WorkerID（环境变量 SNOWFLAKE_WORKER_ID，默认 1）
	workerID := envInt("SNOWFLAKE_WORKER_ID", 1)
	idGen, err := NewIDGen(workerID)
	if err != nil {
		log.Fatalf("[Controller] Snowflake 初始化失败: %v", err)
	}
	log.Infof("[Controller] WorkerID=%d", workerID)

	// ④ Polaris Consumer
	polarisConsumer, err := api.NewConsumerAPI()
	if err != nil {
		log.Fatalf("[Controller] Polaris Consumer 初始化失败: %v", err)
	}

	// ⑤ Pipeline（注入 MySQL Repo）
	msgStore := mysql.NewMessageStore(db)
	authChecker := mysql.NewAuthChecker(db)
	pipeline := &Pipeline{
		idGen:       idGen,
		redis:       redisCli,
		polaris:     polarisConsumer,
		msgStore:    msgStore,
		authChecker: authChecker,
	}

	// ⑥ 协程池
	pool, err := NewPool(pipeline)
	if err != nil {
		log.Fatalf("[Controller] 协程池初始化失败: %v", err)
	}

	// ⑦ Entry + Controller 实现
	entry := &Entry{redis: redisCli, pool: pool}
	svc := &controllerImpl{entry: entry}

	// ⑧ tRPC 服务器
	s := trpc.NewServer()
	ctrlpb.RegisterControllerServiceService(s, svc)

	// ⑨ 优雅关机
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

// controllerImpl 实现 ControllerService 接口。
type controllerImpl struct {
	ctrlpb.UnimplementedControllerService
	entry *Entry
}

// AuthCheck Token 校验 → SDK 本地验证。
func (s *controllerImpl) AuthCheck(ctx context.Context, req *ctrlpb.AuthCheckRequest) (*ctrlpb.AuthCheckResponse, error) {
	uid, platform, err := auth.ValidateTicket(req.Token)
	if err != nil {
		return &ctrlpb.AuthCheckResponse{Valid: false, Reason: "Token 无效"}, nil
	}
	log.InfoContextf(ctx, "[Controller] Token 校验通过: uid=%s, platform=%s", uid, platform)
	return &ctrlpb.AuthCheckResponse{Valid: true, UserId: uid, Reason: "校验通过"}, nil
}

// RouteMessage 消息路由入口 → Entry.Handle。
func (s *controllerImpl) RouteMessage(ctx context.Context, req *ctrlpb.RouteMessageRequest) (*ctrlpb.RouteMessageResponse, error) {
	return s.entry.Handle(ctx, req), nil
}

// UpdateStatus 在线状态变更。
func (s *controllerImpl) UpdateStatus(ctx context.Context, req *ctrlpb.UpdateStatusRequest) (*ctrlpb.UpdateStatusResponse, error) {
	log.InfoContextf(ctx, "[Controller] 状态变更: uid=%s, status=%v, gateway=%s", req.UserId, req.Status, req.GatewayId)
	return &ctrlpb.UpdateStatusResponse{}, nil
}

package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-database/goredis"
	"trpc.group/trpc-go/trpc-go/log"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"

	"echat/sdk/infrastructure/observability"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"
	"echat/sdk/repository/mysql"
	"echat/sdk/usecase/route"

	_ "echat/service/controller/internal/filter"
	ctrlhandler "echat/service/controller/internal/handler"
	"echat/service/controller/internal/pipeline"
	ctrlpb "echat/service/controller/stub"
)

func main() {
	s := trpc.NewServer()
	universalCli, err := goredis.New("echat.redis.service")
	if err != nil {
		log.Fatalf("[Controller] Redis: %v", err)
	}
	redisCli, _ := universalCli.(*redis.Client)

	dsn := env("MYSQL_DSN", "root:@tcp(127.0.0.1:3306)/chat?charset=utf8mb4&parseTime=true")
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatalf("[Controller] MySQL: %v", err)
	}
	defer db.Close()

	idGen, err := pipeline.NewIDGen(envInt("SNOWFLAKE_WORKER_ID", 1))
	if err != nil {
		log.Fatalf("[Controller] Snowflake: %v", err)
	}

	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName: "echat-controller", ServiceVersion: "1.0.0",
		OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"), PprofPort: 6062,
	})
	if err != nil {
		log.Warnf("[Controller] obs: %v", err)
	}
	if obsShutdown != nil {
		defer func() { obsShutdown(context.Background()) }()
	}
	observability.InitBusinessMetrics()

	pl := pipeline.New(pipeline.Config{
		IDGen: idGen, Redis: redisCli, Discovery: route.NewTRPCDiscovery("polarismesh", "default"),
		MsgStore: mysql.NewMessageStore(db), AuthChecker: mysql.NewAuthChecker(db),
	})
	pool, err := pipeline.NewPool(pl)
	if err != nil {
		log.Fatalf("[Controller] Pool: %v", err)
	}

	entry := pipeline.NewEntry(pipeline.EntryConfig{Redis: redisCli, Pool: pool})
	ctrlHandler := ctrlhandler.New(entry, redisCli)
	ctrlpb.RegisterControllerServiceService(s, ctrlHandler)

	// 优雅关闭：监听信号 → 停止接收新请求 → 排空队列 → 释放资源
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("[Controller] 收到关闭信号，开始优雅退出...")

		// 1. 停止 tRPC server — 不再接收新请求
		s.Close(nil)

		// 2. 排空消息队列 — 等待所有在途消息处理完（最长 30s）
		shutdownCh := make(chan struct{})
		go func() {
			log.Info("[Controller] 正在排空消息队列...")
			pool.Shutdown()
			close(shutdownCh)
		}()

		select {
		case <-shutdownCh:
			log.Info("[Controller] 消息队列已排空")
		case <-time.After(30 * time.Second):
			log.Warn("[Controller] 消息队列排空超时（30s），强制退出")
		}
	}()

	log.Info("[Controller] 启动中...")
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[Controller] 已停止")
}

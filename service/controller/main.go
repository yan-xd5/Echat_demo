package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

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
		IDGen: idGen, Redis: redisCli, Discovery: route.NewTRPCDiscovery("polarismesh", "default"),
		MsgStore: mysql.NewMessageStore(db), AuthChecker: mysql.NewAuthChecker(db),
	})
	pool, err := pipeline.NewPool(pl)
	if err != nil { log.Fatalf("[Controller] Pool: %v", err) }

	entry := pipeline.NewEntry(pipeline.EntryConfig{Redis: redisCli, Pool: pool})
	ctrlHandler := ctrlhandler.New(entry)
	ctrlpb.RegisterControllerServiceService(s, ctrlHandler)

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

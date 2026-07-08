package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	redis "github.com/redis/go-redis/v9"

	sdkredis "echat/sdk/repository/redis"
	trpc "trpc.group/trpc-go/trpc-go"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	"trpc.group/trpc-go/trpc-go/log"
	"trpc.group/trpc-go/trpc-database/goredis"

	"echat/sdk/usecase/auth"
	"echat/sdk/infrastructure/idgen"
	"echat/sdk/repository/mysql"
	"echat/sdk/infrastructure/observability"
	_ "echat/service/api/internal/filter"
	"echat/service/api/internal/handler"
	"echat/service/api/internal/restful"
	svctrpc "echat/service/api/internal/trpc"
	pb "echat/service/api/stub"
)

func main() {
	s := trpc.NewServer()

	dsn := GetDSN()
	db, err := mysql.NewDB(dsn)
	if err != nil { log.Fatalf("[API] MySQL: %v", err) }
	defer db.Close()

	workerID := int64(1)
	if s := os.Getenv("SNOWFLAKE_WORKER_ID"); s != "" {
		if v, _ := strconv.ParseInt(s, 10, 64); v >= 1 && v <= 1023 { workerID = v }
	}
	idGen, err := idgen.NewSnowflake(workerID)
	if err != nil { log.Fatalf("[API] Snowflake: %v", err) }

	userRepo := mysql.NewUserRepo(db)
	friendRepo := mysql.NewFriendRepo(db)
	msgRepo := mysql.NewMessageRepo(db)
	groupRepo := mysql.NewGroupRepo(db)
	fileRepo := mysql.NewFileRepo(db)

	svc := handler.NewUserImpl(userRepo, friendRepo, idGen)
	friendSvc := handler.NewFriendImpl(friendRepo, idGen)
	chatRepo := mysql.NewPrivateChatRepo(db)
	msgSvc := handler.NewMessageImpl(msgRepo, chatRepo, groupRepo)
	groupSvc := handler.NewGroupImpl(groupRepo, idGen)
	fileSvc := handler.NewFileImpl(fileRepo, idGen)

	universalCli, err := goredis.New("echat.redis.service")
	if err != nil { log.Fatalf("[API] Redis: %v", err) }
	redisCli, ok := universalCli.(*redis.Client)
	if !ok { log.Fatalf("[API] Redis type error") }
	onlineRepo := sdkredis.NewOnlineRepo(redisCli)

	if err := auth.InitRSA(); err != nil { log.Fatalf("[API] RSA: %v", err) }

	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName: "echat-api", ServiceVersion: "1.0.0",
		OTLPEndpoint: envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"), PprofPort: 6061,
	})
	if err != nil { log.Warnf("[API] obs: %v", err) }
	if obsShutdown != nil { defer func() { obsShutdown(context.Background()) }() }
	observability.InitBusinessMetrics()

	pb.RegisterUserServiceService(s, svc)
	svctrpc.RegisterFriendServiceService(s, friendSvc)
	svctrpc.RegisterMessageServiceService(s, msgSvc)
	svctrpc.RegisterGroupServiceService(s, groupSvc)
	svctrpc.RegisterFileServiceService(s, fileSvc)
	svctrpc.RegisterAuthServiceService(s, svctrpc.NewAuthHandler())

	s.Register(&userServiceHTTPDesc, svc)
	s.Register(&friendServiceHTTPDesc, friendSvc)
	s.Register(&messageServiceHTTPDesc, msgSvc)
	s.Register(&groupServiceHTTPDesc, groupSvc)
	s.Register(&fileServiceHTTPDesc, fileSvc)

	extraSvc := &restful.ExtraService{
		User:    restful.NewExtraUserImpl(userRepo),
		Friend:  restful.NewExtraFriendImpl(friendRepo),
		Message: restful.NewExtraMessageImpl(msgRepo, chatRepo, mysql.NewGroupMessageRepo(db)),
		Group:   restful.NewExtraGroupImpl(groupRepo, idGen),
		File:    restful.NewExtraFileImpl(fileRepo, idGen),
		Misc:    restful.NewExtraMiscImpl(userRepo, fileRepo, mysql.NewGroupMessageRepo(db)),
		Final: restful.NewExtraFinalImpl(
			userRepo, friendRepo, chatRepo, groupRepo, mysql.NewGroupMessageRepo(db), fileRepo, onlineRepo,
		),
		Chat: restful.NewExtraChatImpl(
			chatRepo, groupRepo, mysql.NewGroupMessageRepo(db), onlineRepo, userRepo,
		),
	}
	restful.RegisterExtraService(s, extraSvc)
	s.Register(&extraServiceHTTPDesc, extraSvc)

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[API] 收到信号 %v", <-ch)
		s.Close(nil)
	}()

	log.Info("[API] 启动中...")
	if err := s.Serve(); err != nil { log.Error(err) }
	log.Info("[API] 已停止")
}

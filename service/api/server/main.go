package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	redis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	sdkredis "echat/sdk/redis"
	trpc "trpc.group/trpc-go/trpc-go"
	_ "trpc.group/trpc-go/trpc-opentelemetry/oteltrpc"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh"
	_ "trpc.group/trpc-go/trpc-naming-polarismesh/registry"
	"trpc.group/trpc-go/trpc-go/log"
	"trpc.group/trpc-go/trpc-database/goredis"

	"echat/sdk/auth"
	"echat/sdk/entity"
	"echat/sdk/idgen"
	"echat/sdk/mysql"
	"echat/sdk/observability"
	pb "echat/service/api/stub"
)

type userImpl struct {
	pb.UnimplementedUserService
	userRepo   *mysql.UserRepo
	friendRepo *mysql.FriendRepo
	idGen      *idgen.Snowflake
}

func (s *userImpl) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	log.InfoContextf(ctx, "[API] 注册: account=%s", req.Account)

	exists, err := s.userRepo.ExistsByAccount(ctx, req.Account)
	if err != nil {
		return nil, fmt.Errorf("检查账号失败: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("账号已存在")
	}

	plainPassword, err := auth.DecryptPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("密码解密失败: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	uid := s.idGen.Generate()
	if err := s.userRepo.InsertUser(ctx, &entity.User{
		UID: uid, Account: req.Account, Password: string(hash), Username: req.Username,
		Gender: &req.Gender, Region: &req.Region, Bio: &req.Bio, Avatar: &req.Avatar,
	}); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 注册成功: uid=%s", uid)
	return &pb.RegisterResponse{Uid: uid, Account: req.Account, Username: req.Username}, nil
}

func (s *userImpl) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	log.InfoContextf(ctx, "[API] 登录: account=%s", req.Account)

	u, err := s.userRepo.FindUserByAccount(ctx, req.Account)
	if err != nil {
		return nil, fmt.Errorf("账号或密码错误")
	}
	plainPassword, err := auth.DecryptPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("密码解密失败: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plainPassword)); err != nil {
		return nil, fmt.Errorf("账号或密码错误")
	}

	token, err := auth.SignToken(u.UID, u.Account, "web")
	if err != nil {
		return nil, fmt.Errorf("Token签发失败")
	}

	return &pb.LoginResponse{
		Token: token, ExpiresAt: time.Now().Add(7 * 24 * time.Hour).Unix(),
		User: &pb.User{
			Uid: u.UID, Account: u.Account, Username: u.Username,
			Gender: entity.PtrVal(u.Gender), Avatar: entity.PtrVal(u.Avatar),
			Region: entity.PtrVal(u.Region), Email: entity.PtrVal(u.Email), Bio: entity.PtrVal(u.Bio),
		},
	}, nil
}

func (s *userImpl) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
	// 隐私校验：仅本人或好友可查看完整档案
	requester := getUID(ctx)
	if requester != "" && requester != req.Uid {
		friendship, _ := s.friendRepo.FindFriendshipByUsers(ctx, requester, req.Uid)
		if friendship == nil {
			return nil, fmt.Errorf("无权查看该用户档案")
		}
	}
	u, err := s.userRepo.FindUserByUID(ctx, req.Uid)
	if err != nil {
		return nil, fmt.Errorf("用户不存在")
	}
	return &pb.GetUserInfoResponse{
		User: &pb.User{
			Uid: u.UID, Account: u.Account, Username: u.Username,
			Gender: entity.PtrVal(u.Gender), Avatar: entity.PtrVal(u.Avatar),
			Region: entity.PtrVal(u.Region), Email: entity.PtrVal(u.Email), Bio: entity.PtrVal(u.Bio),
		},
	}, nil
}

func main() {
	// ① tRPC 框架初始化（必须在 goredis.New 之前，否则 client 配置未加载）
	s := trpc.NewServer()

	dsn := GetDSN()
	db, err := mysql.NewDB(dsn)
	if err != nil {
		log.Fatalf("[API] MySQL 初始化失败: %v", err)
	}
	defer db.Close()

	workerID := int64(1)
	if s := os.Getenv("SNOWFLAKE_WORKER_ID"); s != "" {
		if v, _ := strconv.ParseInt(s, 10, 64); v >= 1 && v <= 1023 {
			workerID = v
		}
	}
	idGen, err := idgen.NewSnowflake(workerID)
	if err != nil {
		log.Fatalf("[API] Snowflake 初始化失败: %v", err)
	}

	userRepo := mysql.NewUserRepo(db)
	friendRepo := mysql.NewFriendRepo(db)
	msgRepo := mysql.NewMessageRepo(db)
	groupRepo := mysql.NewGroupRepo(db)
	fileRepo := mysql.NewFileRepo(db)

	svc := &userImpl{userRepo: userRepo, friendRepo: friendRepo, idGen: idGen}
	friendSvc := &friendImpl{friendRepo: friendRepo, idGen: idGen}
	chatRepo := mysql.NewPrivateChatRepo(db)
	msgSvc := &messageImpl{msgRepo: msgRepo, chatRepo: chatRepo, groupRepo: groupRepo}
	groupSvc := &groupImpl{groupRepo: groupRepo, idGen: idGen}
	fileSvc := &fileImpl{fileRepo: fileRepo, idGen: idGen}

	// Redis（统一走 tRPC 框架）
	universalCli, err := goredis.New("echat.redis.service")
	if err != nil {
		log.Fatalf("[API] Redis 初始化失败: %v", err)
	}
	redisCli, ok := universalCli.(*redis.Client)
	if !ok {
		log.Fatalf("[API] Redis 客户端类型异常")
	}
	onlineRepo := sdkredis.NewOnlineRepo(redisCli)

	// 初始化 RSA 密钥对
	if err := auth.InitRSA(); err != nil {
		log.Fatalf("[API] RSA 初始化失败: %v", err)
	}

	// ─── 观测: Metrics + Profiling ───
	obsShutdown, err := observability.Init(observability.InitConfig{
		ServiceName:    "echat-api",
		ServiceVersion: "1.0.0",
		OTLPEndpoint:   envOr("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4318"),
		PprofPort:      6061,
	})
	if err != nil {
		log.Warnf("[API] 观测初始化失败 (metrics): %v", err)
	}
	if obsShutdown != nil {
		defer func() {
			if err := obsShutdown(context.Background()); err != nil {
				log.Warnf("[API] 观测关闭失败: %v", err)
			}
		}()
	}
	if err := observability.InitBusinessMetrics(); err != nil {
		log.Warnf("[API] 业务指标初始化失败: %v", err)
	}

	pb.RegisterUserServiceService(s, svc)
	RegisterFriendServiceService(s, friendSvc)
	RegisterMessageServiceService(s, msgSvc)
	RegisterGroupServiceService(s, groupSvc)
	RegisterFileServiceService(s, fileSvc)
	RegisterAuthServiceService(s, &authImpl{})

	// 为 HTTP RESTful 传输注册同名服务（ServiceDesc.ServiceName 必须匹配 trpc_go.yaml HTTP 传输名）
	s.Register(&userServiceHTTPDesc, svc)
	s.Register(&friendServiceHTTPDesc, friendSvc)
	s.Register(&messageServiceHTTPDesc, msgSvc)
	s.Register(&groupServiceHTTPDesc, groupSvc)
	s.Register(&fileServiceHTTPDesc, fileSvc)

	// 额外 API（13 个新端点）
	extraSvc := &ExtraService{
		User:   &extraUserImpl{userRepo: userRepo},
		Friend: &extraFriendImpl{friendRepo: friendRepo},
		Message: &extraMessageImpl{
			msgRepo: msgRepo, privateRepo: chatRepo,
			groupMsgRepo: mysql.NewGroupMessageRepo(db),
		},
		Group: &extraGroupImpl{groupRepo: groupRepo, idGen: idGen},
		File:  &extraFileImpl{fileRepo: fileRepo, idGen: idGen},
		Misc: &extraMiscImpl{
			userRepo: userRepo, fileRepo: fileRepo, groupMsgRepo: mysql.NewGroupMessageRepo(db),
		},
		Final: &extraFinalImpl{
			userRepo: userRepo, friendRepo: friendRepo, privateRepo: chatRepo,
			groupRepo: groupRepo, groupMsgRepo: mysql.NewGroupMessageRepo(db),
			fileRepo: fileRepo, onlineRepo: onlineRepo,
		},
		Chat: &extraChatImpl{
			privateRepo: chatRepo, groupRepo: groupRepo, groupMsgRepo: mysql.NewGroupMessageRepo(db),
			onlineRepo: onlineRepo, userRepo: userRepo,
		},
	}
	RegisterExtraService(s, extraSvc)
	s.Register(&extraServiceHTTPDesc, extraSvc)

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[API] 收到信号 %v，正在优雅关机...", <-ch)
		s.Close(nil)
	}()

	log.Info("[API] 启动中...(Ctrl+C 停止)")
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[API] 已停止")
}

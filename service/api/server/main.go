package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	trpc "trpc.group/trpc-go/trpc-go"
	"trpc.group/trpc-go/trpc-go/log"

	pb "echat/service/api/stub"
	"echat/service/api/server/repository"
)

// userImpl 实现 UserService 接口
type userImpl struct {
	pb.UnimplementedUserService
	userRepo *repository.UserRepo
}

// Login 登录（account + password 校验）
func (s *userImpl) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	log.InfoContextf(ctx, "[API服务] 收到登录请求: account=%s", req.Account)

	// ★ 根据 account 查询用户
	userRow, err := s.userRepo.FindByAccount(req.Account)
	if err != nil {
		log.ErrorContextf(ctx, "[API服务] 账号不存在: %s", req.Account)
		return nil, fmt.Errorf("账号或密码错误")
	}

	// ★ 校验密码（生产环境用 bcrypt）
	if userRow.Password != req.Password {
		log.ErrorContextf(ctx, "[API服务] 密码错误: account=%s", req.Account)
		return nil, fmt.Errorf("账号或密码错误")
	}

	log.InfoContextf(ctx, "[API服务] 登录成功: uid=%s, username=%s", userRow.UID, userRow.Username)

	return &pb.LoginResponse{
		Token: "eyJhbGciOiJIUzI1NiJ9." + userRow.UID,
		User: &pb.User{
			Uid:      userRow.UID,
			Account:  userRow.Account,
			Username: userRow.Username,
			Gender:   userRow.Gender,
			Avatar:   strPtr(userRow.Avatar),
			Region:   strPtr(userRow.Region),
			Email:    strPtr(userRow.Email),
			Bio:      strPtr(userRow.Bio),
		},
	}, nil
}

// GetUserInfo 获取用户信息
func (s *userImpl) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
	log.InfoContextf(ctx, "[API服务] 查询用户信息: uid=%s", req.Uid)

	userRow, err := s.userRepo.FindByUID(req.Uid)
	if err != nil {
		log.ErrorContextf(ctx, "[API服务] 用户不存在: uid=%s", req.Uid)
		return nil, fmt.Errorf("用户不存在")
	}

	return &pb.GetUserInfoResponse{
		User: &pb.User{
			Uid:      userRow.UID,
			Account:  userRow.Account,
			Username: userRow.Username,
			Gender:   userRow.Gender,
			Avatar:   strPtr(userRow.Avatar),
			Region:   strPtr(userRow.Region),
			Email:    strPtr(userRow.Email),
			Bio:      strPtr(userRow.Bio),
		},
	}, nil
}

// strPtr 将 *string 转为 string，nil 返回空串
func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func main() {
	// ① 初始化 MySQL 连接池（优先读取环境变量 MYSQL_DSN）
	dsn := GetDSN()
	db, err := NewDB(dsn)
	if err != nil {
		log.Fatalf("[API服务] 数据库初始化失败: %v", err)
	}
	defer db.Close()
	log.Info("[API服务] MySQL 连接成功")

	// ② 创建数据访问层
	userRepo := repository.NewUserRepo(db)

	// ③ 创建 service，注入依赖
	svc := &userImpl{
		userRepo: userRepo,
	}

	s := trpc.NewServer()
	pb.RegisterUserServiceService(s, svc)

	// 优雅关机
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Infof("[API服务] 收到信号 %v，正在优雅关机...", <-ch)
		s.Close(nil)
	}()

	log.Info("[API服务] 启动中...(Ctrl+C 停止)")
	if err := s.Serve(); err != nil {
		log.Error(err)
	}
	log.Info("[API服务] 已停止")
}

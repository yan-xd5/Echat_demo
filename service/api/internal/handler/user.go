package handler

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"trpc.group/trpc-go/trpc-go/log"

	sdkredis "echat/sdk/repository/redis"
	"echat/sdk/usecase/auth"
	"echat/sdk/domain/entity"
	"echat/sdk/infrastructure/idgen"
	"echat/sdk/repository/mysql"
	"echat/service/api/internal/shared"
	pb "echat/service/api/stub"
)

// UserImpl 实现 UserService 接口。
type UserImpl struct {
	pb.UnimplementedUserService
	UserRepo   *mysql.UserRepo
	FriendRepo *mysql.FriendRepo
	IDGen      *idgen.Snowflake
	CacheRepo  *sdkredis.CacheRepo
}

func NewUserImpl(userRepo *mysql.UserRepo, friendRepo *mysql.FriendRepo, idGen *idgen.Snowflake, cacheRepo *sdkredis.CacheRepo) *UserImpl {
	return &UserImpl{UserRepo: userRepo, FriendRepo: friendRepo, IDGen: idGen, CacheRepo: cacheRepo}
}

func (s *UserImpl) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	log.InfoContextf(ctx, "[API] 注册: account=%s", req.Account)

	exists, err := s.UserRepo.ExistsByAccount(ctx, req.Account)
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

	uid := s.IDGen.Generate()
	if err := s.UserRepo.InsertUser(ctx, &entity.User{
		UID: uid, Account: req.Account, Password: string(hash), Username: req.Username,
		Gender: &req.Gender, Region: &req.Region, Bio: &req.Bio, Avatar: &req.Avatar,
	}); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 注册成功: uid=%s", uid)
	return &pb.RegisterResponse{Uid: uid, Account: req.Account, Username: req.Username}, nil
}

func (s *UserImpl) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	log.InfoContextf(ctx, "[API] 登录: account=%s", req.Account)

	u, err := s.UserRepo.FindUserByAccount(ctx, req.Account)
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

func (s *UserImpl) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
	requester := shared.GetUID(ctx)
	if requester != "" && requester != req.Uid {
		friendship, _ := s.FriendRepo.FindFriendshipByUsers(ctx, requester, req.Uid)
		if friendship == nil {
			return nil, fmt.Errorf("无权查看该用户档案")
		}
	}

	// 缓存优先
	var u *entity.User
	if cached, ok := s.CacheRepo.GetUser(ctx, req.Uid); ok {
		u = cached
	} else {
		var err error
		u, err = s.UserRepo.FindUserByUID(ctx, req.Uid)
		if err != nil {
			return nil, fmt.Errorf("用户不存在")
		}
		s.CacheRepo.SetUser(ctx, req.Uid, u)
	}

	return &pb.GetUserInfoResponse{
		User: &pb.User{
			Uid: u.UID, Account: u.Account, Username: u.Username,
			Gender: entity.PtrVal(u.Gender), Avatar: entity.PtrVal(u.Avatar),
			Region: entity.PtrVal(u.Region), Email: entity.PtrVal(u.Email), Bio: entity.PtrVal(u.Bio),
		},
	}, nil
}

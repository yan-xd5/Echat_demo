package restful

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/domain/entity"
	"echat/sdk/repository/mysql"
	sdkredis "echat/sdk/repository/redis"
	"echat/service/api/internal/shared"
)

// ExtraUserImpl 用户相关的额外 RESTful API。
type ExtraUserImpl struct {
	UserRepo  *mysql.UserRepo
	CacheRepo *sdkredis.CacheRepo
}

// NewExtraUserImpl 创建 ExtraUserImpl。
func NewExtraUserImpl(userRepo *mysql.UserRepo, cacheRepo *sdkredis.CacheRepo) *ExtraUserImpl {
	return &ExtraUserImpl{UserRepo: userRepo, CacheRepo: cacheRepo}
}

type updateProfileReq struct {
	Username *string `json:"username,omitempty"`
	Gender   *string `json:"gender,omitempty"`
	Region   *string `json:"region,omitempty"`
	Email    *string `json:"email,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
	Bio      *string `json:"bio,omitempty"`
}

// UpdateProfile 更新用户资料。
func (s *ExtraUserImpl) UpdateProfile(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req updateProfileReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数格式错误"})
		return
	}
	user := &entity.User{UID: uid}
	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Gender != nil {
		user.Gender = req.Gender
	}
	if req.Region != nil {
		user.Region = req.Region
	}
	if req.Email != nil {
		user.Email = req.Email
	}
	if req.Avatar != nil {
		user.Avatar = req.Avatar
	}
	if req.Bio != nil {
		user.Bio = req.Bio
	}
	if err := s.UserRepo.UpdateUser(ctx, user); err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	s.CacheRepo.DeleteUser(ctx, uid)
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "修改成功"})
}

// SearchUser 按用户名搜索用户。
func (s *ExtraUserImpl) SearchUser(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Keyword string `json:"keyword"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	keyword := req.Keyword
	if keyword == "" {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "缺少 keyword"})
		return
	}
	users, err := s.UserRepo.FindUserByUsername(ctx, keyword)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	type userInfo struct {
		UID      string `json:"uid"`
		Account  string `json:"account"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	list := make([]userInfo, 0, len(users))
	for _, u := range users {
		list = append(list, userInfo{
			UID: u.UID, Account: u.Account, Username: u.Username,
			Avatar: entity.PtrVal(u.Avatar),
		})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "users": list})
}

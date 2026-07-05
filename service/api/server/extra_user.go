package main

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/entity"
	"echat/sdk/mysql"
)

type extraUserImpl struct {
	userRepo *mysql.UserRepo
}

type updateProfileReq struct {
	Username *string `json:"username,omitempty"`
	Gender   *string `json:"gender,omitempty"`
	Region   *string `json:"region,omitempty"`
	Email    *string `json:"email,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
	Bio      *string `json:"bio,omitempty"`
}

func (s *extraUserImpl) UpdateProfile(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req updateProfileReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "参数格式错误"})
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
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		writeJSON(w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "message": "修改成功"})
}

func (s *extraUserImpl) SearchUser(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if getUID(ctx) == "" {
		writeJSON(w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Keyword string `json:"keyword"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	keyword := req.Keyword
	if keyword == "" {
		writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "缺少 keyword"})
		return
	}
	users, err := s.userRepo.FindUserByUsername(ctx, keyword)
	if err != nil {
		writeJSON(w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
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
	writeJSON(w, 200, map[string]interface{}{"code": 0, "users": list})
}

package main

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/mysql"
)

type extraFriendImpl struct {
	friendRepo *mysql.FriendRepo
}

func (s *extraFriendImpl) BlacklistFriend(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Fid string `json:"fid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Fid == "" {
		writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	f, err := s.friendRepo.FindFriendshipByFID(ctx, req.Fid)
	if err != nil || f == nil {
		writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "好友关系不存在"})
		return
	}
	if f.UID != uid && f.ToUID != uid {
		writeJSON(w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	if err := s.friendRepo.SaveBlacklist(ctx, req.Fid, uid, true); err != nil {
		writeJSON(w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "message": "已拉黑"})
}

func (s *extraFriendImpl) UnblacklistFriend(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Fid string `json:"fid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Fid == "" {
		writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	f, err := s.friendRepo.FindFriendshipByFID(ctx, req.Fid)
	if err != nil || f == nil {
		writeJSON(w, 400, map[string]interface{}{"code": 1, "message": "好友关系不存在"})
		return
	}
	if f.UID != uid && f.ToUID != uid {
		writeJSON(w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	if err := s.friendRepo.SaveBlacklist(ctx, req.Fid, uid, false); err != nil {
		writeJSON(w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"code": 0, "message": "已取消拉黑"})
}

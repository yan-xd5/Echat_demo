package restful

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/repository/mysql"
	"echat/service/api/internal/shared"
)

// ExtraFriendImpl 好友相关的额外 RESTful API。
type ExtraFriendImpl struct {
	FriendRepo *mysql.FriendRepo
}

// NewExtraFriendImpl 创建 ExtraFriendImpl。
func NewExtraFriendImpl(friendRepo *mysql.FriendRepo) *ExtraFriendImpl {
	return &ExtraFriendImpl{FriendRepo: friendRepo}
}

// BlacklistFriend 拉黑好友。
func (s *ExtraFriendImpl) BlacklistFriend(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Fid string `json:"fid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Fid == "" {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	f, err := s.FriendRepo.FindFriendshipByFID(ctx, req.Fid)
	if err != nil || f == nil {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "好友关系不存在"})
		return
	}
	if f.UID != uid && f.ToUID != uid {
		shared.WriteJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	if err := s.FriendRepo.SaveBlacklist(ctx, req.Fid, uid, true); err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已拉黑"})
}

// UnblacklistFriend 取消拉黑好友。
func (s *ExtraFriendImpl) UnblacklistFriend(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Fid string `json:"fid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Fid == "" {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	f, err := s.FriendRepo.FindFriendshipByFID(ctx, req.Fid)
	if err != nil || f == nil {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "好友关系不存在"})
		return
	}
	if f.UID != uid && f.ToUID != uid {
		shared.WriteJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	if err := s.FriendRepo.SaveBlacklist(ctx, req.Fid, uid, false); err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已取消拉黑"})
}

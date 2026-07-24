package restful

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/domain/entity"
	"echat/sdk/repository/mysql"
	"echat/sdk/repository/redis"
	"echat/service/api/internal/shared"
)

// ExtraFinalImpl 最终批次额外 RESTful API（多依赖聚合）。
type ExtraFinalImpl struct {
	UserRepo     *mysql.UserRepo
	FriendRepo   *mysql.FriendRepo
	PrivateRepo  *mysql.PrivateChatRepo
	GroupRepo    *mysql.GroupRepo
	GroupMsgRepo *mysql.GroupMessageRepo
	FileRepo     *mysql.FileRepo
	OnlineRepo   *redis.OnlineRepo
	CacheRepo    *redis.CacheRepo
}

// NewExtraFinalImpl 创建 ExtraFinalImpl。
func NewExtraFinalImpl(
	userRepo *mysql.UserRepo, friendRepo *mysql.FriendRepo, privateRepo *mysql.PrivateChatRepo,
	groupRepo *mysql.GroupRepo, groupMsgRepo *mysql.GroupMessageRepo, fileRepo *mysql.FileRepo,
	onlineRepo *redis.OnlineRepo, cacheRepo *redis.CacheRepo,
) *ExtraFinalImpl {
	return &ExtraFinalImpl{
		UserRepo: userRepo, FriendRepo: friendRepo, PrivateRepo: privateRepo,
		GroupRepo: groupRepo, GroupMsgRepo: groupMsgRepo, FileRepo: fileRepo, OnlineRepo: onlineRepo,
		CacheRepo: cacheRepo,
	}
}

// SearchUserByRegion 按地区搜用户。
func (s *ExtraFinalImpl) SearchUserByRegion(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct{ Region string `json:"region"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Region == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("缺少 region"))
		return
	}
	users, err := s.UserRepo.FindUserByRegion(ctx, req.Region)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	type u struct {
		UID, Account, Username, Avatar string
	}
	list := make([]u, 0, len(users))
	for _, us := range users {
		list = append(list, u{us.UID, us.Account, us.Username, entity.PtrVal(us.Avatar)})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "users": list})
}

// DeleteAccount 注销账号。
func (s *ExtraFinalImpl) DeleteAccount(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	if err := s.UserRepo.DeleteUser(ctx, uid); err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	s.CacheRepo.DeleteUser(ctx, uid)
	s.CacheRepo.DeleteFriends(ctx, uid)
	s.CacheRepo.DeleteUserGroups(ctx, uid)
	shared.WriteJSON(ctx, w, 200, shared.MsgSuccess("账号已注销"))
}

// CancelFriendRequest 取消好友申请。
func (s *ExtraFinalImpl) CancelFriendRequest(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct{ ReqId string `json:"req_id"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.ReqId == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	fr, err := s.FriendRepo.FindFriendRequestByID(ctx, req.ReqId)
	if err != nil || fr == nil {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("申请不存在"))
		return
	}
	if fr.SenderUID != uid {
		shared.WriteJSON(ctx, w, 403, shared.MsgError("无权操作"))
		return
	}
	if fr.Status != entity.ReqStatusPending {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("申请已处理"))
		return
	}
	if err := s.FriendRepo.UpdateRequestStatus(ctx, req.ReqId, entity.ReqStatusRejected); err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	shared.WriteJSON(ctx, w, 200, shared.MsgSuccess("已取消"))
}

// BlacklistList 黑名单列表。
func (s *ExtraFinalImpl) BlacklistList(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	friends, err := s.FriendRepo.FindBlacklistedFriends(ctx, uid)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	type f struct {
		Fid, Uid string
	}
	list := make([]f, 0, len(friends))
	for _, fr := range friends {
		friendUID := fr.ToUID
		if fr.ToUID == uid {
			friendUID = fr.UID
		}
		list = append(list, f{fr.FID, friendUID})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "friends": list})
}

// UnreadMessages 查未读消息列表。
func (s *ExtraFinalImpl) UnreadMessages(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct {
		ChatId   string `json:"chat_id"`
		ChatType string `json:"chat_type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ChatId == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	type m struct {
		MsgId, Content, SenderUid string
		SendTime                  int64
	}
	var list []m
	if req.ChatType == "group" {
		msgs, err := s.GroupMsgRepo.FindUnreadMessagesByUser(ctx, req.ChatId, uid)
		if err != nil {
			shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
			return
		}
		for _, msg := range msgs {
			st := int64(0)
			if msg.SendTime != nil {
				st = *msg.SendTime
			}
			list = append(list, m{msg.MsgID, msg.Content, msg.SenderUID, st})
		}
	} else {
		msgs, err := s.PrivateRepo.FindUnreadMessagesByChat(ctx, req.ChatId, uid)
		if err != nil {
			shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
			return
		}
		for _, msg := range msgs {
			st := int64(0)
			if msg.SendTime != nil {
				st = *msg.SendTime
			}
			list = append(list, m{msg.MsgID, msg.Content, msg.SenderUID, st})
		}
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "messages": list})
}

// MessageReadUsers 查消息已读用户列表。
func (s *ExtraFinalImpl) MessageReadUsers(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct{ MsgId string `json:"msg_id"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.MsgId == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	uids, err := s.GroupMsgRepo.FindReadUsersByMessage(ctx, req.MsgId)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "uids": uids})
}

// MessageChatCount 查会话消息总数。
func (s *ExtraFinalImpl) MessageChatCount(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct {
		ChatId   string `json:"chat_id"`
		ChatType string `json:"chat_type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.ChatId == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	var count int64
	var err error
	if req.ChatType == "group" {
		count, err = s.GroupMsgRepo.GetMessageCountByGroup(ctx, req.ChatId)
	} else {
		count, err = s.PrivateRepo.GetMessageCountByChat(ctx, req.ChatId)
	}
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "count": count})
}

// OwnedGroups 我创建的群。
func (s *ExtraFinalImpl) OwnedGroups(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	groups, err := s.GroupRepo.FindGroupsByOwner(ctx, uid)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	type g struct {
		Gid, GroupName, GroupAvatar string
		MemberCount                 int
	}
	list := make([]g, 0, len(groups))
	for _, gr := range groups {
		cnt, _ := s.GroupRepo.GetMemberCount(ctx, gr.GID)
		list = append(list, g{gr.GID, gr.GroupName, entity.PtrVal(gr.GroupAvatar), cnt})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "groups": list})
}

// MuteList 群禁言列表。
func (s *ExtraFinalImpl) MuteList(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct{ Gid string `json:"gid"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Gid == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	// 检查是否是成员
	if _, err := s.GroupRepo.FindMember(ctx, req.Gid, uid); err != nil {
		shared.WriteJSON(ctx, w, 403, shared.MsgError("不是群成员"))
		return
	}
	records, err := s.GroupRepo.FindMuteRecordsByGroup(ctx, req.Gid)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	type mr struct {
		BanId, Uid string
		Duration   int
		StartTime  int64
	}
	list := make([]mr, 0, len(records))
	for _, rec := range records {
		st := int64(0)
		if rec.StartTime != nil {
			st = *rec.StartTime
		}
		list = append(list, mr{rec.BanID, rec.UID, rec.MuteDuration, st})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "mutes": list})
}

// GroupOnlineMembers 群在线成员。
func (s *ExtraFinalImpl) GroupOnlineMembers(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct{ Gid string `json:"gid"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Gid == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	uids, err := s.OnlineRepo.GetGroupOnlineMembers(ctx, req.Gid)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "online_uids": uids})
}

// MyGroupRequests 我的入群申请。
func (s *ExtraFinalImpl) MyGroupRequests(ctx context.Context) {
	w, _ := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	reqs, err := s.GroupRepo.FindRequestsByUser(ctx, uid)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	type myReq struct {
		ReqId, Gid, Status, ApplyText string
		CreateTime                    int64
	}
	list := make([]myReq, 0, len(reqs))
	for _, gr := range reqs {
		ct := int64(0)
		if gr.CreateTime != nil {
			ct = *gr.CreateTime
		}
		at := ""
		if gr.ApplyText != nil {
			at = *gr.ApplyText
		}
		list = append(list, myReq{gr.ReqID, gr.GID, string(gr.Status), at, ct})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "requests": list})
}

// AllGroupRequests 群全部入群申请（含已处理）。
func (s *ExtraFinalImpl) AllGroupRequests(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct{ Gid string `json:"gid"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Gid == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	op, _ := s.GroupRepo.FindMember(ctx, req.Gid, uid)
	if op == nil || (op.Role != entity.RoleOwner && op.Role != entity.RoleAdmin) {
		shared.WriteJSON(ctx, w, 403, shared.MsgError("无权查看"))
		return
	}
	reqs, err := s.GroupRepo.FindAllRequestsByGroup(ctx, req.Gid)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	type reqEntry struct {
		ReqId, Gid, ApplicantUid, ApproverUid, Status, ApplyText string
		CreateTime, HandleTime                                    int64
	}
	list := make([]reqEntry, 0, len(reqs))
	for _, gr := range reqs {
		ct, ht := int64(0), int64(0)
		if gr.CreateTime != nil {
			ct = *gr.CreateTime
		}
		if gr.HandleTime != nil {
			ht = *gr.HandleTime
		}
		at, au := "", ""
		if gr.ApplyText != nil {
			at = *gr.ApplyText
		}
		if gr.ApproverUID != nil {
			au = *gr.ApproverUID
		}
		list = append(list, reqEntry{gr.ReqID, gr.GID, gr.ApplicantUID, au, string(gr.Status), at, ct, ht})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "requests": list})
}

// DeleteFileAssociation 删除文件关联。
func (s *ExtraFinalImpl) DeleteFileAssociation(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, shared.MsgError("未登录"))
		return
	}
	var req struct{ AssociationId string `json:"association_id"` }
	json.NewDecoder(r.Body).Decode(&req)
	if req.AssociationId == "" {
		shared.WriteJSON(ctx, w, 400, shared.MsgError("参数错误"))
		return
	}
	// 校验所有权：仅创建者可删除
	assoc, err := s.FileRepo.FindFileAssociationByID(ctx, req.AssociationId)
	if err != nil || assoc == nil {
		shared.WriteJSON(ctx, w, 404, shared.MsgError("文件关联不存在"))
		return
	}
	if assoc.CreatorUID != uid {
		shared.WriteJSON(ctx, w, 403, shared.MsgError("无权删除此文件关联"))
		return
	}
	if err := s.FileRepo.DeleteFileAssociation(ctx, req.AssociationId); err != nil {
		shared.WriteJSON(ctx, w, 500, shared.MsgError(err.Error()))
		return
	}
	shared.WriteJSON(ctx, w, 200, shared.MsgSuccess("已删除"))
}

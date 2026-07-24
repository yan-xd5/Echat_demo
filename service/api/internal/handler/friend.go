package handler

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-go/log"

	sdkredis "echat/sdk/repository/redis"
	"echat/sdk/domain/entity"
	"echat/sdk/infrastructure/idgen"
	"echat/sdk/repository/mysql"
	pb "echat/service/api/stub"
	"echat/service/api/internal/shared"
)

// friendImpl 实现 FriendServiceService 接口。
type FriendImpl struct {
	FriendRepo *mysql.FriendRepo
	IDGen      *idgen.Snowflake
	CacheRepo  *sdkredis.CacheRepo
}

// ApplyFriend 发送好友申请。
func (s *FriendImpl) ApplyFriend(ctx context.Context, req *pb.ApplyFriendRequest) (*pb.ApplyFriendResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" || req.ToUid == "" {
		return nil, fmt.Errorf("参数错误")
	}
	if uid == req.ToUid {
		return nil, fmt.Errorf("不能添加自己为好友")
	}

	// 检查是否已是好友
	f, err := s.FriendRepo.FindFriendshipByUsers(ctx, uid, req.ToUid)
	if err != nil {
		return nil, fmt.Errorf("检查好友关系失败: %w", err)
	}
	if f != nil {
		return nil, fmt.Errorf("已是好友")
	}

	reqID := s.IDGen.Generate()
	if err := s.FriendRepo.SaveFriendRequest(ctx, &entity.FriendRequest{
		ReqID:       reqID,
		SenderUID:   uid,
		ReceiverUID: req.ToUid,
		Status:      entity.ReqStatusPending,
		ApplyText:   &req.ApplyText,
	}); err != nil {
		return nil, fmt.Errorf("保存申请失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 好友申请: %s → %s, req_id=%s", uid, req.ToUid, reqID)
	return &pb.ApplyFriendResponse{ReqId: reqID}, nil
}

// AcceptFriend 接受好友申请。
func (s *FriendImpl) AcceptFriend(ctx context.Context, req *pb.AcceptFriendRequest) (*pb.AcceptFriendResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" || req.ReqId == "" {
		return nil, fmt.Errorf("参数错误")
	}

	fr, err := s.FriendRepo.FindFriendRequestByID(ctx, req.ReqId)
	if err != nil || fr == nil {
		return nil, fmt.Errorf("申请不存在")
	}
	if fr.ReceiverUID != uid {
		return nil, fmt.Errorf("无权操作")
	}
	if fr.Status != entity.ReqStatusPending {
		return nil, fmt.Errorf("申请已处理")
	}

	// 排序 UID（friends 表 uid < to_uid）
	uid1, uid2 := fr.SenderUID, fr.ReceiverUID
	if uid1 > uid2 {
		uid1, uid2 = uid2, uid1
	}
	fid := s.IDGen.Generate()
	friendship := &entity.Friends{FID: fid, UID: uid1, ToUID: uid2}
	chat := &entity.PrivateChat{PID: fid, UID1: uid1, UID2: uid2}

	if err := s.FriendRepo.AcceptFriendRequestWithChat(ctx, req.ReqId, time.Now().UnixMilli(), friendship, chat); err != nil {
		return nil, fmt.Errorf("接受申请失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 好友申请通过: req_id=%s, fid=%s", req.ReqId, fid)
	s.CacheRepo.DeleteFriends(ctx, fr.SenderUID)
	s.CacheRepo.DeleteFriends(ctx, fr.ReceiverUID)
	return &pb.AcceptFriendResponse{Fid: fid}, nil
}

// RejectFriend 拒绝好友申请。
func (s *FriendImpl) RejectFriend(ctx context.Context, req *pb.RejectFriendRequest) (*pb.RejectFriendResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" || req.ReqId == "" {
		return nil, fmt.Errorf("参数错误")
	}

	fr, err := s.FriendRepo.FindFriendRequestByID(ctx, req.ReqId)
	if err != nil || fr == nil {
		return nil, fmt.Errorf("申请不存在")
	}
	if fr.ReceiverUID != uid {
		return nil, fmt.Errorf("无权操作")
	}

	if err := s.FriendRepo.UpdateRequestStatus(ctx, req.ReqId, entity.ReqStatusRejected); err != nil {
		return nil, fmt.Errorf("拒绝申请失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 好友申请已拒绝: req_id=%s", req.ReqId)
	return &pb.RejectFriendResponse{}, nil
}

// DeleteFriend 删除好友。
func (s *FriendImpl) DeleteFriend(ctx context.Context, req *pb.DeleteFriendRequest) (*pb.DeleteFriendResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" || req.Fid == "" {
		return nil, fmt.Errorf("参数错误")
	}
	f, err := s.FriendRepo.FindFriendshipByFID(ctx, req.Fid)
	if err != nil || f == nil {
		return nil, fmt.Errorf("好友关系不存在")
	}
	if f.UID != uid && f.ToUID != uid {
		return nil, fmt.Errorf("无权删除")
	}
	if err := s.FriendRepo.DeleteFriendshipWithChat(ctx, req.Fid); err != nil {
		return nil, fmt.Errorf("删除好友失败: %w", err)
	}
	log.InfoContextf(ctx, "[API] 删除好友: fid=%s", req.Fid)
	s.CacheRepo.DeleteFriends(ctx, f.UID)
	s.CacheRepo.DeleteFriends(ctx, f.ToUID)
	return &pb.DeleteFriendResponse{}, nil
}

// ListFriends 好友列表。
func (s *FriendImpl) ListFriends(ctx context.Context, req *pb.ListFriendsRequest) (*pb.ListFriendsResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	// 缓存优先
	var friends []*entity.Friends
	if cached, ok := s.CacheRepo.GetFriends(ctx, uid); ok {
		friends = cached
	} else {
		var err error
		friends, err = s.FriendRepo.FindFriendshipByUID(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("查好友列表失败: %w", err)
		}
		s.CacheRepo.SetFriends(ctx, uid, friends)
	}

	var list []*pb.FriendInfo
	for _, f := range friends {
		friendUID := f.ToUID
		isCurrentUID := true // 当前用户是 uid（较小 UID）
		if f.UID == uid {
			friendUID = f.ToUID
		} else {
			friendUID = f.UID
			isCurrentUID = false // 当前用户是 to_uid
		}
		remark := ""
		if isCurrentUID {
			if f.Remark != nil {
				remark = *f.Remark
			}
		} else {
			if f.ToRemark != nil {
				remark = *f.ToRemark
			}
		}
		black := false
		if isCurrentUID {
			if f.IsBlacklist != nil {
				black = *f.IsBlacklist
			}
		} else {
			if f.ToIsBlacklist != nil {
				black = *f.ToIsBlacklist
			}
		}
		list = append(list, &pb.FriendInfo{
			Fid: f.FID, Uid: friendUID, Remark: remark, IsBlacklist: black,
		})
	}

	return &pb.ListFriendsResponse{Friends: list}, nil
}

// ListRequests 好友申请列表。
func (s *FriendImpl) ListRequests(ctx context.Context, req *pb.ListRequestsRequest) (*pb.ListRequestsResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	var reqs []*entity.FriendRequest
	var err error
	if req.Direction == "sent" {
		reqs, err = s.FriendRepo.FindFriendRequestBySender(ctx, uid)
	} else {
		reqs, err = s.FriendRepo.FindFriendRequestByReceiver(ctx, uid)
	}
	if err != nil {
		return nil, fmt.Errorf("查申请列表失败: %w", err)
	}

	var list []*pb.FriendRequestInfo
	for _, r := range reqs {
		ct := int64(0)
		if r.CreateTime != nil {
			ct = *r.CreateTime
		}
		at := ""
		if r.ApplyText != nil {
			at = *r.ApplyText
		}
		list = append(list, &pb.FriendRequestInfo{
			ReqId: r.ReqID, SenderUid: r.SenderUID, ApplyText: at,
			Status: string(r.Status), CreateTime: ct,
		})
	}

	return &pb.ListRequestsResponse{Requests: list}, nil
}


func NewFriendImpl(FriendRepo *mysql.FriendRepo, IDGen *idgen.Snowflake, cacheRepo *sdkredis.CacheRepo) *FriendImpl {
	return &FriendImpl{FriendRepo: FriendRepo, IDGen: IDGen, CacheRepo: cacheRepo}
}

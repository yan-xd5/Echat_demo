package main

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/entity"
	"echat/sdk/idgen"
	"echat/sdk/mysql"
	pb "echat/service/api/stub"
)

// friendImpl 实现 FriendServiceService 接口。
type friendImpl struct {
	friendRepo *mysql.FriendRepo
	idGen      *idgen.Snowflake
}

// ApplyFriend 发送好友申请。
func (s *friendImpl) ApplyFriend(ctx context.Context, req *pb.ApplyFriendRequest) (*pb.ApplyFriendResponse, error) {
	uid := getUID(ctx)
	if uid == "" || req.ToUid == "" {
		return nil, fmt.Errorf("参数错误")
	}
	if uid == req.ToUid {
		return nil, fmt.Errorf("不能添加自己为好友")
	}

	// 检查是否已是好友
	f, err := s.friendRepo.FindFriendshipByUsers(ctx, uid, req.ToUid)
	if err != nil {
		return nil, fmt.Errorf("检查好友关系失败: %w", err)
	}
	if f != nil {
		return nil, fmt.Errorf("已是好友")
	}

	reqID := s.idGen.Generate()
	if err := s.friendRepo.SaveFriendRequest(ctx, &entity.FriendRequest{
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
func (s *friendImpl) AcceptFriend(ctx context.Context, req *pb.AcceptFriendRequest) (*pb.AcceptFriendResponse, error) {
	uid := getUID(ctx)
	if uid == "" || req.ReqId == "" {
		return nil, fmt.Errorf("参数错误")
	}

	fr, err := s.friendRepo.FindFriendRequestByID(ctx, req.ReqId)
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
	fid := s.idGen.Generate()
	friendship := &entity.Friends{FID: fid, UID: uid1, ToUID: uid2}
	chat := &entity.PrivateChat{PID: fid, UID1: uid1, UID2: uid2}

	if err := s.friendRepo.AcceptFriendRequestWithChat(ctx, req.ReqId, time.Now().UnixMilli(), friendship, chat); err != nil {
		return nil, fmt.Errorf("接受申请失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 好友申请通过: req_id=%s, fid=%s", req.ReqId, fid)
	return &pb.AcceptFriendResponse{Fid: fid}, nil
}

// RejectFriend 拒绝好友申请。
func (s *friendImpl) RejectFriend(ctx context.Context, req *pb.RejectFriendRequest) (*pb.RejectFriendResponse, error) {
	uid := getUID(ctx)
	if uid == "" || req.ReqId == "" {
		return nil, fmt.Errorf("参数错误")
	}

	fr, err := s.friendRepo.FindFriendRequestByID(ctx, req.ReqId)
	if err != nil || fr == nil {
		return nil, fmt.Errorf("申请不存在")
	}
	if fr.ReceiverUID != uid {
		return nil, fmt.Errorf("无权操作")
	}

	if err := s.friendRepo.UpdateRequestStatus(ctx, req.ReqId, entity.ReqStatusRejected); err != nil {
		return nil, fmt.Errorf("拒绝申请失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 好友申请已拒绝: req_id=%s", req.ReqId)
	return &pb.RejectFriendResponse{}, nil
}

// DeleteFriend 删除好友。
func (s *friendImpl) DeleteFriend(ctx context.Context, req *pb.DeleteFriendRequest) (*pb.DeleteFriendResponse, error) {
	uid := getUID(ctx)
	if uid == "" || req.Fid == "" {
		return nil, fmt.Errorf("参数错误")
	}
	f, err := s.friendRepo.FindFriendshipByFID(ctx, req.Fid)
	if err != nil || f == nil {
		return nil, fmt.Errorf("好友关系不存在")
	}
	if f.UID != uid && f.ToUID != uid {
		return nil, fmt.Errorf("无权删除")
	}
	if err := s.friendRepo.DeleteFriendshipWithChat(ctx, req.Fid); err != nil {
		return nil, fmt.Errorf("删除好友失败: %w", err)
	}
	log.InfoContextf(ctx, "[API] 删除好友: fid=%s", req.Fid)
	return &pb.DeleteFriendResponse{}, nil
}

// ListFriends 好友列表。
func (s *friendImpl) ListFriends(ctx context.Context, req *pb.ListFriendsRequest) (*pb.ListFriendsResponse, error) {
	uid := getUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	friends, err := s.friendRepo.FindFriendshipByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("查好友列表失败: %w", err)
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
func (s *friendImpl) ListRequests(ctx context.Context, req *pb.ListRequestsRequest) (*pb.ListRequestsResponse, error) {
	uid := getUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	var reqs []*entity.FriendRequest
	var err error
	if req.Direction == "sent" {
		reqs, err = s.friendRepo.FindFriendRequestBySender(ctx, uid)
	} else {
		reqs, err = s.friendRepo.FindFriendRequestByReceiver(ctx, uid)
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

type uidKey struct{}

// getUID 从 ctx 提取当前用户 UID（由 JWT filter 注入）。
func getUID(ctx context.Context) string {
	if v, ok := ctx.Value(uidKey{}).(string); ok {
		return v
	}
	return ""
}


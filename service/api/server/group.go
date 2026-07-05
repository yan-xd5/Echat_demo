package main

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/entity"
	"echat/sdk/idgen"
	"echat/sdk/mysql"
	pb "echat/service/api/stub"
)

// groupImpl 实现 GroupServiceService 接口。
type groupImpl struct {
	groupRepo *mysql.GroupRepo
	idGen     *idgen.Snowflake
}

// CreateGroup 创建群聊。
func (s *groupImpl) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	uid := getUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	gid := s.idGen.Generate()
	if err := s.groupRepo.SaveGroup(ctx, &entity.GroupChat{
		GID: gid, GroupName: req.GroupName, ManagerUID: uid, GroupIntro: &req.GroupIntro,
	}); err != nil {
		return nil, fmt.Errorf("创建群聊失败: %w", err)
	}

	// 创建者自动成为 owner
	if err := s.groupRepo.SaveMember(ctx, &entity.GroupMember{
		UID: uid, GID: gid, Role: entity.RoleOwner,
	}); err != nil {
		// 保存成员失败 → 删除刚创建的群，避免孤儿群
		s.groupRepo.DeleteGroup(ctx, gid)
		return nil, fmt.Errorf("创建群聊失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 创建群聊: gid=%s, name=%s, owner=%s", gid, req.GroupName, uid)
	return &pb.CreateGroupResponse{Gid: gid}, nil
}

// JoinGroup 申请加入群聊（需群主/管理员审批）。
func (s *groupImpl) JoinGroup(ctx context.Context, req *pb.JoinGroupRequest) (*pb.JoinGroupResponse, error) {
	uid := getUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	// 检查群是否存在
	if _, err := s.groupRepo.FindGroupByGID(ctx, req.Gid); err != nil {
		return nil, fmt.Errorf("群不存在")
	}

	// 检查是否已是成员
	m, err := s.groupRepo.FindMember(ctx, req.Gid, uid)
	if err != nil {
		return nil, fmt.Errorf("检查成员状态失败: %w", err)
	}
	if m != nil {
		return nil, fmt.Errorf("已是群成员")
	}

	// 检查是否有待处理的申请（防重复）
	reqs, err := s.groupRepo.FindRequestsByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("检查申请状态失败: %w", err)
	}
	for _, r := range reqs {
		if r.GID == req.Gid && r.Status == entity.ReqStatusPending {
			return nil, fmt.Errorf("已有待审批的申请")
		}
	}

	// 创建入群申请
	reqID := s.idGen.Generate()
	applyText := req.ApplyText
	if err := s.groupRepo.SaveGroupRequest(ctx, &entity.GroupJoinRequest{
		ReqID:        reqID,
		GID:          req.Gid,
		ApplicantUID: uid,
		Status:       entity.ReqStatusPending,
		ApplyText:    &applyText,
	}); err != nil {
		return nil, fmt.Errorf("提交申请失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 入群申请: gid=%s, uid=%s, req=%s", req.Gid, uid, reqID)
	return &pb.JoinGroupResponse{ReqId: reqID}, nil
}

// LeaveGroup 退出群聊。
func (s *groupImpl) LeaveGroup(ctx context.Context, req *pb.LeaveGroupRequest) (*pb.LeaveGroupResponse, error) {
	uid := getUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	// 群主不能直接退出
	m, err := s.groupRepo.FindMember(ctx, req.Gid, uid)
	if err != nil {
		return nil, fmt.Errorf("检查成员状态失败: %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("不是群成员")
	}
	if m.Role == entity.RoleOwner {
		return nil, fmt.Errorf("群主不能直接退出，请先转让群主")
	}
	if err := s.groupRepo.RemoveMember(ctx, req.Gid, uid); err != nil {
		return nil, fmt.Errorf("退出群聊失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 退出群聊: gid=%s, uid=%s", req.Gid, uid)
	return &pb.LeaveGroupResponse{}, nil
}

// GetMembers 获取群成员列表。
func (s *groupImpl) GetMembers(ctx context.Context, req *pb.GetMembersRequest) (*pb.GetMembersResponse, error) {
	uid := getUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}
	if _, err := s.groupRepo.FindMember(ctx, req.Gid, uid); err != nil {
		return nil, fmt.Errorf("不是群成员")
	}
	members, err := s.groupRepo.FindMembersByGroup(ctx, req.Gid)
	if err != nil {
		return nil, fmt.Errorf("查成员列表失败: %w", err)
	}

	var list []*pb.MemberInfo
	for _, m := range members {
		jt := int64(0)
		if m.JoinTime != nil {
			jt = *m.JoinTime
		}
		list = append(list, &pb.MemberInfo{
			Uid: m.UID, Role: string(m.Role), JoinTime: jt,
		})
	}

	return &pb.GetMembersResponse{Members: list}, nil
}

// GetMyGroups 获取我的群列表。
func (s *groupImpl) GetMyGroups(ctx context.Context, req *pb.GetMyGroupsRequest) (*pb.GetMyGroupsResponse, error) {
	uid := getUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	members, err := s.groupRepo.FindGroupsByUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("查群列表失败: %w", err)
	}

	// 批量查询群信息和成员数（避免 N+1）
	gids := make([]string, 0, len(members))
	for _, m := range members {
		gids = append(gids, m.GID)
	}
	groupMap, err := s.groupRepo.FindGroupsByGIDs(ctx, gids)
	if err != nil {
		return nil, fmt.Errorf("查群信息失败: %w", err)
	}
	countMap, err := s.groupRepo.GetMemberCounts(ctx, gids)
	if err != nil {
		return nil, fmt.Errorf("查成员数失败: %w", err)
	}

	var list []*pb.GroupInfo
	for _, m := range members {
		g, ok := groupMap[m.GID]
		if !ok {
			continue
		}
		list = append(list, &pb.GroupInfo{
			Gid: m.GID, GroupName: g.GroupName, MyRole: string(m.Role),
			MemberCount: int32(countMap[m.GID]),
		})
	}

	return &pb.GetMyGroupsResponse{Groups: list}, nil
}

package main

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/entity"
	"echat/sdk/idgen"
	"echat/sdk/mysql"
)

type extraGroupImpl struct {
	groupRepo *mysql.GroupRepo
	idGen     *idgen.Snowflake
}

func (s *extraGroupImpl) KickMember(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Gid     string `json:"gid"`
		TargetUid string `json:"target_uid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Gid == "" || req.TargetUid == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	// 检查操作者是否是管理员或群主
	op, _ := s.groupRepo.FindMember(ctx, req.Gid, uid)
	if op == nil || (op.Role != entity.RoleOwner && op.Role != entity.RoleAdmin) {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	// 不能踢群主
	target, _ := s.groupRepo.FindMember(ctx, req.Gid, req.TargetUid)
	if target != nil && target.Role == entity.RoleOwner {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "不能踢出群主"})
		return
	}
	if err := s.groupRepo.RemoveMember(ctx, req.Gid, req.TargetUid); err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已踢出"})
}

func (s *extraGroupImpl) MuteMember(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Gid      string `json:"gid"`
		TargetUid string `json:"target_uid"`
		Duration int    `json:"duration"` // 秒，-1=永久
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Gid == "" || req.TargetUid == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	op, _ := s.groupRepo.FindMember(ctx, req.Gid, uid)
	if op == nil || (op.Role != entity.RoleOwner && op.Role != entity.RoleAdmin) {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	banID := s.idGen.Generate()
	if err := s.groupRepo.AddMuteRecord(ctx, &entity.MuteRecord{
		BanID: banID, GID: req.Gid, UID: req.TargetUid, MuteDuration: req.Duration,
	}); err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已禁言", "ban_id": banID})
}

func (s *extraGroupImpl) UnmuteMember(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Gid   string `json:"gid"`
		BanId string `json:"ban_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Gid == "" || req.BanId == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	op, _ := s.groupRepo.FindMember(ctx, req.Gid, uid)
	if op == nil || (op.Role != entity.RoleOwner && op.Role != entity.RoleAdmin) {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	if err := s.groupRepo.RemoveMute(ctx, req.BanId); err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已解除禁言"})
}

func (s *extraGroupImpl) UpdateMemberRole(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Gid       string `json:"gid"`
		TargetUid string `json:"target_uid"`
		Role      string `json:"role"` // admin / member
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Gid == "" || req.TargetUid == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	// 只有群主能改角色
	op, _ := s.groupRepo.FindMember(ctx, req.Gid, uid)
	if op == nil || op.Role != entity.RoleOwner {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "只有群主能修改角色"})
		return
	}
	role := entity.Role(req.Role)
	if role != entity.RoleAdmin && role != entity.RoleMember {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "无效角色"})
		return
	}
	if err := s.groupRepo.UpdateMemberRole(ctx, role, req.Gid, req.TargetUid); err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "角色已修改"})
}

func (s *extraGroupImpl) GetGroupRequests(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var body struct {
		Gid string `json:"gid"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	gid := body.Gid
	if gid == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "缺少 gid"})
		return
	}
	// 检查是否是管理员
	op, _ := s.groupRepo.FindMember(ctx, gid, uid)
	if op == nil || (op.Role != entity.RoleOwner && op.Role != entity.RoleAdmin) {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权查看"})
		return
	}
	reqs, err := s.groupRepo.FindPendingRequestsByGroup(ctx, gid)
	if err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	type reqInfo struct {
		ReqId        string `json:"req_id"`
		Gid         string `json:"gid"`
		ApplicantUid string `json:"applicant_uid"`
		ApplyText   string `json:"apply_text"`
		CreateTime  int64  `json:"create_time"`
	}
	list := make([]reqInfo, 0, len(reqs))
	for _, r := range reqs {
		ct := int64(0)
		if r.CreateTime != nil {
			ct = *r.CreateTime
		}
		at := ""
		if r.ApplyText != nil {
			at = *r.ApplyText
		}
		list = append(list, reqInfo{
			ReqId: r.ReqID, Gid: r.GID, ApplicantUid: r.ApplicantUID,
			ApplyText: at, CreateTime: ct,
		})
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "requests": list})
}

func (s *extraGroupImpl) ApproveGroupRequest(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		ReqId  string `json:"req_id"`
		Approve bool   `json:"approve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ReqId == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	gr, err := s.groupRepo.FindGroupRequestByID(ctx, req.ReqId)
	if err != nil || gr == nil {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "申请不存在"})
		return
	}
	// 检查是否是管理员
	op, _ := s.groupRepo.FindMember(ctx, gr.GID, uid)
	if op == nil || (op.Role != entity.RoleOwner && op.Role != entity.RoleAdmin) {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	if req.Approve {
		// 接受：加入群 + 更新申请状态
		if err := s.groupRepo.SaveMember(ctx, &entity.GroupMember{
			UID: gr.ApplicantUID, GID: gr.GID, Role: entity.RoleMember,
		}); err != nil {
			writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": "添加成员失败: " + err.Error()})
			return
		}
		if err := s.groupRepo.UpdateGroupRequestStatus(ctx, req.ReqId, entity.ReqStatusAccepted, uid, 0); err != nil {
			writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	} else {
		if err := s.groupRepo.UpdateGroupRequestStatus(ctx, req.ReqId, entity.ReqStatusRejected, uid, 0); err != nil {
			writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
			return
		}
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已处理"})
}

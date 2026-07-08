package restful

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/domain/entity"
	"echat/sdk/repository/mysql"
	"echat/service/api/internal/shared"
)

// ExtraMiscImpl 杂项额外 RESTful API。
type ExtraMiscImpl struct {
	UserRepo     *mysql.UserRepo
	FileRepo     *mysql.FileRepo
	GroupMsgRepo *mysql.GroupMessageRepo
}

// NewExtraMiscImpl 创建 ExtraMiscImpl。
func NewExtraMiscImpl(userRepo *mysql.UserRepo, fileRepo *mysql.FileRepo, groupMsgRepo *mysql.GroupMessageRepo) *ExtraMiscImpl {
	return &ExtraMiscImpl{UserRepo: userRepo, FileRepo: fileRepo, GroupMsgRepo: groupMsgRepo}
}

// BatchGetUsers 批量查用户信息。
func (s *ExtraMiscImpl) BatchGetUsers(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		Uids []string `json:"uids"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if len(req.Uids) == 0 {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	users, err := s.UserRepo.FindUsersByUIDs(ctx, req.Uids)
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

// RevokeFilePermission 撤销文件权限。
func (s *ExtraMiscImpl) RevokeFilePermission(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		FileId     string `json:"file_id"`
		AccessType string `json:"access_type"`
		TargetId   string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileId == "" {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	meta, _ := s.FileRepo.FindFileMetadataByID(ctx, req.FileId)
	if meta == nil || meta.OwnerUID != uid {
		shared.WriteJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "无权操作"})
		return
	}
	n, err := s.FileRepo.RevokeFilePermission(ctx, req.FileId, entity.AccessType(req.AccessType), req.TargetId)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "affected": n})
}

// GetFileAssociations 获取文件关联列表。
func (s *ExtraMiscImpl) GetFileAssociations(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		FileId          string `json:"file_id"`
		AssociationType string `json:"association_type"`
		AssociatedId    string `json:"associated_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var assocs []*entity.FileAssociation
	var err error
	if req.FileId != "" {
		assocs, err = s.FileRepo.FindFileAssociations(ctx, req.FileId)
	} else if req.AssociationType != "" && req.AssociatedId != "" {
		assocs, err = s.FileRepo.FindFilesByAssociation(ctx, entity.AssociationType(req.AssociationType), req.AssociatedId)
	} else {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	if err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	type assocInfo struct {
		AssociationId   string `json:"association_id"`
		FileId          string `json:"file_id"`
		AssociationType string `json:"association_type"`
		AssociatedId    string `json:"associated_id"`
		CreatorUid      string `json:"creator_uid"`
		CreateTime      int64  `json:"create_time"`
	}
	list := make([]assocInfo, 0, len(assocs))
	for _, a := range assocs {
		ct := int64(0)
		if a.CreateTime != nil {
			ct = *a.CreateTime
		}
		list = append(list, assocInfo{
			AssociationId: a.AssociationID, FileId: a.FileID,
			AssociationType: string(a.AssociationType), AssociatedId: a.AssociatedID,
			CreatorUid: a.CreatorUID, CreateTime: ct,
		})
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "associations": list})
}

// CreateFileAssociation 创建文件关联。
func (s *ExtraMiscImpl) CreateFileAssociation(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		FileId          string `json:"file_id"`
		AssociationType string `json:"association_type"`
		AssociatedId    string `json:"associated_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileId == "" || req.AssociatedId == "" {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	assocID := req.FileId[:8] + "_" + req.AssociatedId
	if len(assocID) > 20 {
		assocID = assocID[:20]
	}
	if err := s.FileRepo.CreateFileAssociation(ctx, &entity.FileAssociation{
		AssociationID:   assocID,
		FileID:          req.FileId,
		AssociationType: entity.AssociationType(req.AssociationType),
		AssociatedID:    req.AssociatedId,
		CreatorUID:      uid,
	}); err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "已关联", "association_id": assocID})
}

// GetMessageReadCounts 查询消息已读人数。
func (s *ExtraMiscImpl) GetMessageReadCounts(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	if shared.GetUID(ctx) == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		MsgIds []string `json:"msg_ids"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if len(req.MsgIds) == 0 {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	counts, err := s.GroupMsgRepo.GetMessageReadCounts(ctx, req.MsgIds)
	if err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "read_counts": counts})
	_ = r
}

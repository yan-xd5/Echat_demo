package restful

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/domain/entity"
	"echat/sdk/infrastructure/idgen"
	"echat/sdk/repository/mysql"
	"echat/service/api/internal/shared"
)

// ExtraFileImpl 文件相关的额外 RESTful API。
type ExtraFileImpl struct {
	FileRepo *mysql.FileRepo
	IDGen    *idgen.Snowflake
}

// NewExtraFileImpl 创建 ExtraFileImpl。
func NewExtraFileImpl(fileRepo *mysql.FileRepo, idGen *idgen.Snowflake) *ExtraFileImpl {
	return &ExtraFileImpl{FileRepo: fileRepo, IDGen: idGen}
}

// SetFilePermission 设置文件权限。
func (s *ExtraFileImpl) SetFilePermission(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := shared.GetUID(ctx)
	if uid == "" {
		shared.WriteJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		FileId          string `json:"file_id"`
		AccessType      string `json:"access_type"`      // user / friend / group / public
		TargetId        string `json:"target_id"`        // 用户ID / 群ID
		PermissionLevel string `json:"permission_level"` // view / download / share / manage
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileId == "" {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	// 检查文件所有权
	meta, err := s.FileRepo.FindFileMetadataByID(ctx, req.FileId)
	if err != nil || meta == nil {
		shared.WriteJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "文件不存在"})
		return
	}
	if meta.OwnerUID != uid {
		shared.WriteJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "只有文件所有者能设置权限"})
		return
	}
	permID := s.IDGen.Generate()
	perm := &entity.FilePermission{
		PermissionID:    permID,
		FileID:          req.FileId,
		AccessType:      entity.AccessType(req.AccessType),
		PermissionLevel: entity.PermissionLevel(req.PermissionLevel),
		GrantedBy:       uid,
	}
	if req.TargetId != "" {
		perm.TargetID = &req.TargetId
	}
	if err := s.FileRepo.GrantFilePermission(ctx, perm); err != nil {
		shared.WriteJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	shared.WriteJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "权限已设置", "permission_id": permID})
}

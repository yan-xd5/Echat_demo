package main

import (
	"context"
	"encoding/json"

	thttp "trpc.group/trpc-go/trpc-go/http"

	"echat/sdk/entity"
	"echat/sdk/idgen"
	"echat/sdk/mysql"
)

type extraFileImpl struct {
	fileRepo *mysql.FileRepo
	idGen    *idgen.Snowflake
}

func (s *extraFileImpl) SetFilePermission(ctx context.Context) {
	w, r := thttp.Response(ctx), thttp.Request(ctx)
	uid := getUID(ctx)
	if uid == "" {
		writeJSON(ctx, w, 401, map[string]interface{}{"code": 1, "message": "未登录"})
		return
	}
	var req struct {
		FileId          string `json:"file_id"`
		AccessType      string `json:"access_type"`      // user / friend / group / public
		TargetId        string `json:"target_id"`        // 用户ID / 群ID
		PermissionLevel string `json:"permission_level"` // view / download / share / manage
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileId == "" {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "参数错误"})
		return
	}
	// 检查文件所有权
	meta, err := s.fileRepo.FindFileMetadataByID(ctx, req.FileId)
	if err != nil || meta == nil {
		writeJSON(ctx, w, 400, map[string]interface{}{"code": 1, "message": "文件不存在"})
		return
	}
	if meta.OwnerUID != uid {
		writeJSON(ctx, w, 403, map[string]interface{}{"code": 1, "message": "只有文件所有者能设置权限"})
		return
	}
	permID := s.idGen.Generate()
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
	if err := s.fileRepo.GrantFilePermission(ctx, perm); err != nil {
		writeJSON(ctx, w, 500, map[string]interface{}{"code": 999, "message": err.Error()})
		return
	}
	writeJSON(ctx, w, 200, map[string]interface{}{"code": 0, "message": "权限已设置", "permission_id": permID})
}

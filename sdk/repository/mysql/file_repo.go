package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"echat/sdk/domain/entity"
)

type FileRepo struct {
	DB *sqlx.DB
}

func NewFileRepo(db *sqlx.DB) *FileRepo { return &FileRepo{DB: db} }

// ======================== 文件存储 ========================

func (r *FileRepo) CreateOrGetFileStorage(ctx context.Context, fileHash, filePath string, thumbnailPath *string, fileSize int64, mimeType string) (*entity.FileStorage, error) {
	existing, hashErr := r.FindFileStorageByHash(ctx, fileHash)
	if hashErr != nil {
		return nil, fmt.Errorf("find storage: %w", hashErr)
	}
	if existing != nil {
		r.IncrementRefCount(ctx, existing.StorageID)
		return existing, nil
	}

	// 用 fileHash 前16位作为 storage_id，保证唯一
	id := fileHash[:16]
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO file_storage (storage_id, file_hash, file_path, thumbnail_path, file_size, mime_type)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, fileHash, filePath, thumbnailPath, fileSize, mimeType)
	if err != nil {
		return nil, fmt.Errorf("create file_storage: %w", err)
	}
	return &entity.FileStorage{
		StorageID: id, FileHash: fileHash, FilePath: filePath,
		ThumbnailPath: thumbnailPath, FileSize: fileSize, MimeType: mimeType, ReferenceCount: 1,
	}, nil
}

func (r *FileRepo) FindFileStorageByID(ctx context.Context, storageID string) (*entity.FileStorage, error) {
	var fs entity.FileStorage
	err := r.DB.GetContext(ctx, &fs,
		`SELECT storage_id, file_hash, file_path, thumbnail_path, file_size, mime_type,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time, reference_count
		 FROM file_storage WHERE storage_id = ?`, storageID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &fs, err
}

func (r *FileRepo) FindFileStoragesByIDs(ctx context.Context, storageIDs []string) (map[string]*entity.FileStorage, error) {
	if len(storageIDs) == 0 {
		return nil, nil
	}
	query, args, _ := sqlx.In(
		`SELECT storage_id, file_hash, file_path, thumbnail_path, file_size, mime_type, reference_count
		 FROM file_storage WHERE storage_id IN (?)`, storageIDs)
	var files []entity.FileStorage
	if err := r.DB.SelectContext(ctx, &files, r.DB.Rebind(query), args...); err != nil {
		return nil, err
	}
	m := make(map[string]*entity.FileStorage, len(files))
	for i := range files {
		m[files[i].StorageID] = &files[i]
	}
	return m, nil
}

func (r *FileRepo) FindFileStorageByHash(ctx context.Context, fileHash string) (*entity.FileStorage, error) {
	var fs entity.FileStorage
	err := r.DB.GetContext(ctx, &fs,
		`SELECT storage_id, file_hash, file_path, thumbnail_path, file_size, mime_type,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time, reference_count
		 FROM file_storage WHERE file_hash = ?`, fileHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &fs, err
}

func (r *FileRepo) IncrementRefCount(ctx context.Context, storageID string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE file_storage SET reference_count = reference_count + 1 WHERE storage_id = ?`, storageID)
	return err
}

func (r *FileRepo) FindUnusedFiles(ctx context.Context) ([]*entity.FileStorage, error) {
	var files []entity.FileStorage
	err := r.DB.SelectContext(ctx, &files,
		`SELECT storage_id, file_hash, file_path, file_size, reference_count
		 FROM file_storage WHERE reference_count = 0`)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.FileStorage, len(files))
	for i := range files {
		ptr[i] = &files[i]
	}
	return ptr, nil
}

func (r *FileRepo) DeleteFileStorage(ctx context.Context, storageID string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM file_storage WHERE storage_id = ?`, storageID)
	return err
}

// ======================== 文件元数据 ========================

func (r *FileRepo) SaveFileMetadata(ctx context.Context, meta *entity.FileMetadata) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO file_metadata (file_id, storage_id, owner_uid, original_name, display_name, file_type)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		meta.FileID, meta.StorageID, meta.OwnerUID, meta.OriginalName, meta.DisplayName, meta.FileType)
	return err
}

func (r *FileRepo) FindFileMetadataByID(ctx context.Context, fileID string) (*entity.FileMetadata, error) {
	var m entity.FileMetadata
	err := r.DB.GetContext(ctx, &m,
		`SELECT file_id, storage_id, owner_uid, original_name, display_name, file_type,
		 UNIX_TIMESTAMP(upload_time)*1000 AS upload_time, download_count, file_status
		 FROM file_metadata WHERE file_id = ?`, fileID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *FileRepo) FindFilesByOwner(ctx context.Context, ownerUID string, limit, offset int) ([]*entity.FileMetadata, error) {
	var files []entity.FileMetadata
	err := r.DB.SelectContext(ctx, &files,
		`SELECT file_id, storage_id, owner_uid, original_name, display_name, file_type,
		 UNIX_TIMESTAMP(upload_time)*1000 AS upload_time, download_count, file_status
		 FROM file_metadata WHERE owner_uid = ? AND file_status = 'active'
		 ORDER BY upload_time DESC LIMIT ? OFFSET ?`, ownerUID, limit, offset)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.FileMetadata, len(files))
	for i := range files {
		ptr[i] = &files[i]
	}
	return ptr, nil
}

func (r *FileRepo) IncrementDownloadCount(ctx context.Context, fileID string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE file_metadata SET download_count = download_count + 1 WHERE file_id = ?`, fileID)
	return err
}

func (r *FileRepo) SoftDeleteFile(ctx context.Context, fileID string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE file_metadata SET file_status = 'deleted' WHERE file_id = ?`, fileID)
	return err
}

func (r *FileRepo) UpdateLastAccessTime(ctx context.Context, fileID string) error {
	_, err := r.DB.ExecContext(ctx,
		`UPDATE file_metadata SET last_access_time = NOW() WHERE file_id = ?`, fileID)
	return err
}

// ======================== 权限管理 ========================

func (r *FileRepo) GrantFilePermission(ctx context.Context, perm *entity.FilePermission) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO file_permission (permission_id, file_id, access_type, target_id, permission_level, granted_by)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		perm.PermissionID, perm.FileID, string(perm.AccessType), perm.TargetID,
		string(perm.PermissionLevel), perm.GrantedBy)
	return err
}

func (r *FileRepo) VerifyFilePermission(ctx context.Context, fileID, userUID string, requiredLevel entity.PermissionLevel) (bool, error) {
	// 所有者有所有权限
	var ownerUID string
	err := r.DB.GetContext(ctx, &ownerUID,
		`SELECT owner_uid FROM file_metadata WHERE file_id = ?`, fileID)
	if err == nil && ownerUID == userUID {
		return true, nil
	}

	// 检查授权（权限层级：view < download < share < manage，高权限包含低权限）
	levels := permissionLevelsAbove(requiredLevel)
	query, args, _ := sqlx.In(
		`SELECT COUNT(*) FROM file_permission
		 WHERE file_id = ? AND target_id = ? AND permission_level IN (?)`, fileID, userUID, levels)
	var count int
	err = r.DB.GetContext(ctx, &count, r.DB.Rebind(query), args...)
	return count > 0, err
}

func (r *FileRepo) RevokeFilePermission(ctx context.Context, fileID string, accessType entity.AccessType, targetID string) (int64, error) {
	result, err := r.DB.ExecContext(ctx,
		`DELETE FROM file_permission WHERE file_id = ? AND access_type = ? AND target_id = ?`,
		fileID, string(accessType), targetID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ======================== 文件关联 ========================

func (r *FileRepo) CreateFileAssociation(ctx context.Context, assoc *entity.FileAssociation) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO file_association (association_id, file_id, association_type, associated_id, creator_uid)
		 VALUES (?, ?, ?, ?, ?)`,
		assoc.AssociationID, assoc.FileID, string(assoc.AssociationType), assoc.AssociatedID, assoc.CreatorUID)
	return err
}

func (r *FileRepo) FindFilesByAssociation(ctx context.Context, associationType entity.AssociationType, associatedID string) ([]*entity.FileAssociation, error) {
	var assocs []entity.FileAssociation
	err := r.DB.SelectContext(ctx, &assocs,
		`SELECT association_id, file_id, association_type, associated_id, creator_uid,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM file_association WHERE association_type = ? AND associated_id = ?`,
		string(associationType), associatedID)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.FileAssociation, len(assocs))
	for i := range assocs {
		ptr[i] = &assocs[i]
	}
	return ptr, nil
}

func (r *FileRepo) FindFileAssociations(ctx context.Context, fileID string) ([]*entity.FileAssociation, error) {
	var assocs []entity.FileAssociation
	err := r.DB.SelectContext(ctx, &assocs,
		`SELECT association_id, file_id, association_type, associated_id, creator_uid,
		 UNIX_TIMESTAMP(create_time)*1000 AS create_time
		 FROM file_association WHERE file_id = ?`, fileID)
	if err != nil {
		return nil, err
	}
	ptr := make([]*entity.FileAssociation, len(assocs))
	for i := range assocs {
		ptr[i] = &assocs[i]
	}
	return ptr, nil
}

// FindFileAssociationByID 根据关联 ID 查找文件关联。
func (r *FileRepo) FindFileAssociationByID(ctx context.Context, associationID string) (*entity.FileAssociation, error) {
	var a entity.FileAssociation
	err := r.DB.GetContext(ctx, &a, `SELECT * FROM file_association WHERE association_id = ?`, associationID)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *FileRepo) DeleteFileAssociation(ctx context.Context, associationID string) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM file_association WHERE association_id = ?`, associationID)
	return err
}

func (r *FileRepo) BatchDeleteAssociations(ctx context.Context, associationType entity.AssociationType, associatedID string) (int64, error) {
	result, err := r.DB.ExecContext(ctx,
		`DELETE FROM file_association WHERE association_type = ? AND associated_id = ?`,
		string(associationType), associatedID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// permissionLevelsAbove 返回 requiredLevel 及以上所有权限级别。
func permissionLevelsAbove(level entity.PermissionLevel) []string {
	order := []entity.PermissionLevel{entity.PermView, entity.PermDownload, entity.PermShare, entity.PermManage}
	for i, l := range order {
		if l == level {
			result := make([]string, len(order)-i)
			for j := range result {
				result[j] = string(order[i+j])
			}
			return result
		}
	}
	return []string{string(level)}
}

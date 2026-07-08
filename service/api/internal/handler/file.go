package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"trpc.group/trpc-go/trpc-go/log"

	"echat/sdk/domain/entity"
	"echat/sdk/infrastructure/idgen"
	"echat/sdk/repository/mysql"
	pb "echat/service/api/stub"
	"echat/service/api/internal/shared"
)

var uploadDir = "uploads"

func init() { os.MkdirAll(uploadDir, 0755) }

type FileImpl struct {
	FileRepo *mysql.FileRepo
	IDGen    *idgen.Snowflake
}

// Upload 上传文件（SHA256 去重 + 存本地磁盘）。
func (s *FileImpl) Upload(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(req.FileData))
	fileID := s.IDGen.Generate()
	ext := filepath.Ext(req.FileName)
	diskPath := filepath.Join(uploadDir, fileID+ext)

	// 写入磁盘
	if err := os.WriteFile(diskPath, req.FileData, 0644); err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	storage, err := s.FileRepo.CreateOrGetFileStorage(ctx, hash, diskPath, nil, int64(len(req.FileData)), req.MimeType)
	if err != nil {
		os.Remove(diskPath)
		return nil, fmt.Errorf("创建存储记录失败: %w", err)
	}

	if err := s.FileRepo.SaveFileMetadata(ctx, &entity.FileMetadata{
		FileID: fileID, StorageID: storage.StorageID, OwnerUID: uid,
		OriginalName: req.FileName, DisplayName: req.FileName, FileType: req.FileType,
	}); err != nil {
		os.Remove(diskPath)
		s.FileRepo.DeleteFileStorage(ctx, storage.StorageID)
		return nil, fmt.Errorf("保存元数据失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 文件上传: file_id=%s, hash=%s, size=%d", fileID, hash[:16], len(req.FileData))
	return &pb.UploadResponse{FileId: fileID, FileUrl: "/api/v1/file/download?file_id=" + fileID}, nil
}

// Download 下载文件（权限校验 + 返回文件内容）。
func (s *FileImpl) Download(ctx context.Context, req *pb.DownloadRequest) (*pb.DownloadResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	meta, err := s.FileRepo.FindFileMetadataByID(ctx, req.FileId)
	if err != nil || meta == nil {
		return nil, fmt.Errorf("文件不存在")
	}

	storage, err := s.FileRepo.FindFileStorageByID(ctx, meta.StorageID)
	if err != nil || storage == nil {
		return nil, fmt.Errorf("文件存储不存在")
	}

	perm, err := s.FileRepo.VerifyFilePermission(ctx, req.FileId, uid, entity.PermDownload)
	if err != nil {
		return nil, fmt.Errorf("权限校验失败: %w", err)
	}
	if !perm && meta.OwnerUID != uid {
		return nil, fmt.Errorf("无权下载")
	}

	data, err := os.ReadFile(storage.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	s.FileRepo.IncrementDownloadCount(ctx, req.FileId)
	s.FileRepo.UpdateLastAccessTime(ctx, req.FileId)

	return &pb.DownloadResponse{
		FileName: meta.OriginalName, MimeType: storage.MimeType, FileData: data,
	}, nil
}

// Preview 预览文件（权限校验 + 缩略图）。
func (s *FileImpl) Preview(ctx context.Context, req *pb.PreviewRequest) (*pb.PreviewResponse, error) {
	resp, err := s.Download(ctx, &pb.DownloadRequest{FileId: req.FileId})
	if err != nil {
		return nil, err
	}
	return &pb.PreviewResponse{
		FileName: resp.FileName, MimeType: resp.MimeType, FileData: resp.FileData,
	}, nil
}

// Delete 软删除文件。
func (s *FileImpl) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	meta, err := s.FileRepo.FindFileMetadataByID(ctx, req.FileId)
	if err != nil || meta == nil {
		return nil, fmt.Errorf("文件不存在")
	}
	if meta.OwnerUID != uid {
		return nil, fmt.Errorf("无权删除")
	}

	if err := s.FileRepo.SoftDeleteFile(ctx, req.FileId); err != nil {
		return nil, fmt.Errorf("删除失败: %w", err)
	}

	log.InfoContextf(ctx, "[API] 文件删除: file_id=%s", req.FileId)
	return &pb.DeleteResponse{}, nil
}

// ListFiles 列出用户文件。
func (s *FileImpl) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	uid := shared.GetUID(ctx)
	if uid == "" {
		return nil, fmt.Errorf("未登录")
	}

	limit, offset := int(req.Limit), int(req.Offset)
	if limit <= 0 {
		limit = 20
	}

	files, err := s.FileRepo.FindFilesByOwner(ctx, uid, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查文件列表失败: %w", err)
	}

	// 批量查询文件存储信息（避免 N+1）
	sids := make([]string, 0, len(files))
	for _, f := range files {
		sids = append(sids, f.StorageID)
	}
	storageMap, _ := s.FileRepo.FindFileStoragesByIDs(ctx, sids)

	var list []*pb.FileInfo
	for _, f := range files {
		size := int64(0)
		mime := ""
		if s, ok := storageMap[f.StorageID]; ok {
			size = s.FileSize
			mime = s.MimeType
		}
		ut := int64(0)
		if f.UploadTime != nil {
			ut = *f.UploadTime
		}
		list = append(list, &pb.FileInfo{
			FileId: f.FileID, OriginalName: f.OriginalName, FileType: f.FileType,
			FileSize: size, MimeType: mime, UploadTime: ut, DownloadCount: f.DownloadCount,
		})
	}

	return &pb.ListFilesResponse{Files: list, Total: int32(len(list))}, nil
}
func NewFileImpl(FileRepo *mysql.FileRepo, IDGen *idgen.Snowflake) *FileImpl {
	return &FileImpl{FileRepo: FileRepo, IDGen: IDGen}
}

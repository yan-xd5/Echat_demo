// Package repo 定义各业务模块的数据访问接口（Repository 层）。
// 接口定义与 New_echat/Database/SQL.md 的表结构一致，
// 入参/出参使用 entity 包中定义的实体类型。
package repo

import (
	"context"

	"echat/sdk/entity"
)

// ============================================================
// UserAuthRepository — 用户认证
// ============================================================

// UserAuthRepository 认证相关的用户数据访问接口。
type UserAuthRepository interface {
	FindUserByAccount(ctx context.Context, account string) (*entity.User, error)
	InsertUser(ctx context.Context, user *entity.User) error
	SaveUser(ctx context.Context, user *entity.User) error
	ExistsByAccount(ctx context.Context, account string) (bool, error)
	DeleteUser(ctx context.Context, uid string) error
}

// ============================================================
// UserProfileRepository — 用户资料 + 在线状态
// ============================================================

// UserProfileRepository 用户资料与在线状态数据访问接口。
type UserProfileRepository interface {
	// MySQL
	FindUserByUID(ctx context.Context, uid string) (*entity.User, error)
	FindUsersByUIDs(ctx context.Context, uids []string) (map[string]*entity.User, error)
	FindUserByUsername(ctx context.Context, username string) ([]*entity.User, error)
	FindUserByRegion(ctx context.Context, region string) ([]*entity.User, error)
	FindUserByCreateTimeRange(ctx context.Context, start, end int64) ([]*entity.User, error)
	UpdateUser(ctx context.Context, user *entity.User) error

	// Redis
	UserOnline(ctx context.Context, info *entity.UserOnline, groupIDs []string) error
	UserOffline(ctx context.Context, account string, groupIDs []string) error
	UpdateHeartbeat(ctx context.Context, groupIDs []string) error
	BatchCheckOnlineStatus(ctx context.Context, accounts []string) ([]string, error)
	GetGroupOnlineMembers(ctx context.Context, gid string) ([]string, error)
}

// ============================================================
// FriendshipRepository — 好友
// ============================================================

// FriendshipRepository 好友数据访问接口。
type FriendshipRepository interface {
	// 好友关系
	SaveFriendship(ctx context.Context, f *entity.Friends) error
	FindFriendshipByFID(ctx context.Context, fid string) (*entity.Friends, error)
	FindFriendshipByUsers(ctx context.Context, uid1, uid2 string) (*entity.Friends, error)
	FindFriendshipByUID(ctx context.Context, uid string) ([]*entity.Friends, error)
	DeleteFriendship(ctx context.Context, fid string) error
	DeleteFriendshipWithChat(ctx context.Context, fid string) error

	// 黑名单
	SaveBlacklist(ctx context.Context, fid, uid string, isBlacklist bool) error
	FindBlacklistedFriends(ctx context.Context, uid string) ([]*entity.Friends, error)

	// 好友申请
	SaveFriendRequest(ctx context.Context, req *entity.FriendRequest) error
	FindFriendRequestByID(ctx context.Context, reqID string) (*entity.FriendRequest, error)
	FindFriendRequestByReceiver(ctx context.Context, receiverUID string) ([]*entity.FriendRequest, error)
	FindFriendRequestBySender(ctx context.Context, senderUID string) ([]*entity.FriendRequest, error)
	UpdateRequestStatus(ctx context.Context, reqID string, status entity.ReqStatus, handleTime int64) error
	AcceptFriendRequestWithChat(ctx context.Context, reqID string, handleTime int64, friendship *entity.Friends, chat *entity.PrivateChat) error

	// 权限校验
	ValidatePrivateMessagePermission(ctx context.Context, senderUID, receiverUID string) error
}

// ============================================================
// PrivateChatRepository — 私聊
// ============================================================

// PrivateChatRepository 私聊数据访问接口。
type PrivateChatRepository interface {
	// 会话管理
	SaveChat(ctx context.Context, chat *entity.PrivateChat) error
	FindChatByPID(ctx context.Context, pid string) (*entity.PrivateChat, error)
	FindChatByUsers(ctx context.Context, uid1, uid2 string) (*entity.PrivateChat, error)
	FindChatsByUser(ctx context.Context, uid string) ([]*entity.PrivateChat, error)
	UpdatePinStatus(ctx context.Context, pid, uid string, isPinned bool) error

	// 消息管理
	SaveMessage(ctx context.Context, msg *entity.PrivateMessage) error
	FindMessageByID(ctx context.Context, msgID string) (*entity.PrivateMessage, error)
	FindMessagesByChat(ctx context.Context, pid string) ([]*entity.PrivateMessage, error)
	MarkMessageAsRead(ctx context.Context, msgID string) error
	MarkMessagesAsReadByChatAndTime(ctx context.Context, pid, uid string, timestamp int64) (int64, error)
	MarkMessageAsRevoked(ctx context.Context, msgID string) error
	FindUnreadMessagesByChat(ctx context.Context, pid, uid string) ([]*entity.PrivateMessage, error)
	GetUnreadMessageCountByChat(ctx context.Context, pid, uid string) (int, error)
	FindLatestMessageByChat(ctx context.Context, pid string) (*entity.PrivateMessage, error)
	GetMessageCountByChat(ctx context.Context, pid string) (int64, error)
}

// ============================================================
// GroupChatRepository — 群聊
// ============================================================

// GroupChatRepository 群聊数据访问接口。
type GroupChatRepository interface {
	// 群聊基础
	SaveGroup(ctx context.Context, group *entity.GroupChat) error
	FindGroupByGID(ctx context.Context, gid string) (*entity.GroupChat, error)
	FindGroupsByOwner(ctx context.Context, ownerUID string) ([]*entity.GroupChat, error)
	FindGroupByName(ctx context.Context, name string) ([]*entity.GroupChat, error)
	DeleteGroup(ctx context.Context, gid string) error

	// 成员管理
	SaveMember(ctx context.Context, member *entity.GroupMember) error
	FindMember(ctx context.Context, gid, uid string) (*entity.GroupMember, error)
	FindMembersByGroup(ctx context.Context, gid string) ([]*entity.GroupMember, error)
	FindGroupsByUser(ctx context.Context, uid string) ([]*entity.GroupMember, error)
	UpdateMemberRole(ctx context.Context, role entity.Role, gid, uid string) error
	RemoveMember(ctx context.Context, gid, uid string) error

	// 禁言管理
	AddMuteRecord(ctx context.Context, mute *entity.MuteRecord) error
	FindMuteRecordsByGroup(ctx context.Context, gid string) ([]*entity.MuteRecord, error)
	FindMuteRecordByUser(ctx context.Context, gid, uid string) (*entity.MuteRecord, error)
	RemoveMute(ctx context.Context, banID string) error
	FindExpiredMuteRecords(ctx context.Context) ([]*entity.MuteRecord, error)

	// 群申请
	SaveGroupRequest(ctx context.Context, req *entity.GroupJoinRequest) error
	FindGroupRequestByID(ctx context.Context, reqID string) (*entity.GroupJoinRequest, error)
	FindPendingRequestsByGroup(ctx context.Context, gid string) ([]*entity.GroupJoinRequest, error)
	FindAllRequestsByGroup(ctx context.Context, gid string) ([]*entity.GroupJoinRequest, error)
	FindRequestsByUser(ctx context.Context, uid string) ([]*entity.GroupJoinRequest, error)
	UpdateGroupRequestStatus(ctx context.Context, reqID string, status entity.ReqStatus, approverUID string, handleTime int64) error

	// 权限校验
	ValidateGroupMessagePermission(ctx context.Context, senderUID, gid string) error
}

// ============================================================
// GroupMessageRepository — 群消息
// ============================================================

// GroupMessageRepository 群消息数据访问接口。
type GroupMessageRepository interface {
	// 消息管理
	SaveMessage(ctx context.Context, msg *entity.GroupMessage) error
	FindMessageByID(ctx context.Context, msgID string) (*entity.GroupMessage, error)
	FindMessagesByGroup(ctx context.Context, gid string) ([]*entity.GroupMessage, error)
	FindMessagesByGroupWithPagination(ctx context.Context, gid string, limit, offset int64) ([]*entity.GroupMessage, error)
	FindMessagesByGroupAndTimeRange(ctx context.Context, gid string, start, end int64) ([]*entity.GroupMessage, error)
	GetMessageCountByGroup(ctx context.Context, gid string) (int64, error)
	MarkMessageAsRevoked(ctx context.Context, msgID string) error
	FindAnnouncesByGroup(ctx context.Context, gid string) ([]*entity.GroupMessage, error)

	// 已读状态
	MarkMessageAsRead(ctx context.Context, msgID, gid, uid string) error
	FindReadUsersByMessage(ctx context.Context, msgID string) ([]string, error)
	FindUnreadMessagesByUser(ctx context.Context, gid, uid string) ([]*entity.GroupMessage, error)
	GetUnreadMessageCountByGroup(ctx context.Context, gid, uid string) (int, error)
	GetMessageReadCount(ctx context.Context, msgID string) (int64, error)
	GetMessageReadCounts(ctx context.Context, msgIDs []string) (map[string]int64, error)
	FindLatestMessageByGroup(ctx context.Context, gid string) (*entity.GroupMessage, error)
	MarkMessagesAsReadByGroupAndTime(ctx context.Context, gid, uid string, timestamp int64) (int64, error)
}

// ============================================================
// FileRepository — 文件
// ============================================================

// FileRepository 文件数据访问接口。
type FileRepository interface {
	// 文件存储
	CreateOrGetFileStorage(ctx context.Context, fileHash, filePath string, thumbnailPath *string, fileSize int64, mimeType string) (*entity.FileStorage, error)
	FindFileStorageByID(ctx context.Context, storageID string) (*entity.FileStorage, error)
	FindFileStorageByHash(ctx context.Context, fileHash string) (*entity.FileStorage, error)
	IncrementRefCount(ctx context.Context, storageID string) error
	FindUnusedFiles(ctx context.Context) ([]*entity.FileStorage, error)
	DeleteFileStorage(ctx context.Context, storageID string) error

	// 文件元数据
	SaveFileMetadata(ctx context.Context, meta *entity.FileMetadata) error
	FindFileMetadataByID(ctx context.Context, fileID string) (*entity.FileMetadata, error)
	FindFilesByOwner(ctx context.Context, ownerUID string, limit, offset int) ([]*entity.FileMetadata, error)
	IncrementDownloadCount(ctx context.Context, fileID string) error
	SoftDeleteFile(ctx context.Context, fileID string) error
	UpdateLastAccessTime(ctx context.Context, fileID string) error

	// 权限管理
	GrantFilePermission(ctx context.Context, perm *entity.FilePermission) error
	VerifyFilePermission(ctx context.Context, fileID, userUID string, requiredLevel entity.PermissionLevel) (bool, error)
	RevokeFilePermission(ctx context.Context, fileID string, accessType entity.AccessType, targetID string) (int64, error)

	// 文件关联
	CreateFileAssociation(ctx context.Context, assoc *entity.FileAssociation) error
	FindFilesByAssociation(ctx context.Context, associationType entity.AssociationType, associatedID string) ([]*entity.FileAssociation, error)
	FindFileAssociations(ctx context.Context, fileID string) ([]*entity.FileAssociation, error)
	DeleteFileAssociation(ctx context.Context, associationID string) error
	BatchDeleteAssociations(ctx context.Context, associationType entity.AssociationType, associatedID string) (int64, error)
}

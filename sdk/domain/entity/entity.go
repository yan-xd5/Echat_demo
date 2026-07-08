// Package entity 定义 echat 系统全部数据实体与枚举类型。
// 实体字段与 New_echat/Database/SQL.md 表结构一一对应，
// NULL 列使用指针类型（nil = NULL）。
package entity

// ============================================================
// 枚举
// ============================================================

// ReqStatus 申请状态
type ReqStatus string

const (
	ReqStatusPending  ReqStatus = "pending"
	ReqStatusAccepted ReqStatus = "accepted"
	ReqStatusRejected ReqStatus = "rejected"
	ReqStatusExpired  ReqStatus = "expired"
)

// Role 群成员角色
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// PermissionLevel 文件权限级别
type PermissionLevel string

const (
	PermView     PermissionLevel = "view"
	PermDownload PermissionLevel = "download"
	PermShare    PermissionLevel = "share"
	PermManage   PermissionLevel = "manage"
)

// MsgType 消息内容类型
type MsgType string

const (
	MsgTypeText         MsgType = "text"
	MsgTypeImage        MsgType = "image"
	MsgTypeFile         MsgType = "file"
	MsgTypeVoice        MsgType = "voice"
	MsgTypeVideo        MsgType = "video"
	MsgTypeLink         MsgType = "link"
	MsgTypeEmoji        MsgType = "emoji"
	MsgTypeAnnouncement MsgType = "announcement"
)

// FileStatus 文件状态
type FileStatus string

const (
	FileStatusActive   FileStatus = "active"
	FileStatusDeleted  FileStatus = "deleted"
	FileStatusArchived FileStatus = "archived"
)

// AccessType 文件权限访问类型
type AccessType string

const (
	AccessTypeUser   AccessType = "user"
	AccessTypeFriend AccessType = "friend"
	AccessTypeGroup  AccessType = "group"
	AccessTypePublic AccessType = "public"
)

// AssociationType 文件关联类型
type AssociationType string

const (
	AssocPrivateMessage AssociationType = "private_message"
	AssocGroupMessage   AssociationType = "group_message"
	AssocUserAvatar     AssociationType = "user_avatar"
	AssocGroupAvatar    AssociationType = "group_avatar"
	AssocPostAttachment AssociationType = "post_attachment"
)

// ============================================================
// 实体
// ============================================================

// User 用户（对应 user 表）
type User struct {
	UID        string  `db:"uid"         json:"uid"`
	Username   string  `db:"username"    json:"username"`
	Account    string  `db:"account"     json:"account"`
	Password   string  `db:"password"    json:"-"` // 不序列化
	Gender     *string `db:"gender"      json:"gender,omitempty"`
	Region     *string `db:"region"      json:"region,omitempty"`
	Email      *string `db:"email"       json:"email,omitempty"`
	CreateTime *int64  `db:"create_time" json:"create_time,omitempty"`
	Avatar     *string `db:"avatar"      json:"avatar,omitempty"`
	Bio        *string `db:"bio"         json:"bio,omitempty"`
}

// UserProfile 不含密码的安全投影
type UserProfile struct {
	UID      string `json:"uid"`
	Username string `json:"username"`
	Account  string `json:"account"`
	Gender   string `json:"gender"`
	Region   string `json:"region,omitempty"`
	Email    string `json:"email,omitempty"`
	Avatar   string `json:"avatar"`
	Bio      string `json:"bio"`
}

// ToProfile 剥离 Password，返回安全投影
func (u *User) ToProfile() *UserProfile {
	return &UserProfile{
		UID:      u.UID,
		Username: u.Username,
		Account:  u.Account,
		Gender:   PtrVal(u.Gender),
		Region:   PtrVal(u.Region),
		Email:    PtrVal(u.Email),
		Avatar:   PtrVal(u.Avatar),
		Bio:      PtrVal(u.Bio),
	}
}

// UserOnline 在线用户信息（Redis Set）
type UserOnline struct {
	Account  string `json:"account"`
	Username string `json:"username"`
}

// Friends 好友关系（对应 friends 表，uid < to_uid）
type Friends struct {
	FID           string  `db:"fid"             json:"fid"`
	UID           string  `db:"uid"             json:"uid"`     // 较小 UID
	ToUID         string  `db:"to_uid"          json:"to_uid"`  // 较大 UID
	CreateTime    *int64  `db:"create_time"     json:"create_time,omitempty"`
	IsBlacklist   *bool   `db:"is_blacklist"    json:"is_blacklist,omitempty"`
	ToIsBlacklist *bool   `db:"to_is_blacklist" json:"to_is_blacklist,omitempty"`
	Remark        *string `db:"remark"          json:"remark,omitempty"`
	ToRemark      *string `db:"to_remark"       json:"to_remark,omitempty"`
	GroupBy       *string `db:"groupby"         json:"groupby,omitempty"`
	ToGroupBy     *string `db:"to_groupby"      json:"to_groupby,omitempty"`
}

// FriendRequest 好友申请（对应 friend_request 表）
type FriendRequest struct {
	ReqID       string    `db:"req_id"       json:"req_id"`
	SenderUID   string    `db:"sender_uid"   json:"sender_uid"`
	ReceiverUID string    `db:"receiver_uid" json:"receiver_uid"`
	Status      ReqStatus `db:"status"       json:"status"`
	ApplyText   *string   `db:"apply_text"   json:"apply_text,omitempty"`
	CreateTime  *int64    `db:"create_time"  json:"create_time,omitempty"`
	HandleTime  *int64    `db:"handle_time"  json:"handle_time,omitempty"`
}

// PrivateChat 私聊会话（对应 private_chat 表）
type PrivateChat struct {
	PID               string `db:"pid"                  json:"pid"`
	UID1              string `db:"uid1"                 json:"uid1"` // 较小 UID
	UID2              string `db:"uid2"                 json:"uid2"` // 较大 UID
	CreateTime        *int64 `db:"create_time"          json:"create_time,omitempty"`
	IsPinnedByUID1    *bool  `db:"is_pinned_by_uid1"    json:"is_pinned_by_uid1,omitempty"`
	IsPinnedByUID2    *bool  `db:"is_pinned_by_uid2"    json:"is_pinned_by_uid2,omitempty"`
	DoNotDisturbUID1  *bool  `db:"do_not_disturb_uid1"  json:"do_not_disturb_uid1,omitempty"`
	DoNotDisturbUID2  *bool  `db:"do_not_disturb_uid2"  json:"do_not_disturb_uid2,omitempty"`
}

// PrivateMessage 私聊消息（对应 private_message 表）
type PrivateMessage struct {
	MsgID     string  `db:"msg_id"     json:"msg_id"`
	PID       string  `db:"pid"        json:"pid"`
	SeqID     int64   `db:"seq_id"     json:"seq_id"`
	Content   string  `db:"content"    json:"content"`
	SenderUID string  `db:"sender_uid" json:"sender_uid"`
	SendTime  *int64  `db:"send_time"  json:"send_time,omitempty"`
	IsRevoked *bool   `db:"is_revoked" json:"is_revoked,omitempty"`
	IsRead    *bool   `db:"is_read"    json:"is_read,omitempty"`
	Type      MsgType `db:"type"       json:"type"`
	ExtInfo   *string `db:"ext_info"   json:"ext_info,omitempty"`
}

// GroupChat 群聊（对应 group_chat 表）
type GroupChat struct {
	GID         string  `db:"gid"          json:"gid"`
	GroupName   string  `db:"group_name"   json:"group_name"`
	ManagerUID  string  `db:"manager_uid"  json:"manager_uid"`
	GroupAvatar *string `db:"group_avatar" json:"group_avatar,omitempty"`
	GroupIntro  *string `db:"group_intro"  json:"group_intro,omitempty"`
	CreateTime  *int64  `db:"create_time"  json:"create_time,omitempty"`
}

// GroupMember 群成员（对应 group_member 表）
type GroupMember struct {
	UID          string  `db:"uid"            json:"uid"`
	GID          string  `db:"gid"            json:"gid"`
	Role         Role    `db:"role"           json:"role"`
	Nickname     *string `db:"nickname"       json:"nickname,omitempty"`
	Level        *int    `db:"level"          json:"level,omitempty"`
	JoinTime     *int64  `db:"join_time"      json:"join_time,omitempty"`
	DoNotDisturb *bool   `db:"do_not_disturb" json:"do_not_disturb,omitempty"`
	GroupBy      *string `db:"group_by"       json:"group_by,omitempty"`
	Remark       *string `db:"remark"         json:"remark,omitempty"`
	IsPinned     *bool   `db:"is_pinned"      json:"is_pinned,omitempty"`
}

// MuteRecord 禁言记录（对应 mute_record 表）
type MuteRecord struct {
	BanID        string `db:"ban_id"        json:"ban_id"`
	GID          string `db:"gid"           json:"gid"`
	UID          string `db:"uid"           json:"uid"`
	MuteDuration int    `db:"mute_duration" json:"mute_duration"` // 秒，-1=永久
	StartTime    *int64 `db:"start_time"    json:"start_time,omitempty"`
}

// GroupMessage 群聊消息（对应 group_message 表）
type GroupMessage struct {
	MsgID          string  `db:"msg_id"          json:"msg_id"`
	GID            string  `db:"gid"             json:"gid"`
	SeqID          int64   `db:"seq_id"          json:"seq_id"`
	Content        string  `db:"content"         json:"content"`
	SenderUID      string  `db:"sender_uid"      json:"sender_uid"`
	SendTime       *int64  `db:"send_time"       json:"send_time,omitempty"`
	IsRevoked      *bool   `db:"is_revoked"      json:"is_revoked,omitempty"`
	Type           MsgType `db:"type"            json:"type"`
	MentionedUIDs  *string `db:"mentioned_uids"  json:"mentioned_uids,omitempty"`  // JSON 数组
	QuoteMsgID     *string `db:"quote_msg_id"    json:"quote_msg_id,omitempty"`
	IsAnnouncement *bool   `db:"is_announcement" json:"is_announcement,omitempty"`
	ExtInfo        *string `db:"ext_info"        json:"ext_info,omitempty"`
}

// GroupMessageRead 群消息已读（对应 group_message_read 表）
type GroupMessageRead struct {
	MsgID string `db:"msg_id" json:"msg_id"`
	GID   string `db:"gid"    json:"gid"`
	UID   string `db:"uid"    json:"uid"`
}

// GroupJoinRequest 群加入申请（对应 group_join_request 表）
type GroupJoinRequest struct {
	ReqID        string    `db:"req_id"        json:"req_id"`
	GID          string    `db:"gid"           json:"gid"`
	ApplicantUID string    `db:"applicant_uid" json:"applicant_uid"`
	ApproverUID  *string   `db:"approver_uid"  json:"approver_uid,omitempty"`
	Status       ReqStatus `db:"status"        json:"status"`
	ApplyText    *string   `db:"apply_text"    json:"apply_text,omitempty"`
	CreateTime   *int64    `db:"create_time"   json:"create_time,omitempty"`
	HandleTime   *int64    `db:"handle_time"   json:"handle_time,omitempty"`
}

// FileStorage 物理文件存储（对应 file_storage 表）
type FileStorage struct {
	StorageID      string  `db:"storage_id"       json:"storage_id"`
	FileHash       string  `db:"file_hash"        json:"file_hash"`
	FilePath       string  `db:"file_path"        json:"file_path"`
	ThumbnailPath  *string `db:"thumbnail_path"   json:"thumbnail_path,omitempty"`
	FileSize       int64   `db:"file_size"        json:"file_size"`
	MimeType       string  `db:"mime_type"        json:"mime_type"`
	CreateTime     *int64  `db:"create_time"      json:"create_time,omitempty"`
	ReferenceCount int     `db:"reference_count"  json:"reference_count"`
	StorageLocation string `db:"storage_location"  json:"storage_location"`
}

// FileMetadata 文件元数据（对应 file_metadata 表）
type FileMetadata struct {
	FileID         string     `db:"file_id"          json:"file_id"`
	StorageID      string     `db:"storage_id"       json:"storage_id"`
	OwnerUID       string     `db:"owner_uid"        json:"owner_uid"`
	OriginalName   string     `db:"original_name"    json:"original_name"`
	DisplayName    string     `db:"display_name"     json:"display_name"`
	FileType       string     `db:"file_type"        json:"file_type"`
	UploadTime     *int64     `db:"upload_time"      json:"upload_time,omitempty"`
	LastAccessTime *int64     `db:"last_access_time" json:"last_access_time,omitempty"`
	DownloadCount  int64      `db:"download_count"   json:"download_count"`
	FileStatus     FileStatus `db:"file_status"      json:"file_status"`
}

// FilePermission 文件权限（对应 file_permission 表）
type FilePermission struct {
	PermissionID    string          `db:"permission_id"    json:"permission_id"`
	FileID          string          `db:"file_id"          json:"file_id"`
	AccessType      AccessType      `db:"access_type"      json:"access_type"`
	TargetID        *string         `db:"target_id"        json:"target_id,omitempty"`
	PermissionLevel PermissionLevel `db:"permission_level" json:"permission_level"`
	GrantedBy       string          `db:"granted_by"       json:"granted_by"`
	GrantedAt       *int64          `db:"granted_at"       json:"granted_at,omitempty"`
	ExpiresAt       *int64          `db:"expires_at"       json:"expires_at,omitempty"`
}

// FileAssociation 文件关联（对应 file_association 表）
type FileAssociation struct {
	AssociationID   string          `db:"association_id"   json:"association_id"`
	FileID          string          `db:"file_id"          json:"file_id"`
	AssociationType AssociationType `db:"association_type" json:"association_type"`
	AssociatedID    string          `db:"associated_id"    json:"associated_id"`
	CreatorUID      string          `db:"creator_uid"      json:"creator_uid"`
	CreateTime      *int64          `db:"create_time"      json:"create_time,omitempty"`
}

// ============================================================
// 工具函数
// ============================================================

// PtrVal 指针解引用：nil 返回零值，非 nil 返回解引用值
func PtrVal[T any](p *T) T {
	if p == nil {
		var z T
		return z
	}
	return *p
}

// Ptr 创建指针
func Ptr[T any](v T) *T { return &v }

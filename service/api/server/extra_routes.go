package main

import (
	"context"

	"trpc.group/trpc-go/trpc-go/log"
	"trpc.group/trpc-go/trpc-go/restful"
	"trpc.group/trpc-go/trpc-go/server"
)

// ExtraService 聚合所有额外 API handler。
type ExtraService struct {
	User    *extraUserImpl
	Friend  *extraFriendImpl
	Message *extraMessageImpl
	Group   *extraGroupImpl
	File    *extraFileImpl
	Chat    *extraChatImpl
	Misc    *extraMiscImpl
	Final   *extraFinalImpl
}

// extraBinding 创建绑定，闭包捕获 svc 和 handler。
func extraBinding(method, path string, handler func(svc *ExtraService, ctx context.Context)) *restful.Binding {
	return &restful.Binding{
		Name:  path,
		Input: func() restful.ProtoMessage { return nil },
		Filter: func(svc interface{}, ctx context.Context, _ interface{}) (interface{}, error) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("[API] handler panic: path=%s, err=%v", path, r)
				}
			}()
			handler(svc.(*ExtraService), ctx)
			return nil, nil
		},
		HTTPMethod: method,
		Pattern:    restful.Enforce(path),
	}
}

// ExtraServiceServer_ServiceDesc 所有额外 API 的 ServiceDesc。
var ExtraServiceServer_ServiceDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: (*interface{})(nil),
	Methods: []server.Method{
		{Name: "/extra/user/profile", Bindings: []*restful.Binding{
			extraBinding("PUT", "/api/v1/user/profile", func(s *ExtraService, ctx context.Context) { s.User.UpdateProfile(ctx) }),
		}},
		{Name: "/extra/user/search", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/user/search", func(s *ExtraService, ctx context.Context) { s.User.SearchUser(ctx) }),
		}},
		{Name: "/extra/friend/blacklist", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/friend/blacklist", func(s *ExtraService, ctx context.Context) { s.Friend.BlacklistFriend(ctx) }),
		}},
		{Name: "/extra/friend/unblacklist", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/friend/unblacklist", func(s *ExtraService, ctx context.Context) { s.Friend.UnblacklistFriend(ctx) }),
		}},
		{Name: "/extra/message/revoke", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/message/revoke", func(s *ExtraService, ctx context.Context) { s.Message.RevokeMessage(ctx) }),
		}},
		{Name: "/extra/message/unread-count", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/message/unread-count", func(s *ExtraService, ctx context.Context) { s.Message.GetUnreadCount(ctx) }),
		}},
		{Name: "/extra/group/kick", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/kick", func(s *ExtraService, ctx context.Context) { s.Group.KickMember(ctx) }),
		}},
		{Name: "/extra/group/mute", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/mute", func(s *ExtraService, ctx context.Context) { s.Group.MuteMember(ctx) }),
		}},
		{Name: "/extra/group/unmute", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/unmute", func(s *ExtraService, ctx context.Context) { s.Group.UnmuteMember(ctx) }),
		}},
		{Name: "/extra/group/role", Bindings: []*restful.Binding{
			extraBinding("PUT", "/api/v1/group/role", func(s *ExtraService, ctx context.Context) { s.Group.UpdateMemberRole(ctx) }),
		}},
		{Name: "/extra/group/requests", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/requests", func(s *ExtraService, ctx context.Context) { s.Group.GetGroupRequests(ctx) }),
		}},
		{Name: "/extra/group/approve-request", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/approve-request", func(s *ExtraService, ctx context.Context) { s.Group.ApproveGroupRequest(ctx) }),
		}},
		// Chat
		{Name: "/extra/chat/conversations", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/chat/conversations", func(s *ExtraService, ctx context.Context) { s.Chat.GetConversations(ctx) }),
		}},
		{Name: "/extra/chat/pin", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/chat/pin", func(s *ExtraService, ctx context.Context) { s.Chat.PinConversation(ctx) }),
		}},
		{Name: "/extra/chat/online", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/chat/online-status", func(s *ExtraService, ctx context.Context) { s.Chat.GetOnlineStatus(ctx) }),
		}},
		// Group extra
		{Name: "/extra/group/disband", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/disband", func(s *ExtraService, ctx context.Context) { s.Chat.DisbandGroup(ctx) }),
		}},
		{Name: "/extra/group/search", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/search", func(s *ExtraService, ctx context.Context) { s.Chat.SearchGroup(ctx) }),
		}},
		{Name: "/extra/group/announces", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/announces", func(s *ExtraService, ctx context.Context) { s.Chat.GetGroupAnnounces(ctx) }),
		}},
		// Misc
		{Name: "/extra/misc/batch-users", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/user/batch", func(s *ExtraService, ctx context.Context) { s.Misc.BatchGetUsers(ctx) }),
		}},
		{Name: "/extra/misc/file-revoke-perm", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/file/revoke-permission", func(s *ExtraService, ctx context.Context) { s.Misc.RevokeFilePermission(ctx) }),
		}},
		{Name: "/extra/misc/file-associations", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/file/associations", func(s *ExtraService, ctx context.Context) { s.Misc.GetFileAssociations(ctx) }),
		}},
		{Name: "/extra/misc/file-associate", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/file/associate", func(s *ExtraService, ctx context.Context) { s.Misc.CreateFileAssociation(ctx) }),
		}},
		{Name: "/extra/misc/msg-read-counts", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/message/read-counts", func(s *ExtraService, ctx context.Context) { s.Misc.GetMessageReadCounts(ctx) }),
		}},
		// Final batch
		{Name: "/extra/final/search-by-region", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/user/search-by-region", func(s *ExtraService, ctx context.Context) { s.Final.SearchUserByRegion(ctx) }),
		}},
		{Name: "/extra/final/delete-account", Bindings: []*restful.Binding{
			extraBinding("DELETE", "/api/v1/user/account", func(s *ExtraService, ctx context.Context) { s.Final.DeleteAccount(ctx) }),
		}},
		{Name: "/extra/final/cancel-friend-req", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/friend/cancel-request", func(s *ExtraService, ctx context.Context) { s.Final.CancelFriendRequest(ctx) }),
		}},
		{Name: "/extra/final/blacklist-list", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/friend/blacklist-list", func(s *ExtraService, ctx context.Context) { s.Final.BlacklistList(ctx) }),
		}},
		{Name: "/extra/final/unread-list", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/message/unread-list", func(s *ExtraService, ctx context.Context) { s.Final.UnreadMessages(ctx) }),
		}},
		{Name: "/extra/final/read-users", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/message/read-users", func(s *ExtraService, ctx context.Context) { s.Final.MessageReadUsers(ctx) }),
		}},
		{Name: "/extra/final/chat-count", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/message/chat-count", func(s *ExtraService, ctx context.Context) { s.Final.MessageChatCount(ctx) }),
		}},
		{Name: "/extra/final/owned-groups", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/owned", func(s *ExtraService, ctx context.Context) { s.Final.OwnedGroups(ctx) }),
		}},
		{Name: "/extra/final/mute-list", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/mute-list", func(s *ExtraService, ctx context.Context) { s.Final.MuteList(ctx) }),
		}},
		{Name: "/extra/final/online-members", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/online-members", func(s *ExtraService, ctx context.Context) { s.Final.GroupOnlineMembers(ctx) }),
		}},
		{Name: "/extra/final/my-group-reqs", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/my-requests", func(s *ExtraService, ctx context.Context) { s.Final.MyGroupRequests(ctx) }),
		}},
		{Name: "/extra/final/all-group-reqs", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/group/all-requests", func(s *ExtraService, ctx context.Context) { s.Final.AllGroupRequests(ctx) }),
		}},
		{Name: "/extra/final/del-file-assoc", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/file/delete-association", func(s *ExtraService, ctx context.Context) { s.Final.DeleteFileAssociation(ctx) }),
		}},
		// File
		{Name: "/extra/file/permission", Bindings: []*restful.Binding{
			extraBinding("POST", "/api/v1/file/permission", func(s *ExtraService, ctx context.Context) { s.File.SetFilePermission(ctx) }),
		}},
	},
}

// RegisterExtraService 注册额外 API。
func RegisterExtraService(s server.Service, svc *ExtraService) {
	if err := s.Register(&ExtraServiceServer_ServiceDesc, svc); err != nil {
		panic("ExtraService register error:" + err.Error())
	}
}

package trpc

import (
	"context"

	"trpc.group/trpc-go/trpc-go/restful"
	"trpc.group/trpc-go/trpc-go/server"

	pb "echat/service/api/stub"
)

// FriendServiceService 好友服务接口。
type FriendServiceService interface {
	ApplyFriend(ctx context.Context, req *pb.ApplyFriendRequest) (*pb.ApplyFriendResponse, error)
	AcceptFriend(ctx context.Context, req *pb.AcceptFriendRequest) (*pb.AcceptFriendResponse, error)
	RejectFriend(ctx context.Context, req *pb.RejectFriendRequest) (*pb.RejectFriendResponse, error)
	DeleteFriend(ctx context.Context, req *pb.DeleteFriendRequest) (*pb.DeleteFriendResponse, error)
	ListFriends(ctx context.Context, req *pb.ListFriendsRequest) (*pb.ListFriendsResponse, error)
	ListRequests(ctx context.Context, req *pb.ListRequestsRequest) (*pb.ListRequestsResponse, error)
}

type bodyApplyFriend struct{}

func (bodyApplyFriend) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.ApplyFriendRequest) }
func (bodyApplyFriend) Body() string                              { return "*" }

type bodyAcceptFriend struct{}

func (bodyAcceptFriend) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.AcceptFriendRequest) }
func (bodyAcceptFriend) Body() string                              { return "*" }

type bodyRejectFriend struct{}

func (bodyRejectFriend) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.RejectFriendRequest) }
func (bodyRejectFriend) Body() string                              { return "*" }

// FriendServiceServer_ServiceDesc 好友服务描述符。
var FriendServiceServer_ServiceDesc = server.ServiceDesc{
	ServiceName: "echat.api.FriendService",
	HandlerType: ((*FriendServiceService)(nil)),
	Methods: []server.Method{
		{
			Name: "/echat.api.FriendService/ApplyFriend",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.ApplyFriendRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FriendServiceService).ApplyFriend(ctx, rb.(*pb.ApplyFriendRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.FriendService/ApplyFriend",
				Input:  func() restful.ProtoMessage { return new(pb.ApplyFriendRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FriendServiceService).ApplyFriend(ctx, rb.(*pb.ApplyFriendRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/friend/apply"), Body: bodyApplyFriend{},
			}},
		},
		{
			Name: "/echat.api.FriendService/AcceptFriend",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.AcceptFriendRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FriendServiceService).AcceptFriend(ctx, rb.(*pb.AcceptFriendRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.FriendService/AcceptFriend",
				Input:  func() restful.ProtoMessage { return new(pb.AcceptFriendRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FriendServiceService).AcceptFriend(ctx, rb.(*pb.AcceptFriendRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/friend/accept"), Body: bodyAcceptFriend{},
			}},
		},
		{
			Name: "/echat.api.FriendService/RejectFriend",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.RejectFriendRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FriendServiceService).RejectFriend(ctx, rb.(*pb.RejectFriendRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.FriendService/RejectFriend",
				Input:  func() restful.ProtoMessage { return new(pb.RejectFriendRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FriendServiceService).RejectFriend(ctx, rb.(*pb.RejectFriendRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/friend/reject"), Body: bodyRejectFriend{},
			}},
		},
		{
			Name: "/echat.api.FriendService/DeleteFriend",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.DeleteFriendRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FriendServiceService).DeleteFriend(ctx, rb.(*pb.DeleteFriendRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.FriendService/DeleteFriend",
				Input:  func() restful.ProtoMessage { return new(pb.DeleteFriendRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FriendServiceService).DeleteFriend(ctx, rb.(*pb.DeleteFriendRequest))
				},
				HTTPMethod: "DELETE", Pattern: restful.Enforce("/api/v1/friend/{fid}"),
			}},
		},
		{
			Name: "/echat.api.FriendService/ListFriends",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.ListFriendsRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FriendServiceService).ListFriends(ctx, rb.(*pb.ListFriendsRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.FriendService/ListFriends",
				Input:  func() restful.ProtoMessage { return new(pb.ListFriendsRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FriendServiceService).ListFriends(ctx, rb.(*pb.ListFriendsRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/friend/list"),
			}},
		},
		{
			Name: "/echat.api.FriendService/ListRequests",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.ListRequestsRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FriendServiceService).ListRequests(ctx, rb.(*pb.ListRequestsRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.FriendService/ListRequests",
				Input:  func() restful.ProtoMessage { return new(pb.ListRequestsRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FriendServiceService).ListRequests(ctx, rb.(*pb.ListRequestsRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/friend/requests"),
			}},
		},
	},
}

// RegisterFriendServiceService 注册好友服务。
func RegisterFriendServiceService(s server.Service, svr FriendServiceService) {
	if err := s.Register(&FriendServiceServer_ServiceDesc, svr); err != nil {
		panic("FriendService register error:" + err.Error())
	}
}

package trpc

import (
	"context"

	"trpc.group/trpc-go/trpc-go/restful"
	"trpc.group/trpc-go/trpc-go/server"

	pb "echat/service/api/stub"
)

// GroupServiceService 群组服务接口。
type GroupServiceService interface {
	CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error)
	JoinGroup(ctx context.Context, req *pb.JoinGroupRequest) (*pb.JoinGroupResponse, error)
	LeaveGroup(ctx context.Context, req *pb.LeaveGroupRequest) (*pb.LeaveGroupResponse, error)
	GetMembers(ctx context.Context, req *pb.GetMembersRequest) (*pb.GetMembersResponse, error)
	GetMyGroups(ctx context.Context, req *pb.GetMyGroupsRequest) (*pb.GetMyGroupsResponse, error)
}

type bodyCreateGroup struct{}

func (bodyCreateGroup) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.CreateGroupRequest) }
func (bodyCreateGroup) Body() string                              { return "*" }

type bodyJoinGroup struct{}

func (bodyJoinGroup) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.JoinGroupRequest) }
func (bodyJoinGroup) Body() string                              { return "*" }

type bodyLeaveGroup struct{}

func (bodyLeaveGroup) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.LeaveGroupRequest) }
func (bodyLeaveGroup) Body() string                              { return "*" }

// GroupServiceServer_ServiceDesc 群组服务描述符。
var GroupServiceServer_ServiceDesc = server.ServiceDesc{
	ServiceName: "echat.api.GroupService",
	HandlerType: ((*GroupServiceService)(nil)),
	Methods: []server.Method{
		{
			Name: "/echat.api.GroupService/CreateGroup",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.CreateGroupRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(GroupServiceService).CreateGroup(ctx, rb.(*pb.CreateGroupRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.GroupService/CreateGroup",
				Input:  func() restful.ProtoMessage { return new(pb.CreateGroupRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(GroupServiceService).CreateGroup(ctx, rb.(*pb.CreateGroupRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/group/create"), Body: bodyCreateGroup{},
			}},
		},
		{
			Name: "/echat.api.GroupService/JoinGroup",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.JoinGroupRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(GroupServiceService).JoinGroup(ctx, rb.(*pb.JoinGroupRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.GroupService/JoinGroup",
				Input:  func() restful.ProtoMessage { return new(pb.JoinGroupRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(GroupServiceService).JoinGroup(ctx, rb.(*pb.JoinGroupRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/group/join"), Body: bodyJoinGroup{},
			}},
		},
		{
			Name: "/echat.api.GroupService/LeaveGroup",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.LeaveGroupRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(GroupServiceService).LeaveGroup(ctx, rb.(*pb.LeaveGroupRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.GroupService/LeaveGroup",
				Input:  func() restful.ProtoMessage { return new(pb.LeaveGroupRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(GroupServiceService).LeaveGroup(ctx, rb.(*pb.LeaveGroupRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/group/leave"), Body: bodyLeaveGroup{},
			}},
		},
		{
			Name: "/echat.api.GroupService/GetMembers",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.GetMembersRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(GroupServiceService).GetMembers(ctx, rb.(*pb.GetMembersRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.GroupService/GetMembers",
				Input:  func() restful.ProtoMessage { return new(pb.GetMembersRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(GroupServiceService).GetMembers(ctx, rb.(*pb.GetMembersRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/group/members"),
			}},
		},
		{
			Name: "/echat.api.GroupService/GetMyGroups",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.GetMyGroupsRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(GroupServiceService).GetMyGroups(ctx, rb.(*pb.GetMyGroupsRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.GroupService/GetMyGroups",
				Input:  func() restful.ProtoMessage { return new(pb.GetMyGroupsRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(GroupServiceService).GetMyGroups(ctx, rb.(*pb.GetMyGroupsRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/group/my"),
			}},
		},
	},
}

// RegisterGroupServiceService 注册群组服务。
func RegisterGroupServiceService(s server.Service, svr GroupServiceService) {
	if err := s.Register(&GroupServiceServer_ServiceDesc, svr); err != nil {
		panic("GroupService register error:" + err.Error())
	}
}

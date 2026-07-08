package trpc

import (
	"context"

	"trpc.group/trpc-go/trpc-go/restful"
	"trpc.group/trpc-go/trpc-go/server"

	pb "echat/service/api/stub"
)

// MessageServiceService 消息服务接口。
type MessageServiceService interface {
	GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.GetHistoryResponse, error)
	MarkRead(ctx context.Context, req *pb.MarkReadRequest) (*pb.MarkReadResponse, error)
}

type bodyMarkRead struct{}

func (bodyMarkRead) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.MarkReadRequest) }
func (bodyMarkRead) Body() string                              { return "*" }

// MessageServiceServer_ServiceDesc 消息服务描述符。
var MessageServiceServer_ServiceDesc = server.ServiceDesc{
	ServiceName: "echat.api.MessageService",
	HandlerType: ((*MessageServiceService)(nil)),
	Methods: []server.Method{
		{
			Name: "/echat.api.MessageService/GetHistory",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.GetHistoryRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(MessageServiceService).GetHistory(ctx, rb.(*pb.GetHistoryRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.MessageService/GetHistory",
				Input:  func() restful.ProtoMessage { return new(pb.GetHistoryRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(MessageServiceService).GetHistory(ctx, rb.(*pb.GetHistoryRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/message/history"),
			}},
		},
		{
			Name: "/echat.api.MessageService/MarkRead",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.MarkReadRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(MessageServiceService).MarkRead(ctx, rb.(*pb.MarkReadRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name:   "/echat.api.MessageService/MarkRead",
				Input:  func() restful.ProtoMessage { return new(pb.MarkReadRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(MessageServiceService).MarkRead(ctx, rb.(*pb.MarkReadRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/message/read"), Body: bodyMarkRead{},
			}},
		},
	},
}

// RegisterMessageServiceService 注册消息服务。
func RegisterMessageServiceService(s server.Service, svr MessageServiceService) {
	if err := s.Register(&MessageServiceServer_ServiceDesc, svr); err != nil {
		panic("MessageService register error:" + err.Error())
	}
}

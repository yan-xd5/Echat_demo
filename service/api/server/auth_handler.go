package main

import (
	"context"

	"google.golang.org/protobuf/types/known/structpb"
	"trpc.group/trpc-go/trpc-go/restful"
	"trpc.group/trpc-go/trpc-go/server"

	"echat/sdk/auth"
)

// AuthServiceService 认证服务接口。
type AuthServiceService interface {
	GetPublicKey(ctx context.Context) (*structpb.Struct, error)
}

type authImpl struct{}

func (s *authImpl) GetPublicKey(ctx context.Context) (*structpb.Struct, error) {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"public_key": structpb.NewStringValue(auth.GetPublicKeyPEM()),
		},
	}, nil
}

// AuthServiceServer_ServiceDesc 仅用于 RESTful 路由注册。
var AuthServiceServer_ServiceDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: ((*AuthServiceService)(nil)),
	Methods: []server.Method{
		{
			Name: "/echat.api.AuthService/GetPublicKey",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				return svr.(AuthServiceService).GetPublicKey(ctx)
			},
			Bindings: []*restful.Binding{{
				Name:  "/echat.api.AuthService/GetPublicKey",
				Input: func() restful.ProtoMessage { return nil },
				Filter: func(svc interface{}, ctx context.Context, _ interface{}) (interface{}, error) {
					return svc.(AuthServiceService).GetPublicKey(ctx)
				},
				HTTPMethod: "GET",
				Pattern:    restful.Enforce("/api/v1/auth/public-key"),
			}},
		},
	},
}

// RegisterAuthServiceService 注册认证服务。
func RegisterAuthServiceService(s server.Service, svr AuthServiceService) {
	if err := s.Register(&AuthServiceServer_ServiceDesc, svr); err != nil {
		panic("AuthService register error:" + err.Error())
	}
}

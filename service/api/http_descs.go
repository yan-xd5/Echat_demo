package main

import (
	"echat/service/api/internal/restful"
	pb "echat/service/api/stub"
	"trpc.group/trpc-go/trpc-go/server"
)

// HTTP 版 ServiceDesc，ServiceName 改为 trpc_go.yaml 中 HTTP 传输名 echat.api.UserService.http。
// 所有服务共享同一 HTTP 传输端口 9001，因此需要统一 ServiceName。
// 方法和 Handler 复用原 Desc，仅 ServiceName 修改。

var userServiceHTTPDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: pb.UserServiceServer_ServiceDesc.HandlerType,
	Methods:     pb.UserServiceServer_ServiceDesc.Methods,
}

var friendServiceHTTPDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: FriendServiceServer_ServiceDesc.HandlerType,
	Methods:     FriendServiceServer_ServiceDesc.Methods,
}

var messageServiceHTTPDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: MessageServiceServer_ServiceDesc.HandlerType,
	Methods:     MessageServiceServer_ServiceDesc.Methods,
}

var groupServiceHTTPDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: GroupServiceServer_ServiceDesc.HandlerType,
	Methods:     GroupServiceServer_ServiceDesc.Methods,
}

var extraServiceHTTPDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: (*interface{})(nil),
	Methods:     restful.ExtraServiceServer_ServiceDesc.Methods,
}

var fileServiceHTTPDesc = server.ServiceDesc{
	ServiceName: "echat.api.UserService.http",
	HandlerType: FileServiceServer_ServiceDesc.HandlerType,
	Methods:     FileServiceServer_ServiceDesc.Methods,
}

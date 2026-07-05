package main

import (
	"context"

	"trpc.group/trpc-go/trpc-go/restful"
	"trpc.group/trpc-go/trpc-go/server"

	pb "echat/service/api/stub"
)

type FileServiceService interface {
	Upload(ctx context.Context, req *pb.UploadRequest) (*pb.UploadResponse, error)
	Download(ctx context.Context, req *pb.DownloadRequest) (*pb.DownloadResponse, error)
	Preview(ctx context.Context, req *pb.PreviewRequest) (*pb.PreviewResponse, error)
	Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error)
	ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error)
}

type bodyUpload struct{}
func (bodyUpload) Locate(m restful.ProtoMessage) interface{} { return m.(*pb.UploadRequest) }
func (bodyUpload) Body() string                              { return "*" }

var FileServiceServer_ServiceDesc = server.ServiceDesc{
	ServiceName: "echat.api.FileService",
	HandlerType: ((*FileServiceService)(nil)),
	Methods: []server.Method{
		{
			Name: "/echat.api.FileService/Upload",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.UploadRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FileServiceService).Upload(ctx, rb.(*pb.UploadRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name: "/echat.api.FileService/Upload", Input: func() restful.ProtoMessage { return new(pb.UploadRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FileServiceService).Upload(ctx, rb.(*pb.UploadRequest))
				},
				HTTPMethod: "POST", Pattern: restful.Enforce("/api/v1/file/upload"), Body: bodyUpload{},
			}},
		},
		{
			Name: "/echat.api.FileService/Download",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.DownloadRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FileServiceService).Download(ctx, rb.(*pb.DownloadRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name: "/echat.api.FileService/Download", Input: func() restful.ProtoMessage { return new(pb.DownloadRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FileServiceService).Download(ctx, rb.(*pb.DownloadRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/file/download"),
			}},
		},
		{
			Name: "/echat.api.FileService/Preview",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.PreviewRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FileServiceService).Preview(ctx, rb.(*pb.PreviewRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name: "/echat.api.FileService/Preview", Input: func() restful.ProtoMessage { return new(pb.PreviewRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FileServiceService).Preview(ctx, rb.(*pb.PreviewRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/file/preview"),
			}},
		},
		{
			Name: "/echat.api.FileService/Delete",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.DeleteRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FileServiceService).Delete(ctx, rb.(*pb.DeleteRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name: "/echat.api.FileService/Delete", Input: func() restful.ProtoMessage { return new(pb.DeleteRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FileServiceService).Delete(ctx, rb.(*pb.DeleteRequest))
				},
				HTTPMethod: "DELETE", Pattern: restful.Enforce("/api/v1/file/{file_id}"),
			}},
		},
		{
			Name: "/echat.api.FileService/ListFiles",
			Func: func(svr interface{}, ctx context.Context, f server.FilterFunc) (interface{}, error) {
				req := &pb.ListFilesRequest{}
				filters, _ := f(req)
				return filters.Filter(ctx, req, func(ctx context.Context, rb interface{}) (interface{}, error) {
					return svr.(FileServiceService).ListFiles(ctx, rb.(*pb.ListFilesRequest))
				})
			},
			Bindings: []*restful.Binding{{
				Name: "/echat.api.FileService/ListFiles", Input: func() restful.ProtoMessage { return new(pb.ListFilesRequest) },
				Filter: func(svc interface{}, ctx context.Context, rb interface{}) (interface{}, error) {
					return svc.(FileServiceService).ListFiles(ctx, rb.(*pb.ListFilesRequest))
				},
				HTTPMethod: "GET", Pattern: restful.Enforce("/api/v1/file/list"),
			}},
		},
	},
}

func RegisterFileServiceService(s server.Service, svr FileServiceService) {
	if err := s.Register(&FileServiceServer_ServiceDesc, svr); err != nil {
		panic("FileService register error:" + err.Error())
	}
}

package resource

import (
	"context"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Server struct {
	Config
}

type Config struct {
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

var _ pbresource.ResourceServiceServer = (*Server)(nil)

func (s *Server) Register(grpcServer *grpc.Server) {
	pbresource.RegisterResourceServiceServer(grpcServer, s)
}

func (s *Server) Read(ctx context.Context, req *pbresource.ReadRequest) (*pbresource.ReadResponse, error) {
	return nil, nil
}

func (s *Server) Write(ctx context.Context, req *pbresource.WriteRequest) (*pbresource.WriteResponse, error) {
	return nil, nil
}

func (s *Server) WriteStatus(ctx context.Context, req *pbresource.WriteStatusRequest) (*pbresource.WriteStatusResponse, error) {
	return nil, nil
}

func (s *Server) List(ctx context.Context, req *pbresource.ListRequest) (*pbresource.ListResponse, error) {
	return nil, nil
}

func (s *Server) Delete(ctx context.Context, req *pbresource.DeleteRequest) (*pbresource.DeleteResponse, error) {
	return nil, nil
}

func (s *Server) Watch(req *pbresource.WatchRequest, ws pbresource.ResourceService_WatchServer) error {
	return nil
}

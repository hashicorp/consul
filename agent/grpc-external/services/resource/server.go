package resource

import (
	"context"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Server struct {
	Config
}

type Config struct {
	registry resource.Registry
	backend  storage.Backend
}

//go:generate mockery --name Backend --inpackage
type Backend interface {
	storage.Backend
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

var _ pbresource.ResourceServiceServer = (*Server)(nil)

func (s *Server) Register(grpcServer *grpc.Server) {
	pbresource.RegisterResourceServiceServer(grpcServer, s)
}

func (s *Server) Write(ctx context.Context, req *pbresource.WriteRequest) (*pbresource.WriteResponse, error) {
	// TODO
	return &pbresource.WriteResponse{}, nil
}

func (s *Server) WriteStatus(ctx context.Context, req *pbresource.WriteStatusRequest) (*pbresource.WriteStatusResponse, error) {
	// TODO
	return &pbresource.WriteStatusResponse{}, nil
}

func (s *Server) List(ctx context.Context, req *pbresource.ListRequest) (*pbresource.ListResponse, error) {
	// TODO
	return &pbresource.ListResponse{}, nil
}

func (s *Server) Delete(ctx context.Context, req *pbresource.DeleteRequest) (*pbresource.DeleteResponse, error) {
	// TODO
	return &pbresource.DeleteResponse{}, nil
}

func (s *Server) Watch(req *pbresource.WatchRequest, ws pbresource.ResourceService_WatchServer) error {
	// TODO
	return nil
}

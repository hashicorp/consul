package resource

import (
	"context"

	"google.golang.org/grpc"

	"github.com/hashicorp/consul/internal/resource"
	storage "github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Server struct {
	Config
}

type Config struct {
	registry Registry
	backend  Backend
}

//go:generate mockery --name Registry --inpackage
type Registry interface {
	Resolve(typ *pbresource.Type) (reg resource.Registration, ok bool)
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

func (s *Server) Read(ctx context.Context, req *pbresource.ReadRequest) (*pbresource.ReadResponse, error) {
	// TODO
	return &pbresource.ReadResponse{}, nil
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

package resource

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
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

	// check type exists
	_, ok := s.registry.Resolve(req.Id.Type)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("resource type %s not registered", resource.ToGVK(req.Id.Type)))
	}

	readFn := s.backend.Read
	if isConsistentRead(ctx) {
		readFn = s.backend.ReadConsistent
	}

	resource, err := readFn(ctx, req.Id)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if _, ok := err.(storage.GroupVersionMismatchError); ok {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, err
	}
	return &pbresource.ReadResponse{Resource: resource}, nil
}

func isConsistentRead(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	vals := md.Get("x-consul-consistency-mode")
	if len(vals) == 0 {
		return false
	}

	return vals[0] == "consistent"
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

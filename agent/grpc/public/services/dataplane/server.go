package dataplane

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
)

type Server struct {
	Config
}

type Config struct {
	Logger      hclog.Logger
	ACLResolver ACLResolver
}

//go:generate mockery -name ACLResolver -inpkg
type ACLResolver interface {
	ResolveTokenAndDefaultMeta(string, *structs.EnterpriseMeta, *acl.AuthorizerContext) (acl.Authorizer, error)
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

var _ pbdataplane.DataplaneServiceServer = (*Server)(nil)

func (s *Server) Register(grpcServer *grpc.Server) {
	pbdataplane.RegisterDataplaneServiceServer(grpcServer, s)
}

package dataplane

import (
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
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
	ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (acl.Authorizer, error)
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

var _ pbdataplane.DataplaneServiceServer = (*Server)(nil)

func (s *Server) Register(grpcServer *grpc.Server) {
	pbdataplane.RegisterDataplaneServiceServer(grpcServer, s)
}

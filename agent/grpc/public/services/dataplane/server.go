package dataplane

import (
	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
)

type Server struct {
	Config
}

type Config struct {
	GetStore    func() StateStore
	Logger      hclog.Logger
	ACLResolver ACLResolver
	// Datacenter of the Consul server this gRPC server is hosted on
	Datacenter string
}

type StateStore interface {
	ServiceNode(string, string, string, *acl.EnterpriseMeta) (uint64, *structs.ServiceNode, error)
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

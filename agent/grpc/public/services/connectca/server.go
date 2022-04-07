package connectca

import (
	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
)

type Server struct {
	Config
}

type Config struct {
	GetStore    func() StateStore
	Logger      hclog.Logger
	ACLResolver ACLResolver
}

type StateStore interface {
	EventPublisher() state.EventPublisher
	CAConfig(memdb.WatchSet) (uint64, *structs.CAConfiguration, error)
	AbandonCh() <-chan struct{}
}

//go:generate mockery -name ACLResolver -inpkg
type ACLResolver interface {
	ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (acl.Authorizer, error)
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

func (s *Server) Register(grpcServer *grpc.Server) {
	pbconnectca.RegisterConnectCAServiceServer(grpcServer, s)
}

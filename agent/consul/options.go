package consul

import (
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
)

type Deps struct {
	Logger          hclog.InterceptLogger
	TLSConfigurator *tlsutil.Configurator
	Tokens          *token.Store
	Router          *router.Router
	ConnPool        *pool.ConnPool
	GRPCConnPool    GRPCClientConner
	LeaderForwarder LeaderForwarder
	EnterpriseDeps
}

type GRPCClientConner interface {
	ClientConn(datacenter string) (*grpc.ClientConn, error)
	ClientConnLeader() (*grpc.ClientConn, error)
	SetGatewayResolver(func(string) string)
}

type LeaderForwarder interface {
	// UpdateLeaderAddr updates the leader address in the local DC's resolver.
	UpdateLeaderAddr(datacenter, addr string)
}

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
}

type GRPCClientConner interface {
	ClientConn(datacenter string) (*grpc.ClientConn, error)
}

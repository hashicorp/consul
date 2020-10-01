package consul

import (
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
)

type Deps struct {
	Logger          hclog.InterceptLogger
	TLSConfigurator *tlsutil.Configurator
	Tokens          *token.Store
	Router          *router.Router
	ConnPool        *pool.ConnPool
}

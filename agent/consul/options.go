package consul

import (
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
)

type ConsulOption struct {
	logger          hclog.InterceptLogger
	tlsConfigurator *tlsutil.Configurator
	connPool        *pool.ConnPool
	tokens          *token.Store
}

func WithLogger(logger hclog.InterceptLogger) ConsulOption {
	return ConsulOption{
		logger: logger,
	}
}

func WithTLSConfigurator(tlsConfigurator *tlsutil.Configurator) ConsulOption {
	return ConsulOption{
		tlsConfigurator: tlsConfigurator,
	}
}

func WithConnectionPool(connPool *pool.ConnPool) ConsulOption {
	return ConsulOption{
		connPool: connPool,
	}
}

func WithTokenStore(tokens *token.Store) ConsulOption {
	return ConsulOption{
		tokens: tokens,
	}
}

func flattenConsulOptions(options []ConsulOption) ConsulOption {
	var flat ConsulOption
	for _, opt := range options {
		if opt.logger != nil {
			flat.logger = opt.logger
		}
		if opt.tlsConfigurator != nil {
			flat.tlsConfigurator = opt.tlsConfigurator
		}
		if opt.connPool != nil {
			flat.connPool = opt.connPool
		}
		if opt.tokens != nil {
			flat.tokens = opt.tokens
		}
	}
	return flat
}

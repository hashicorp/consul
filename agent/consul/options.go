package consul

import (
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
)

type consulOptions struct {
	logger          hclog.InterceptLogger
	tlsConfigurator *tlsutil.Configurator
	connPool        *pool.ConnPool
	tokens          *token.Store
}

type ConsulOption func(*consulOptions)

func WithLogger(logger hclog.InterceptLogger) ConsulOption {
	return func(opt *consulOptions) {
		opt.logger = logger
	}
}

func WithTLSConfigurator(tlsConfigurator *tlsutil.Configurator) ConsulOption {
	return func(opt *consulOptions) {
		opt.tlsConfigurator = tlsConfigurator
	}
}

func WithConnectionPool(connPool *pool.ConnPool) ConsulOption {
	return func(opt *consulOptions) {
		opt.connPool = connPool
	}
}

func WithTokenStore(tokens *token.Store) ConsulOption {
	return func(opt *consulOptions) {
		opt.tokens = tokens
	}
}

func flattenConsulOptions(options []ConsulOption) consulOptions {
	var flat consulOptions
	for _, opt := range options {
		opt(&flat)
	}
	return flat
}

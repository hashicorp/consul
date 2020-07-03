package consul

import (
	"io"
	"os"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
)

// consulOptions are dependencies provided to a Client or Server constructor.
// A consulOptions struct is populated by Ops, and read by the constructor.
type consulOptions struct {
	logOutput       io.Writer
	logger          hclog.InterceptLogger
	tlsConfigurator *tlsutil.Configurator
	connPool        *pool.ConnPool
	tokens          *token.Store
}

// Op is a function (an operation) which updates a consulOptions. Ops are
// applied using applyOps.
type Op func(*consulOptions)

func WithLogger(logger hclog.InterceptLogger) Op {
	return func(opt *consulOptions) {
		opt.logger = logger
	}
}

func WithTLSConfigurator(tlsConfigurator *tlsutil.Configurator) Op {
	return func(opt *consulOptions) {
		opt.tlsConfigurator = tlsConfigurator
	}
}

func WithConnectionPool(connPool *pool.ConnPool) Op {
	return func(opt *consulOptions) {
		opt.connPool = connPool
	}
}

func WithTokenStore(tokens *token.Store) Op {
	return func(opt *consulOptions) {
		opt.tokens = tokens
	}
}

// applyOps to construct a consulOptions, then apply default values, and return
// the consulOptions.
func applyOps(options []Op) consulOptions {
	var flat consulOptions
	for _, opt := range options {
		opt(&flat)
	}

	if flat.logOutput == nil {
		flat.logOutput = os.Stderr
	}

	if flat.logger == nil {
		flat.logger = hclog.NewInterceptLogger(&hclog.LoggerOptions{
			Level:  hclog.Debug,
			Output: flat.logOutput,
		})
	}

	if flat.tokens == nil {
		flat.tokens = new(token.Store)
	}

	return flat
}

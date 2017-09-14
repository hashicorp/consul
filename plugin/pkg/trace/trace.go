package trace

import (
	"github.com/coredns/coredns/plugin"
	ot "github.com/opentracing/opentracing-go"
)

// Trace holds the tracer and endpoint info
type Trace interface {
	plugin.Handler
	Tracer() ot.Tracer
}

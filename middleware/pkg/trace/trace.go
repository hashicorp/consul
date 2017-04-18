package trace

import (
	"github.com/coredns/coredns/middleware"
	ot "github.com/opentracing/opentracing-go"
)

// Trace holds the tracer and endpoint info
type Trace interface {
	middleware.Handler
	Tracer() ot.Tracer
}

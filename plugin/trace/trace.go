// Package trace implements OpenTracing-based tracing
package trace

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/coredns/coredns/plugin"
	// Plugin the trace package.
	_ "github.com/coredns/coredns/plugin/pkg/trace"

	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
	"golang.org/x/net/context"
)

type trace struct {
	Next            plugin.Handler
	ServiceEndpoint string
	Endpoint        string
	EndpointType    string
	tracer          ot.Tracer
	serviceName     string
	clientServer    bool
	every           uint64
	count           uint64
	Once            sync.Once
}

func (t *trace) Tracer() ot.Tracer {
	return t.tracer
}

// OnStartup sets up the tracer
func (t *trace) OnStartup() error {
	var err error
	t.Once.Do(func() {
		switch t.EndpointType {
		case "zipkin":
			err = t.setupZipkin()
		default:
			err = fmt.Errorf("unknown endpoint type: %s", t.EndpointType)
		}
	})
	return err
}

func (t *trace) setupZipkin() error {

	collector, err := zipkin.NewHTTPCollector(t.Endpoint)
	if err != nil {
		return err
	}

	recorder := zipkin.NewRecorder(collector, false, t.ServiceEndpoint, t.serviceName)
	t.tracer, err = zipkin.NewTracer(recorder, zipkin.ClientServerSameSpan(t.clientServer))

	return err
}

// Name implements the Handler interface.
func (t *trace) Name() string {
	return "trace"
}

// ServeDNS implements the plugin.Handle interface.
func (t *trace) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	trace := false
	if t.every > 0 {
		queryNr := atomic.AddUint64(&t.count, 1)

		if queryNr%t.every == 0 {
			trace = true
		}
	}
	if span := ot.SpanFromContext(ctx); span == nil && trace {
		span := t.Tracer().StartSpan("servedns")
		defer span.Finish()
		ctx = ot.ContextWithSpan(ctx, span)
	}
	return plugin.NextOrFailure(t.Name(), t.Next, ctx, w, r)
}

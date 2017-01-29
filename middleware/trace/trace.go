// Package trace implements OpenTracing-based tracing
package trace

import (
	"fmt"
	"sync"

	"golang.org/x/net/context"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
)

// Trace holds the tracer and endpoint info
type Trace struct {
	Next            middleware.Handler
	ServiceEndpoint string
	Endpoint        string
	EndpointType    string
	Tracer          ot.Tracer
	Once            sync.Once
}

// OnStartup sets up the tracer
func (t *Trace) OnStartup() error {
	var err error
	t.Once.Do(func() {
		switch t.EndpointType {
		case "zipkin":
			err = t.setupZipkin()
		default:
			err = fmt.Errorf("Unknown endpoint type: %s", t.EndpointType)
		}
	})
	return err
}

func (t *Trace) setupZipkin() error {

	collector, err := zipkin.NewHTTPCollector(t.Endpoint)
	if err != nil {
		return err
	}

	recorder := zipkin.NewRecorder(collector, false, t.ServiceEndpoint, "coredns")
	t.Tracer, err = zipkin.NewTracer(recorder, zipkin.ClientServerSameSpan(false))
	if err != nil {
		return err
	}
	return nil
}

// Name implements the Handler interface.
func (t *Trace) Name() string {
	return "trace"
}

// ServeDNS implements the middleware.Handle interface.
func (t *Trace) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	span := t.Tracer.StartSpan("servedns")
	defer span.Finish()
	ctx = ot.ContextWithSpan(ctx, span)
	return middleware.NextOrFailure(t.Name(), t.Next, ctx, w, r)
}

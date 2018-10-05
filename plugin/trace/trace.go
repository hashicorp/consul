// Package trace implements OpenTracing-based tracing
package trace

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/rcode"
	// Plugin the trace package.
	_ "github.com/coredns/coredns/plugin/pkg/trace"
	"github.com/coredns/coredns/request"

	ddtrace "github.com/DataDog/dd-trace-go/opentracing"
	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	zipkin "github.com/openzipkin/zipkin-go-opentracing"
)

const (
	tagName  = "coredns.io/name"
	tagType  = "coredns.io/type"
	tagRcode = "coredns.io/rcode"
)

type trace struct {
	Next            plugin.Handler
	Endpoint        string
	EndpointType    string
	tracer          ot.Tracer
	serviceEndpoint string
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
		case "datadog":
			err = t.setupDatadog()
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

	recorder := zipkin.NewRecorder(collector, false, t.serviceEndpoint, t.serviceName)
	t.tracer, err = zipkin.NewTracer(recorder, zipkin.ClientServerSameSpan(t.clientServer))

	return err
}

func (t *trace) setupDatadog() error {
	config := ddtrace.NewConfiguration()
	config.ServiceName = t.serviceName

	host := strings.Split(t.Endpoint, ":")
	config.AgentHostname = host[0]

	if len(host) == 2 {
		config.AgentPort = host[1]
	}

	tracer, _, err := ddtrace.NewTracer(config)
	t.tracer = tracer
	return err
}

// Name implements the Handler interface.
func (t *trace) Name() string { return "trace" }

// ServeDNS implements the plugin.Handle interface.
func (t *trace) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	trace := false
	if t.every > 0 {
		queryNr := atomic.AddUint64(&t.count, 1)

		if queryNr%t.every == 0 {
			trace = true
		}
	}
	span := ot.SpanFromContext(ctx)
	if !trace || span != nil {
		return plugin.NextOrFailure(t.Name(), t.Next, ctx, w, r)
	}

	req := request.Request{W: w, Req: r}
	span = t.Tracer().StartSpan(spanName(ctx, req))
	defer span.Finish()

	rw := dnstest.NewRecorder(w)
	ctx = ot.ContextWithSpan(ctx, span)
	status, err := plugin.NextOrFailure(t.Name(), t.Next, ctx, rw, r)

	span.SetTag(tagName, req.Name())
	span.SetTag(tagType, req.Type())
	span.SetTag(tagRcode, rcode.ToString(rw.Rcode))

	return status, err
}

func spanName(ctx context.Context, req request.Request) string {
	return "servedns:" + metrics.WithServer(ctx) + " " + req.Name()
}

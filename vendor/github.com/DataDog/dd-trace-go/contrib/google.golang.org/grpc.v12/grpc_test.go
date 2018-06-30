package grpc

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"google.golang.org/grpc"

	context "golang.org/x/net/context"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/stretchr/testify/assert"
)

const (
	debug = false
)

func TestClient(t *testing.T) {
	assert := assert.New(t)

	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	rig, err := newRig(testTracer, true)
	if err != nil {
		t.Fatalf("error setting up rig: %s", err)
	}
	defer rig.Close()
	client := rig.client

	span := testTracer.NewRootSpan("a", "b", "c")
	ctx := tracer.ContextWithSpan(context.Background(), span)
	resp, err := client.Ping(ctx, &FixtureRequest{Name: "pass"})
	assert.Nil(err)
	span.Finish()
	assert.Equal(resp.Message, "passed")

	testTracer.ForceFlush()
	traces := testTransport.Traces()

	// A word here about what is going on: this is technically a
	// distributed trace, while we're in this example in the Go world
	// and within the same exec, client could know about server details.
	// But this is not the general cases. So, as we only connect client
	// and server through their span IDs, they can be flushed as independant
	// traces. They could also be flushed at once, this is an implementation
	// detail, what is important is that all of it is flushed, at some point.
	if len(traces) == 0 {
		assert.Fail("there should be at least one trace")
	}
	var spans []*tracer.Span
	for _, trace := range traces {
		for _, span := range trace {
			spans = append(spans, span)
		}
	}
	assert.Len(spans, 3)

	var sspan, cspan, tspan *tracer.Span

	for _, s := range spans {
		// order of traces in buffer is not garanteed
		switch s.Name {
		case "grpc.server":
			sspan = s
		case "grpc.client":
			cspan = s
		case "a":
			tspan = s
		}
	}

	assert.NotNil(sspan, "there should be a span with 'grpc.server' as Name")

	assert.NotNil(cspan, "there should be a span with 'grpc.client' as Name")
	assert.Equal(cspan.GetMeta("grpc.code"), "OK")

	assert.NotNil(tspan, "there should be a span with 'a' as Name")
	assert.Equal(cspan.TraceID, tspan.TraceID)
	assert.Equal(sspan.TraceID, tspan.TraceID)
}

func TestDisabled(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)
	testTracer.SetEnabled(false)

	rig, err := newRig(testTracer, true)
	if err != nil {
		t.Fatalf("error setting up rig: %s", err)
	}
	defer rig.Close()

	client := rig.client
	resp, err := client.Ping(context.Background(), &FixtureRequest{Name: "disabled"})
	assert.Nil(err)
	assert.Equal(resp.Message, "disabled")
	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Nil(traces)
}

func TestChild(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	rig, err := newRig(testTracer, false)
	if err != nil {
		t.Fatalf("error setting up rig: %s", err)
	}
	defer rig.Close()

	client := rig.client
	resp, err := client.Ping(context.Background(), &FixtureRequest{Name: "child"})
	assert.Nil(err)
	assert.Equal(resp.Message, "child")
	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 2)

	var sspan, cspan *tracer.Span

	for _, s := range spans {
		// order of traces in buffer is not garanteed
		switch s.Name {
		case "grpc.server":
			sspan = s
		case "child":
			cspan = s
		}
	}

	assert.NotNil(cspan, "there should be a span with 'child' as Name")
	assert.Equal(cspan.Error, int32(0))
	assert.Equal(cspan.Service, "grpc")
	assert.Equal(cspan.Resource, "child")
	assert.True(cspan.Duration > 0)

	assert.NotNil(sspan, "there should be a span with 'grpc.server' as Name")
	assert.Equal(sspan.Error, int32(0))
	assert.Equal(sspan.Service, "grpc")
	assert.Equal(sspan.Resource, "/grpc.Fixture/Ping")
	assert.True(sspan.Duration > 0)
}

func TestPass(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	rig, err := newRig(testTracer, false)
	if err != nil {
		t.Fatalf("error setting up rig: %s", err)
	}
	defer rig.Close()

	client := rig.client
	resp, err := client.Ping(context.Background(), &FixtureRequest{Name: "pass"})
	assert.Nil(err)
	assert.Equal(resp.Message, "passed")
	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)

	s := spans[0]
	assert.Equal(s.Error, int32(0))
	assert.Equal(s.Name, "grpc.server")
	assert.Equal(s.Service, "grpc")
	assert.Equal(s.Resource, "/grpc.Fixture/Ping")
	assert.Equal(s.Type, "go")
	assert.True(s.Duration > 0)
}

// fixtureServer a dummy implemenation of our grpc fixtureServer.
type fixtureServer struct{}

func newFixtureServer() *fixtureServer {
	return &fixtureServer{}
}

func (s *fixtureServer) Ping(ctx context.Context, in *FixtureRequest) (*FixtureReply, error) {
	switch {
	case in.Name == "child":
		span, ok := tracer.SpanFromContext(ctx)
		if ok {
			t := span.Tracer()
			t.NewChildSpan("child", span).Finish()
		}
		return &FixtureReply{Message: "child"}, nil
	case in.Name == "disabled":
		_, ok := tracer.SpanFromContext(ctx)
		if ok {
			panic("should be disabled")
		}
		return &FixtureReply{Message: "disabled"}, nil
	}

	return &FixtureReply{Message: "passed"}, nil
}

// ensure it's a fixtureServer
var _ FixtureServer = &fixtureServer{}

// rig contains all of the servers and connections we'd need for a
// grpc integration test
type rig struct {
	server   *grpc.Server
	listener net.Listener
	conn     *grpc.ClientConn
	client   FixtureClient
}

func (r *rig) Close() {
	r.server.Stop()
	r.conn.Close()
	r.listener.Close()
}

func newRig(t *tracer.Tracer, traceClient bool) (*rig, error) {

	server := grpc.NewServer(grpc.UnaryInterceptor(UnaryServerInterceptor("grpc", t)))

	RegisterFixtureServer(server, newFixtureServer())

	li, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	// start our test fixtureServer.
	go server.Serve(li)

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	if traceClient {
		opts = append(opts, grpc.WithUnaryInterceptor(UnaryClientInterceptor("grpc", t)))
	}

	conn, err := grpc.Dial(li.Addr().String(), opts...)
	if err != nil {
		return nil, fmt.Errorf("error dialing: %s", err)
	}

	r := &rig{
		listener: li,
		server:   server,
		conn:     conn,
		client:   NewFixtureClient(conn),
	}

	return r, err
}

// getTestTracer returns a Tracer with a DummyTransport
func getTestTracer() (*tracer.Tracer, *dummyTransport) {
	transport := &dummyTransport{}
	tracer := tracer.NewTracerTransport(transport)
	return tracer, transport
}

// dummyTransport is a transport that just buffers spans and encoding
type dummyTransport struct {
	traces   [][]*tracer.Span
	services map[string]tracer.Service
}

func (t *dummyTransport) SendTraces(traces [][]*tracer.Span) (*http.Response, error) {
	t.traces = append(t.traces, traces...)
	return nil, nil
}

func (t *dummyTransport) SendServices(services map[string]tracer.Service) (*http.Response, error) {
	t.services = services
	return nil, nil
}

func (t *dummyTransport) Traces() [][]*tracer.Span {
	traces := t.traces
	t.traces = nil
	return traces
}

func (t *dummyTransport) SetHeader(key, value string) {}

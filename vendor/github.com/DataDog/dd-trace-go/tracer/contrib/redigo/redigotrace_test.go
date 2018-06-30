package redigotrace

import (
	"context"
	"fmt"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

const (
	debug = false
)

func TestClient(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	c, _ := TracedDial("my-service", testTracer, "tcp", "127.0.0.1:56379")
	c.Do("SET", 1, "truck")

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]

	assert.Len(spans, 1)
	span := spans[0]
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.Service, "my-service")
	assert.Equal(span.Resource, "SET")
	assert.Equal(span.GetMeta("out.host"), "127.0.0.1")
	assert.Equal(span.GetMeta("out.port"), "56379")
	assert.Equal(span.GetMeta("redis.raw_command"), "SET 1 truck")
	assert.Equal(span.GetMeta("redis.args_length"), "2")
}

func TestCommandError(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	c, _ := TracedDial("my-service", testTracer, "tcp", "127.0.0.1:56379")
	_, err := c.Do("NOT_A_COMMAND", context.Background())

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	span := spans[0]

	assert.Equal(int32(span.Error), int32(1))
	assert.Equal(span.GetMeta("error.msg"), err.Error())
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.Service, "my-service")
	assert.Equal(span.Resource, "NOT_A_COMMAND")
	assert.Equal(span.GetMeta("out.host"), "127.0.0.1")
	assert.Equal(span.GetMeta("out.port"), "56379")
	assert.Equal(span.GetMeta("redis.raw_command"), "NOT_A_COMMAND")
}

func TestConnectionError(t *testing.T) {
	assert := assert.New(t)
	testTracer, _ := getTestTracer()
	testTracer.SetDebugLogging(debug)

	_, err := TracedDial("redis-service", testTracer, "tcp", "127.0.0.1:1000")

	assert.Contains(err.Error(), "dial tcp 127.0.0.1:1000")
}

func TestInheritance(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	// Parent span
	ctx := context.Background()
	parent_span := testTracer.NewChildSpanFromContext("parent_span", ctx)
	ctx = tracer.ContextWithSpan(ctx, parent_span)
	client, _ := TracedDial("my_service", testTracer, "tcp", "127.0.0.1:56379")
	client.Do("SET", "water", "bottle", ctx)
	parent_span.Finish()

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 2)

	var child_span, pspan *tracer.Span
	for _, s := range spans {
		// order of traces in buffer is not garanteed
		switch s.Name {
		case "redis.command":
			child_span = s
		case "parent_span":
			pspan = s
		}
	}
	assert.NotNil(child_span, "there should be a child redis.command span")
	assert.NotNil(child_span, "there should be a parent span")

	assert.Equal(child_span.ParentID, pspan.SpanID)
	assert.Equal(child_span.GetMeta("out.host"), "127.0.0.1")
	assert.Equal(child_span.GetMeta("out.port"), "56379")
}

func TestCommandsToSring(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	stringify_test := TestStruct{Cpython: 57, Cgo: 8}
	c, _ := TracedDial("my-service", testTracer, "tcp", "127.0.0.1:56379")
	c.Do("SADD", "testSet", "a", int(0), int32(1), int64(2), stringify_test, context.Background())

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	span := spans[0]

	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.Service, "my-service")
	assert.Equal(span.Resource, "SADD")
	assert.Equal(span.GetMeta("out.host"), "127.0.0.1")
	assert.Equal(span.GetMeta("out.port"), "56379")
	assert.Equal(span.GetMeta("redis.raw_command"), "SADD testSet a 0 1 2 [57, 8]")
}

func TestPool(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	pool := &redis.Pool{
		MaxIdle:     2,
		MaxActive:   3,
		IdleTimeout: 23,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			return TracedDial("my-service", testTracer, "tcp", "127.0.0.1:56379")
		},
	}

	pc := pool.Get()
	pc.Do("SET", " whiskey", " glass", context.Background())
	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	span := spans[0]
	assert.Equal(span.GetMeta("out.network"), "tcp")
}

func TestTracingDialUrl(t *testing.T) {
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)
	url := "redis://127.0.0.1:56379"
	client, _ := TracedDialURL("redis-service", testTracer, url)
	client.Do("SET", "ONE", " TWO", context.Background())

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
}

// TestStruct implements String interface
type TestStruct struct {
	Cpython int
	Cgo     int
}

func (ts TestStruct) String() string {
	return fmt.Sprintf("[%d, %d]", ts.Cpython, ts.Cgo)
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

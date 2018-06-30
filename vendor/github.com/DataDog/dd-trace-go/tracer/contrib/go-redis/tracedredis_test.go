package goredistrace

import (
	"context"
	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

const (
	debug = false
)

func TestClient(t *testing.T) {
	opts := &redis.Options{
		Addr:     "127.0.0.1:56379",
		Password: "", // no password set
		DB:       0,  // use default db
	}
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	client := NewTracedClient(opts, testTracer, "my-redis")
	client.Set("test_key", "test_value", 0)

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)

	span := spans[0]
	assert.Equal(span.Service, "my-redis")
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.GetMeta("out.host"), "127.0.0.1")
	assert.Equal(span.GetMeta("out.port"), "56379")
	assert.Equal(span.GetMeta("redis.raw_command"), "set test_key test_value: ")
	assert.Equal(span.GetMeta("redis.args_length"), "3")
}

func TestPipeline(t *testing.T) {
	opts := &redis.Options{
		Addr:     "127.0.0.1:56379",
		Password: "", // no password set
		DB:       0,  // use default db
	}
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	client := NewTracedClient(opts, testTracer, "my-redis")
	pipeline := client.Pipeline()
	pipeline.Expire("pipeline_counter", time.Hour)

	// Exec with context test
	pipeline.ExecWithContext(context.Background())

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)

	span := spans[0]
	assert.Equal(span.Service, "my-redis")
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.GetMeta("out.port"), "56379")
	assert.Equal(span.GetMeta("redis.pipeline_length"), "1")
	assert.Equal(span.Resource, "expire pipeline_counter 3600: false\n")

	pipeline.Expire("pipeline_counter", time.Hour)
	pipeline.Expire("pipeline_counter_1", time.Minute)

	// Rewriting Exec
	pipeline.Exec()

	testTracer.ForceFlush()
	traces = testTransport.Traces()
	assert.Len(traces, 1)
	spans = traces[0]
	assert.Len(spans, 1)

	span = spans[0]
	assert.Equal(span.Service, "my-redis")
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.GetMeta("redis.pipeline_length"), "2")
	assert.Equal(span.Resource, "expire pipeline_counter 3600: false\nexpire pipeline_counter_1 60: false\n")
}

func TestChildSpan(t *testing.T) {
	opts := &redis.Options{
		Addr:     "127.0.0.1:56379",
		Password: "", // no password set
		DB:       0,  // use default DB
	}
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	// Parent span
	ctx := context.Background()
	parent_span := testTracer.NewChildSpanFromContext("parent_span", ctx)
	ctx = tracer.ContextWithSpan(ctx, parent_span)

	client := NewTracedClient(opts, testTracer, "my-redis")
	client.SetContext(ctx)

	client.Set("test_key", "test_value", 0)
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

func TestMultipleCommands(t *testing.T) {
	opts := &redis.Options{
		Addr:     "127.0.0.1:56379",
		Password: "", // no password set
		DB:       0,  // use default DB
	}
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	client := NewTracedClient(opts, testTracer, "my-redis")
	client.Set("test_key", "test_value", 0)
	client.Get("test_key")
	client.Incr("int_key")
	client.ClientList()

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 4)
	spans := traces[0]
	assert.Len(spans, 1)

	// Checking all commands were recorded
	var commands [4]string
	for i := 0; i < 4; i++ {
		commands[i] = traces[i][0].GetMeta("redis.raw_command")
	}
	assert.Contains(commands, "set test_key test_value: ")
	assert.Contains(commands, "get test_key: ")
	assert.Contains(commands, "incr int_key: 0")
	assert.Contains(commands, "client list: ")
}

func TestError(t *testing.T) {
	opts := &redis.Options{
		Addr:     "127.0.0.1:56379",
		Password: "", // no password set
		DB:       0,  // use default DB
	}
	assert := assert.New(t)
	testTracer, testTransport := getTestTracer()
	testTracer.SetDebugLogging(debug)

	client := NewTracedClient(opts, testTracer, "my-redis")
	err := client.Get("non_existent_key")

	testTracer.ForceFlush()
	traces := testTransport.Traces()
	assert.Len(traces, 1)
	spans := traces[0]
	assert.Len(spans, 1)
	span := spans[0]

	assert.Equal(int32(span.Error), int32(1))
	assert.Equal(span.GetMeta("error.msg"), err.Err().Error())
	assert.Equal(span.Name, "redis.command")
	assert.Equal(span.GetMeta("out.host"), "127.0.0.1")
	assert.Equal(span.GetMeta("out.port"), "56379")
	assert.Equal(span.GetMeta("redis.raw_command"), "get non_existent_key: ")
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

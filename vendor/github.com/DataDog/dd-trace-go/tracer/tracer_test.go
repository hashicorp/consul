package tracer

import (
	"context"
	"fmt"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTracer(t *testing.T) {
	assert := assert.New(t)

	var wg sync.WaitGroup

	// the default client must be available
	assert.NotNil(DefaultTracer)

	// package free functions must proxy the calls to the
	// default client
	root := NewRootSpan("pylons.request", "pylons", "/")
	NewChildSpan("pylons.request", root)

	wg.Add(2)

	go func() {
		for i := 0; i < 1000; i++ {
			Disable()
			Enable()
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			_ = DefaultTracer.Enabled()
		}
		wg.Done()
	}()

	wg.Wait()
}

func TestNewSpan(t *testing.T) {
	assert := assert.New(t)

	// the tracer must create root spans
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")
	assert.Equal(uint64(0), span.ParentID)
	assert.Equal("pylons", span.Service)
	assert.Equal("pylons.request", span.Name)
	assert.Equal("/", span.Resource)
}

func TestNewSpanFromContextNil(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()

	child := tracer.NewChildSpanFromContext("abc", nil)
	assert.Equal("abc", child.Name)
	assert.Equal("", child.Service)

	child = tracer.NewChildSpanFromContext("def", context.Background())
	assert.Equal("def", child.Name)
	assert.Equal("", child.Service)

}

func TestNewChildSpanWithContext(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()

	// nil context
	span, ctx := tracer.NewChildSpanWithContext("abc", nil)
	assert.Equal("abc", span.Name)
	assert.Equal("", span.Service)
	assert.Equal(span.ParentID, span.SpanID) // it should be a root span
	assert.Equal(span.Tracer(), tracer)
	// the returned ctx should contain the created span
	assert.NotNil(ctx)
	ctxSpan, ok := SpanFromContext(ctx)
	assert.True(ok)
	assert.Equal(span, ctxSpan)

	// context without span
	span, ctx = tracer.NewChildSpanWithContext("abc", context.Background())
	assert.Equal("abc", span.Name)
	assert.Equal("", span.Service)
	assert.Equal(span.ParentID, span.SpanID) // it should be a root span
	// the returned ctx should contain the created span
	assert.NotNil(ctx)
	ctxSpan, ok = SpanFromContext(ctx)
	assert.True(ok)
	assert.Equal(span, ctxSpan)

	// context with span
	parent := tracer.NewRootSpan("pylons.request", "pylons", "/")
	parentCTX := ContextWithSpan(context.Background(), parent)
	span, ctx = tracer.NewChildSpanWithContext("def", parentCTX)
	assert.Equal("def", span.Name)
	assert.Equal("pylons", span.Service)
	assert.Equal(parent.Service, span.Service)
	// the created span should be a child of the parent span
	assert.Equal(span.ParentID, parent.SpanID)
	// the returned ctx should contain the created span
	assert.NotNil(ctx)
	ctxSpan, ok = SpanFromContext(ctx)
	assert.True(ok)
	assert.Equal(ctxSpan, span)
}

func TestNewSpanFromContext(t *testing.T) {
	assert := assert.New(t)

	// the tracer must create child spans
	tracer := NewTracer()
	parent := tracer.NewRootSpan("pylons.request", "pylons", "/")
	ctx := ContextWithSpan(context.Background(), parent)

	child := tracer.NewChildSpanFromContext("redis.command", ctx)
	// ids and services are inherited
	assert.Equal(parent.SpanID, child.ParentID)
	assert.Equal(parent.TraceID, child.TraceID)
	assert.Equal(parent.Service, child.Service)
	// the resource is not inherited and defaults to the name
	assert.Equal("redis.command", child.Resource)
	// the tracer instance is the same
	assert.Equal(tracer, parent.tracer)
	assert.Equal(tracer, child.tracer)

}

func TestNewSpanChild(t *testing.T) {
	assert := assert.New(t)

	// the tracer must create child spans
	tracer := NewTracer()
	parent := tracer.NewRootSpan("pylons.request", "pylons", "/")
	child := tracer.NewChildSpan("redis.command", parent)
	// ids and services are inherited
	assert.Equal(parent.SpanID, child.ParentID)
	assert.Equal(parent.TraceID, child.TraceID)
	assert.Equal(parent.Service, child.Service)
	// the resource is not inherited and defaults to the name
	assert.Equal("redis.command", child.Resource)
	// the tracer instance is the same
	assert.Equal(tracer, parent.tracer)
	assert.Equal(tracer, child.tracer)
}

func TestNewRootSpanHasPid(t *testing.T) {
	assert := assert.New(t)

	tracer := NewTracer()
	root := tracer.NewRootSpan("pylons.request", "pylons", "/")

	assert.Equal(strconv.Itoa(os.Getpid()), root.GetMeta(ext.Pid))
}

func TestNewChildHasNoPid(t *testing.T) {
	assert := assert.New(t)

	tracer := NewTracer()
	root := tracer.NewRootSpan("pylons.request", "pylons", "/")
	child := tracer.NewChildSpan("redis.command", root)

	assert.Equal("", child.GetMeta(ext.Pid))
}

func TestTracerDisabled(t *testing.T) {
	assert := assert.New(t)

	// disable the tracer and be sure that the span is not added
	tracer := NewTracer()
	tracer.SetEnabled(false)
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")
	span.Finish()
	assert.Len(tracer.channels.trace, 0)
}

func TestTracerEnabledAgain(t *testing.T) {
	assert := assert.New(t)

	// disable the tracer and enable it again
	tracer := NewTracer()
	tracer.SetEnabled(false)
	preSpan := tracer.NewRootSpan("pylons.request", "pylons", "/")
	preSpan.Finish()
	assert.Len(tracer.channels.trace, 0)
	tracer.SetEnabled(true)
	postSpan := tracer.NewRootSpan("pylons.request", "pylons", "/")
	postSpan.Finish()
	assert.Len(tracer.channels.trace, 1)
}

func TestTracerSampler(t *testing.T) {
	assert := assert.New(t)

	sampleRate := 0.5
	tracer := NewTracer()
	tracer.SetSampleRate(sampleRate)

	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// The span might be sampled or not, we don't know, but at least it should have the sample rate metric
	assert.Equal(sampleRate, span.Metrics[sampleRateMetricKey])
}

func TestTracerEdgeSampler(t *testing.T) {
	assert := assert.New(t)

	// a sample rate of 0 should sample nothing
	tracer0 := NewTracer()
	tracer0.SetSampleRate(0)
	// a sample rate of 1 should sample everything
	tracer1 := NewTracer()
	tracer1.SetSampleRate(1)

	count := traceChanLen / 3

	for i := 0; i < count; i++ {
		span0 := tracer0.NewRootSpan("pylons.request", "pylons", "/")
		span0.Finish()
		span1 := tracer1.NewRootSpan("pylons.request", "pylons", "/")
		span1.Finish()
	}

	assert.Len(tracer0.channels.trace, 0)
	assert.Len(tracer1.channels.trace, count)

	tracer0.Stop()
	tracer1.Stop()
}

func TestTracerConcurrent(t *testing.T) {
	assert := assert.New(t)
	tracer, transport := getTestTracer()
	defer tracer.Stop()

	// Wait for three different goroutines that should create
	// three different traces with one child each
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		tracer.NewRootSpan("pylons.request", "pylons", "/").Finish()
	}()
	go func() {
		defer wg.Done()
		tracer.NewRootSpan("pylons.request", "pylons", "/home").Finish()
	}()
	go func() {
		defer wg.Done()
		tracer.NewRootSpan("pylons.request", "pylons", "/trace").Finish()
	}()

	wg.Wait()
	tracer.ForceFlush()
	traces := transport.Traces()
	assert.Len(traces, 3)
	assert.Len(traces[0], 1)
	assert.Len(traces[1], 1)
	assert.Len(traces[2], 1)
}

func TestTracerParentFinishBeforeChild(t *testing.T) {
	assert := assert.New(t)
	tracer, transport := getTestTracer()
	defer tracer.Stop()

	// Testing an edge case: a child refers to a parent that is already closed.

	parent := tracer.NewRootSpan("pylons.request", "pylons", "/")
	parent.Finish()

	tracer.ForceFlush()
	traces := transport.Traces()
	assert.Len(traces, 1)
	assert.Len(traces[0], 1)
	assert.Equal(parent, traces[0][0])

	child := tracer.NewChildSpan("redis.command", parent)
	child.Finish()

	tracer.ForceFlush()

	traces = transport.Traces()
	assert.Len(traces, 1)
	assert.Len(traces[0], 1)
	assert.Equal(child, traces[0][0])
	assert.Equal(parent.SpanID, traces[0][0].ParentID, "child should refer to parent, even if they have been flushed separately")
}

func TestTracerConcurrentMultipleSpans(t *testing.T) {
	assert := assert.New(t)
	tracer, transport := getTestTracer()
	defer tracer.Stop()

	// Wait for two different goroutines that should create
	// two traces with two children each
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		parent := tracer.NewRootSpan("pylons.request", "pylons", "/")
		child := tracer.NewChildSpan("redis.command", parent)
		child.Finish()
		parent.Finish()
	}()
	go func() {
		defer wg.Done()
		parent := tracer.NewRootSpan("pylons.request", "pylons", "/")
		child := tracer.NewChildSpan("redis.command", parent)
		child.Finish()
		parent.Finish()
	}()

	wg.Wait()
	tracer.ForceFlush()
	traces := transport.Traces()
	assert.Len(traces, 2)
	assert.Len(traces[0], 2)
	assert.Len(traces[1], 2)
}

func TestTracerAtomicFlush(t *testing.T) {
	assert := assert.New(t)
	tracer, transport := getTestTracer()
	defer tracer.Stop()

	// Make sure we don't flush partial bits of traces
	root := tracer.NewRootSpan("pylons.request", "pylons", "/")
	span := tracer.NewChildSpan("redis.command", root)
	span1 := tracer.NewChildSpan("redis.command.1", span)
	span2 := tracer.NewChildSpan("redis.command.2", span)
	span.Finish()
	span1.Finish()
	span2.Finish()

	tracer.ForceFlush()
	traces := transport.Traces()
	assert.Len(traces, 0, "nothing should be flushed now as span2 is not finished yet")

	root.Finish()

	tracer.ForceFlush()
	traces = transport.Traces()
	assert.Len(traces, 1)
	assert.Len(traces[0], 4, "all spans should show up at once")
}

func TestTracerServices(t *testing.T) {
	assert := assert.New(t)
	tracer, transport := getTestTracer()

	tracer.SetServiceInfo("svc1", "a", "b")
	tracer.SetServiceInfo("svc2", "c", "d")
	tracer.SetServiceInfo("svc1", "e", "f")

	tracer.Stop()

	assert.Len(transport.services, 2)

	svc1 := transport.services["svc1"]
	assert.NotNil(svc1)
	assert.Equal("svc1", svc1.Name)
	assert.Equal("e", svc1.App)
	assert.Equal("f", svc1.AppType)

	svc2 := transport.services["svc2"]
	assert.NotNil(svc2)
	assert.Equal("svc2", svc2.Name)
	assert.Equal("c", svc2.App)
	assert.Equal("d", svc2.AppType)
}

func TestTracerServicesDisabled(t *testing.T) {
	assert := assert.New(t)
	tracer, transport := getTestTracer()

	tracer.SetEnabled(false)
	tracer.SetServiceInfo("svc1", "a", "b")
	tracer.Stop()

	assert.Len(transport.services, 0)
}

func TestTracerMeta(t *testing.T) {
	assert := assert.New(t)

	var nilTracer *Tracer
	nilTracer.SetMeta("key", "value")
	assert.Nil(nilTracer.getAllMeta(), "nil tracer should return nil meta")

	tracer, _ := getTestTracer()
	defer tracer.Stop()

	assert.Nil(tracer.getAllMeta(), "by default, no meta")
	tracer.SetMeta("env", "staging")

	span := tracer.NewRootSpan("pylons.request", "pylons", "/")
	assert.Equal("staging", span.GetMeta("env"))
	assert.Equal("", span.GetMeta("component"))
	span.Finish()
	assert.Equal(map[string]string{"env": "staging"}, tracer.getAllMeta(), "there should be one meta")

	tracer.SetMeta("component", "core")
	span = tracer.NewRootSpan("pylons.request", "pylons", "/")
	assert.Equal("staging", span.GetMeta("env"))
	assert.Equal("core", span.GetMeta("component"))
	span.Finish()
	assert.Equal(map[string]string{"env": "staging", "component": "core"}, tracer.getAllMeta(), "there should be two entries")

	tracer.SetMeta("env", "prod")
	span = tracer.NewRootSpan("pylons.request", "pylons", "/")
	assert.Equal("prod", span.GetMeta("env"))
	assert.Equal("core", span.GetMeta("component"))
	span.SetMeta("env", "sandbox")
	assert.Equal("sandbox", span.GetMeta("env"))
	assert.Equal("core", span.GetMeta("component"))
	span.Finish()

	assert.Equal(map[string]string{"env": "prod", "component": "core"}, tracer.getAllMeta(), "key1 should have been updated")
}

func TestTracerRace(t *testing.T) {
	assert := assert.New(t)

	tracer, transport := getTestTracer()
	defer tracer.Stop()

	total := (traceChanLen / 3) / 10
	var wg sync.WaitGroup
	wg.Add(total)

	// Trying to be quite brutal here, firing lots of concurrent things, finishing in
	// different orders, and modifying spans after creation.
	for n := 0; n < total; n++ {
		i := n // keep local copy
		odd := ((i % 2) != 0)
		go func() {
			if i%11 == 0 {
				time.Sleep(time.Microsecond)
			}

			tracer.SetMeta("foo", "bar")

			parent := tracer.NewRootSpan("pylons.request", "pylons", "/")

			NewChildSpan("redis.command", parent).Finish()
			child := NewChildSpan("async.service", parent)

			if i%13 == 0 {
				time.Sleep(time.Microsecond)
			}

			if odd {
				parent.SetMeta("odd", "true")
				parent.SetMetric("oddity", 1)
				parent.Finish()
			} else {
				child.SetMeta("odd", "false")
				child.SetMetric("oddity", 0)
				child.Finish()
			}

			if i%17 == 0 {
				time.Sleep(time.Microsecond)
			}

			if odd {
				child.Resource = "HGETALL"
				child.SetMeta("odd", "false")
				child.SetMetric("oddity", 0)
			} else {
				parent.Resource = "/" + strconv.Itoa(i) + ".html"
				parent.SetMeta("odd", "true")
				parent.SetMetric("oddity", 1)
			}

			if i%19 == 0 {
				time.Sleep(time.Microsecond)
			}

			if odd {
				child.Finish()
			} else {
				parent.Finish()
			}

			wg.Done()
		}()
	}

	wg.Wait()

	tracer.ForceFlush()
	traces := transport.Traces()
	assert.Len(traces, total, "we should have exactly as many traces as expected")
	for _, trace := range traces {
		assert.Len(trace, 3, "each trace should have exactly 3 spans")
		var parent, child, redis *Span
		for _, span := range trace {
			assert.Equal("bar", span.GetMeta("foo"), "tracer meta should have been applied to all spans")
			switch span.Name {
			case "pylons.request":
				parent = span
			case "async.service":
				child = span
			case "redis.command":
				redis = span
			default:
				assert.Fail("unexpected span", span)
			}
		}
		assert.NotNil(parent)
		assert.NotNil(child)
		assert.NotNil(redis)

		assert.Equal(uint64(0), parent.ParentID)
		assert.Equal(parent.TraceID, parent.SpanID)

		assert.Equal(parent.TraceID, redis.TraceID)
		assert.Equal(parent.TraceID, child.TraceID)

		assert.Equal(parent.TraceID, redis.ParentID)
		assert.Equal(parent.TraceID, child.ParentID)
	}
}

// TestWorker is definitely a flaky test, as here we test that the worker
// background task actually does flush things. Most other tests are and should
// be using ForceFlush() to make sure things are really sent to transport.
// Here, we just wait until things show up, as we would do with a real program.
func TestWorker(t *testing.T) {
	assert := assert.New(t)

	tracer, transport := getTestTracer()
	defer tracer.Stop()

	n := traceChanLen * 10 // put more traces than the chan size, on purpose
	for i := 0; i < n; i++ {
		root := tracer.NewRootSpan("pylons.request", "pylons", "/")
		child := tracer.NewChildSpan("redis.command", root)
		child.Finish()
		root.Finish()
	}

	now := time.Now()
	count := 0
	for time.Now().Before(now.Add(time.Minute)) && count < traceChanLen {
		nbTraces := len(transport.Traces())
		if nbTraces > 0 {
			t.Logf("popped %d traces", nbTraces)
		}
		count += nbTraces
		time.Sleep(time.Millisecond)
	}
	// here we just check that we have "enough traces". In practice, lots of them
	// are dropped, it's another interesting side-effect of this test: it does
	// trigger error messages (which are repeated, so it aggregates them etc.)
	if count < traceChanLen {
		assert.Fail(fmt.Sprintf("timeout, not enough traces in buffer (%d/%d)", count, n))
	}
}

// BenchmarkConcurrentTracing tests the performance of spawning a lot of
// goroutines where each one creates a trace with a parent and a child.
func BenchmarkConcurrentTracing(b *testing.B) {
	tracer, _ := getTestTracer()
	defer tracer.Stop()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		go func() {
			parent := tracer.NewRootSpan("pylons.request", "pylons", "/")
			defer parent.Finish()

			for i := 0; i < 10; i++ {
				tracer.NewChildSpan("redis.command", parent).Finish()
			}
		}()
	}
}

// BenchmarkTracerAddSpans tests the performance of creating and finishing a root
// span. It should include the encoding overhead.
func BenchmarkTracerAddSpans(b *testing.B) {
	tracer, _ := getTestTracer()
	defer tracer.Stop()

	for n := 0; n < b.N; n++ {
		span := tracer.NewRootSpan("pylons.request", "pylons", "/")
		span.Finish()
	}
}

// getTestTracer returns a Tracer with a DummyTransport
func getTestTracer() (*Tracer, *dummyTransport) {
	transport := &dummyTransport{getEncoder: msgpackEncoderFactory}
	tracer := NewTracerTransport(transport)
	return tracer, transport
}

// Mock Transport with a real Encoder
type dummyTransport struct {
	getEncoder encoderFactory
	traces     [][]*Span
	services   map[string]Service

	sync.RWMutex // required because of some poll-testing (eg: worker)
}

func (t *dummyTransport) SendTraces(traces [][]*Span) (*http.Response, error) {
	t.Lock()
	t.traces = append(t.traces, traces...)
	t.Unlock()

	encoder := t.getEncoder()
	return nil, encoder.EncodeTraces(traces)
}

func (t *dummyTransport) SendServices(services map[string]Service) (*http.Response, error) {
	t.Lock()
	t.services = services
	t.Unlock()

	encoder := t.getEncoder()
	return nil, encoder.EncodeServices(services)
}

func (t *dummyTransport) Traces() [][]*Span {
	t.Lock()
	defer t.Unlock()

	traces := t.traces
	t.traces = nil
	return traces
}

func (t *dummyTransport) SetHeader(key, value string) {}

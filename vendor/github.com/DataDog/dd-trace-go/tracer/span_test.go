package tracer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/dd-trace-go/tracer/ext"
)

func TestSpanStart(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// a new span sets the Start after the initialization
	assert.NotEqual(int64(0), span.Start)
}

func TestSpanString(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")
	// don't bother checking the contents, just make sure it works.
	assert.NotEqual("", span.String())
	span.Finish()
	assert.NotEqual("", span.String())
}

func TestSpanSetMeta(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// check the map is properly initialized
	span.SetMeta("status.code", "200")
	assert.Equal("200", span.Meta["status.code"])

	// operating on a finished span is a no-op
	nMeta := len(span.Meta)
	span.Finish()
	span.SetMeta("finished.test", "true")
	assert.Equal(len(span.Meta), nMeta)
	assert.Equal(span.Meta["finished.test"], "")
}

func TestSpanSetMetas(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")
	span.SetSamplingPriority(0) // avoid interferences with "_sampling_priority_v1" meta
	metas := map[string]string{
		"error.msg":   "Something wrong",
		"error.type":  "*errors.errorString",
		"status.code": "200",
		"system.pid":  "29176",
	}
	extraMetas := map[string]string{
		"custom.1": "something custom",
		"custom.2": "something even more special",
	}
	nopMetas := map[string]string{
		"nopKey1": "nopValue1",
		"nopKey2": "nopValue2",
	}

	// check the map is properly initialized
	span.SetMetas(metas)
	assert.Equal(len(metas), len(span.Meta))
	for k := range metas {
		assert.Equal(metas[k], span.Meta[k])
	}

	// check a second call adds the new metas, but does not remove old ones
	span.SetMetas(extraMetas)
	assert.Equal(len(metas)+len(extraMetas), len(span.Meta))
	for k := range extraMetas {
		assert.Equal(extraMetas[k], span.Meta[k])
	}

	assert.Equal(span.Meta["status.code"], "200")

	// operating on a finished span is a no-op
	span.Finish()
	span.SetMetas(nopMetas)
	assert.Equal(len(metas)+len(extraMetas), len(span.Meta))
	for k := range nopMetas {
		assert.Equal("", span.Meta[k])
	}

}

func TestSpanSetMetric(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// check the map is properly initialized
	span.SetMetric("bytes", 1024.42)
	assert.Equal(1, len(span.Metrics))
	assert.Equal(1024.42, span.Metrics["bytes"])

	// operating on a finished span is a no-op
	span.Finish()
	span.SetMetric("finished.test", 1337)
	assert.Equal(1, len(span.Metrics))
	assert.Equal(0.0, span.Metrics["finished.test"])
}

func TestSpanError(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// check the error is set in the default meta
	err := errors.New("Something wrong")
	span.SetError(err)
	assert.Equal(int32(1), span.Error)
	assert.Equal("Something wrong", span.Meta["error.msg"])
	assert.Equal("*errors.errorString", span.Meta["error.type"])
	assert.NotEqual("", span.Meta["error.stack"])

	// operating on a finished span is a no-op
	span = tracer.NewRootSpan("flask.request", "flask", "/")
	nMeta := len(span.Meta)
	span.Finish()
	span.SetError(err)
	assert.Equal(int32(0), span.Error)
	assert.Equal(nMeta, len(span.Meta))
	assert.Equal("", span.Meta["error.msg"])
	assert.Equal("", span.Meta["error.type"])
	assert.Equal("", span.Meta["error.stack"])
}

func TestSpanError_Typed(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// check the error is set in the default meta
	err := &boomError{}
	span.SetError(err)
	assert.Equal(int32(1), span.Error)
	assert.Equal("boom", span.Meta["error.msg"])
	assert.Equal("*tracer.boomError", span.Meta["error.type"])
	assert.NotEqual("", span.Meta["error.stack"])
}

func TestEmptySpan(t *testing.T) {
	// ensure the empty span won't crash the app
	var span Span
	span.SetMeta("a", "b")
	span.SetError(nil)
	span.Finish()

	var s *Span
	s.SetMeta("a", "b")
	s.SetError(nil)
	s.Finish()
}

func TestSpanErrorNil(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// don't set the error if it's nil
	nMeta := len(span.Meta)
	span.SetError(nil)
	assert.Equal(int32(0), span.Error)
	assert.Equal(nMeta, len(span.Meta))
}

func TestSpanFinish(t *testing.T) {
	assert := assert.New(t)
	wait := time.Millisecond * 2
	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	// the finish should set finished and the duration
	time.Sleep(wait)
	span.Finish()
	assert.True(span.Duration > int64(wait))
	assert.True(span.finished)
}

func TestSpanFinishTwice(t *testing.T) {
	assert := assert.New(t)
	wait := time.Millisecond * 2

	tracer, _ := getTestTracer()
	defer tracer.Stop()

	assert.Len(tracer.channels.trace, 0)

	// the finish must be idempotent
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")
	time.Sleep(wait)
	span.Finish()
	assert.Len(tracer.channels.trace, 1)

	previousDuration := span.Duration
	time.Sleep(wait)
	span.Finish()
	assert.Equal(previousDuration, span.Duration)
	assert.Len(tracer.channels.trace, 1)
}

func TestSpanContext(t *testing.T) {
	ctx := context.Background()
	_, ok := SpanFromContext(ctx)
	assert.False(t, ok)

	tracer := NewTracer()
	span := tracer.NewRootSpan("pylons.request", "pylons", "/")

	ctx = span.Context(ctx)
	s2, ok := SpanFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, span.SpanID, s2.SpanID)

}

// Prior to a bug fix, this failed when running `go test -race`
func TestSpanModifyWhileFlushing(t *testing.T) {
	tracer, _ := getTestTracer()
	defer tracer.Stop()

	done := make(chan struct{})
	go func() {
		span := tracer.NewRootSpan("pylons.request", "pylons", "/")
		span.Finish()
		// It doesn't make much sense to update the span after it's been finished,
		// but an error in a user's code could lead to this.
		span.SetMeta("race_test", "true")
		span.SetMetric("race_test2", 133.7)
		span.SetMetrics("race_test3", 133.7)
		span.SetError(errors.New("t"))
		done <- struct{}{}
	}()

	run := true
	for run {
		select {
		case <-done:
			run = false
		default:
			tracer.flushTraces()
		}
	}
}

func TestSpanSamplingPriority(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()

	span := tracer.NewRootSpan("my.name", "my.service", "my.resource")
	assert.Equal(0.0, span.Metrics["_sampling_priority_v1"], "default sampling priority if undefined is 0")
	assert.False(span.HasSamplingPriority(), "by default, sampling priority is undefined")
	assert.Equal(0, span.GetSamplingPriority(), "default sampling priority for root spans is 0")

	childSpan := tracer.NewChildSpan("my.child", span)
	assert.Equal(span.Metrics["_sampling_priority_v1"], childSpan.Metrics["_sampling_priority_v1"])
	assert.Equal(span.HasSamplingPriority(), childSpan.HasSamplingPriority())
	assert.Equal(span.GetSamplingPriority(), childSpan.GetSamplingPriority())

	for _, priority := range []int{
		ext.PriorityUserReject,
		ext.PriorityAutoReject,
		ext.PriorityAutoKeep,
		ext.PriorityUserKeep,
		999, // not used yet, but we should allow it
	} {
		span.SetSamplingPriority(priority)
		assert.True(span.HasSamplingPriority())
		assert.Equal(priority, span.GetSamplingPriority())
		childSpan = tracer.NewChildSpan("my.child", span)
		assert.Equal(span.Metrics["_sampling_priority_v1"], childSpan.Metrics["_sampling_priority_v1"])
		assert.Equal(span.HasSamplingPriority(), childSpan.HasSamplingPriority())
		assert.Equal(span.GetSamplingPriority(), childSpan.GetSamplingPriority())
	}
}

type boomError struct{}

func (e *boomError) Error() string { return "boom" }

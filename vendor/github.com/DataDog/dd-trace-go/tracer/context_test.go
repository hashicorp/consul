package tracer

import (
	"testing"

	"context"

	"github.com/stretchr/testify/assert"
)

func TestContextWithSpanDefault(t *testing.T) {
	assert := assert.New(t)

	// create a new context with a span
	span := SpanFromContextDefault(nil)
	assert.NotNil(span)

	ctx := context.Background()
	assert.NotNil(SpanFromContextDefault(ctx))
}

func TestSpanFromContext(t *testing.T) {
	assert := assert.New(t)

	// create a new context with a span
	ctx := context.Background()
	tracer := NewTracer()
	expectedSpan := tracer.NewRootSpan("pylons.request", "pylons", "/")
	ctx = ContextWithSpan(ctx, expectedSpan)

	span, ok := SpanFromContext(ctx)
	assert.True(ok)
	assert.Equal(expectedSpan, span)
}

func TestSpanFromContextNil(t *testing.T) {
	assert := assert.New(t)

	// create a context without a span
	ctx := context.Background()

	span, ok := SpanFromContext(ctx)
	assert.False(ok)
	assert.Nil(span)

	span, ok = SpanFromContext(nil)
	assert.False(ok)
	assert.Nil(span)

}

func TestSpanMissingParent(t *testing.T) {
	assert := assert.New(t)
	tracer := NewTracer()

	// assuming we're in an inner function and we
	// forget the nil or ok checks
	ctx := context.Background()
	span, _ := SpanFromContext(ctx)

	// span is nil according to the API
	child := tracer.NewChildSpan("redis.command", span)
	child.Finish()

	// the child is finished but it's not recorded in
	// the tracer buffer because the service is missing
	assert.True(child.Duration > 0)
	assert.Equal(1, len(tracer.channels.trace))
}

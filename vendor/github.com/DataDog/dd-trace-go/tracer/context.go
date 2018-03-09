package tracer

import (
	"context"
)

var spanKey = "datadog_trace_span"

// ContextWithSpan will return a new context that includes the given span.
// DEPRECATED: use span.Context(ctx) instead.
func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	if span == nil {
		return ctx
	}
	return span.Context(ctx)
}

// SpanFromContext returns the stored *Span from the Context if it's available.
// This helper returns also the ok value that is true if the span is present.
func SpanFromContext(ctx context.Context) (*Span, bool) {
	if ctx == nil {
		return nil, false
	}
	span, ok := ctx.Value(spanKey).(*Span)
	return span, ok
}

// SpanFromContextDefault returns the stored *Span from the Context. If not, it
// will return an empty span that will do nothing.
func SpanFromContextDefault(ctx context.Context) *Span {

	// FIXME[matt] is it better to return a singleton empty span?
	if ctx == nil {
		return &Span{}
	}

	span, ok := SpanFromContext(ctx)
	if !ok {
		return &Span{}
	}
	return span
}

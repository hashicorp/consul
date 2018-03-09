package opentracing_test

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"
)

// You can leverage the Golang `Context` for intra-process propagation of
// Spans. In this example we create a root Span, so that it can be reused
// in a nested function to create a child Span.
func Example_startContext() {
	// create a new root Span and return a new Context that includes
	// the Span itself
	ctx := context.Background()
	rootSpan, ctx := opentracing.StartSpanFromContext(ctx, "web.request")
	defer rootSpan.Finish()

	requestHandler(ctx)
}

func requestHandler(ctx context.Context) {
	// retrieve the previously set root Span
	span := opentracing.SpanFromContext(ctx)
	span.SetTag("resource.name", "/")

	// or simply create a new child Span from the previous Context
	childSpan, _ := opentracing.StartSpanFromContext(ctx, "sql.query")
	defer childSpan.Finish()
}

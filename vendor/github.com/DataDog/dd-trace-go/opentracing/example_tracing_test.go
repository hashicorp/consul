package opentracing_test

import (
	opentracing "github.com/opentracing/opentracing-go"
)

// You can use the GlobalTracer to create a root Span. If you need to create a hierarchy,
// simply use the `ChildOf` reference
func Example_startSpan() {
	// use the GlobalTracer previously set
	rootSpan := opentracing.StartSpan("web.request")
	defer rootSpan.Finish()

	// set the reference to create a hierarchy of spans
	reference := opentracing.ChildOf(rootSpan.Context())
	childSpan := opentracing.StartSpan("sql.query", reference)
	defer childSpan.Finish()

	dbQuery()
}

func dbQuery() {
	// start a database query
}

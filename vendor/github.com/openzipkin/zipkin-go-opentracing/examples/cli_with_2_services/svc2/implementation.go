// +build go1.7

package svc2

import (
	"context"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// svc2 is our actual service implementation.
type svc2 struct{}

// NewService returns a new implementation of our Service.
func NewService() Service {
	return &svc2{}
}

// Sum implements our Service interface.
func (s *svc2) Sum(ctx context.Context, a int64, b int64) (int64, error) {
	// We love starting up slow
	time.Sleep(5 * time.Millisecond)

	// Pull span from context.
	span := opentracing.SpanFromContext(ctx)

	// Example binary annotations.
	span.SetTag("service", "svc2")
	span.SetTag("string", "some value")
	span.SetTag("int", 123)
	span.SetTag("bool", true)

	// Example annotation
	span.LogEvent("MyEventAnnotation")

	// Let's wait a little so it shows up nicely in our tracing graphics.
	time.Sleep(10 * time.Millisecond)

	// Let's assume we want to trace a call we do to a database.
	s.fakeDBCall(span)

	// Check for Int overflow condition.
	if (b > 0 && a > (Int64Max-b)) || (b < 0 && a < (Int64Min-b)) {
		span.SetTag("error", ErrIntOverflow.Error())
		return 0, ErrIntOverflow
	}

	// calculate and return the result (all that boilerplate for this?) ;)
	return a + b, nil
}

func (s *svc2) fakeDBCall(span opentracing.Span) {
	resourceSpan := opentracing.StartSpan(
		"myComplexQuery",
		opentracing.ChildOf(span.Context()),
	)
	defer resourceSpan.Finish()
	// mark span as resource type
	ext.SpanKind.Set(resourceSpan, "resource")
	// name of the resource we try to reach
	ext.PeerService.Set(resourceSpan, "PostgreSQL")
	// hostname of the resource
	ext.PeerHostname.Set(resourceSpan, "localhost")
	// port of the resource
	ext.PeerPort.Set(resourceSpan, 3306)
	// let's binary annotate the query we run
	resourceSpan.SetTag(
		"query", "SELECT recipes FROM cookbook WHERE topic = 'world domination'",
	)

	// Let's assume the query is going to take some time. Finding the right
	// world domination recipes is like searching for a needle in a haystack.
	time.Sleep(20 * time.Millisecond)

	// sweet... all done
}

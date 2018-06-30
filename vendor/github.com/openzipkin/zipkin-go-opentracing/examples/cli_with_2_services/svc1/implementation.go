// +build go1.7

package svc1

import (
	"context"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/openzipkin/zipkin-go-opentracing/examples/cli_with_2_services/svc2"
)

// svc1 is our actual service implementation
type svc1 struct {
	svc2Client svc2.Service
}

func (s *svc1) Concat(ctx context.Context, a, b string) (string, error) {
	// test for length overflow
	if len(a)+len(b) > StrMaxSize {
		// pull span from context (has already been created by our middleware)
		span := opentracing.SpanFromContext(ctx)
		span.SetTag("error", ErrMaxSize.Error())
		return "", ErrMaxSize
	}

	return a + b, nil
}

func (s *svc1) Sum(ctx context.Context, a, b int64) (int64, error) {
	// pull span from context (has already been created by our middleware)
	span := opentracing.SpanFromContext(ctx)
	span.SetTag("proxy-to", "svc2")

	// proxy request to svc2
	result, err := s.svc2Client.Sum(ctx, a, b)
	if err != nil {
		span.SetTag("error", err.Error())
		return 0, err
	}

	return result, nil
}

// NewService returns a new implementation of our Service.
func NewService(svc2Client svc2.Service) Service {
	return &svc1{
		svc2Client: svc2Client,
	}
}

package otgrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opentracing/opentracing-go/mocktracer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	firstCode = codes.OK
	lastCode  = codes.DataLoss
)

func TestSpanTags(t *testing.T) {
	tracer := mocktracer.New()
	for code := firstCode; code <= lastCode; code++ {
		// Client error
		tracer.Reset()
		span := tracer.StartSpan("test-trace-client")
		err := status.Error(code, "")
		SetSpanTags(span, err, true)
		span.Finish()

		// Assert added tags
		rawSpan := tracer.FinishedSpans()[0]
		expectedTags := map[string]interface{}{
			"response_code":  code,
			"response_class": ErrorClass(err),
		}
		if err != nil {
			expectedTags["error"] = true
		}
		assert.Equal(t, expectedTags, rawSpan.Tags())

		// Server error
		tracer.Reset()
		span = tracer.StartSpan("test-trace-server")
		err = status.Error(code, "")
		SetSpanTags(span, err, false)
		span.Finish()

		// Assert added tags
		rawSpan = tracer.FinishedSpans()[0]
		expectedTags = map[string]interface{}{
			"response_code":  code,
			"response_class": ErrorClass(err),
		}
		if err != nil && ErrorClass(err) == ServerError {
			expectedTags["error"] = true
		}
		assert.Equal(t, expectedTags, rawSpan.Tags())
	}
}

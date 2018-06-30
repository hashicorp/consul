package elastictrace

import (
	"testing"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/stretchr/testify/assert"
)

func TestQuantize(t *testing.T) {
	tr := tracer.NewTracer()
	for _, tc := range []struct {
		url, method string
		expected    string
	}{
		{
			url:      "/twitter/tweets",
			method:   "POST",
			expected: "POST /twitter/tweets",
		},
		{
			url:      "/logs_2016_05/event/_search",
			method:   "GET",
			expected: "GET /logs_?_?/event/_search",
		},
		{
			url:      "/twitter/tweets/123",
			method:   "GET",
			expected: "GET /twitter/tweets/?",
		},
		{
			url:      "/logs_2016_05/event/123",
			method:   "PUT",
			expected: "PUT /logs_?_?/event/?",
		},
	} {
		span := tracer.NewSpan("name", "elasticsearch", "", 0, 0, 0, tr)
		span.SetMeta("elasticsearch.url", tc.url)
		span.SetMeta("elasticsearch.method", tc.method)
		Quantize(span)
		assert.Equal(t, tc.expected, span.Resource)
	}
}

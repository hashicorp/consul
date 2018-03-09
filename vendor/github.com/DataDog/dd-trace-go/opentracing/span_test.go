package opentracing

import (
	"errors"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestSpanBaggage(t *testing.T) {
	assert := assert.New(t)

	span := NewSpan("web.request")
	span.SetBaggageItem("key", "value")
	assert.Equal("value", span.BaggageItem("key"))
}

func TestSpanContext(t *testing.T) {
	assert := assert.New(t)

	span := NewSpan("web.request")
	assert.NotNil(span.Context())
}

func TestSpanOperationName(t *testing.T) {
	assert := assert.New(t)

	span := NewSpan("web.request")
	span.SetOperationName("http.request")
	assert.Equal("http.request", span.Span.Name)
}

func TestSpanFinish(t *testing.T) {
	assert := assert.New(t)

	span := NewSpan("web.request")
	span.Finish()

	assert.True(span.Span.Duration > 0)
}

func TestSpanFinishWithTime(t *testing.T) {
	assert := assert.New(t)

	finishTime := time.Now().Add(10 * time.Second)
	span := NewSpan("web.request")
	span.FinishWithOptions(opentracing.FinishOptions{FinishTime: finishTime})

	duration := finishTime.UnixNano() - span.Span.Start
	assert.Equal(duration, span.Span.Duration)
}

func TestSpanSetTag(t *testing.T) {
	assert := assert.New(t)

	span := NewSpan("web.request")
	span.SetTag("component", "tracer")
	assert.Equal("tracer", span.Meta["component"])

	span.SetTag("tagInt", 1234)
	assert.Equal("1234", span.Meta["tagInt"])
}

func TestSpanSetDatadogTags(t *testing.T) {
	assert := assert.New(t)

	span := NewSpan("web.request")
	span.SetTag("span.type", "http")
	span.SetTag("service.name", "db-cluster")
	span.SetTag("resource.name", "SELECT * FROM users;")

	assert.Equal("http", span.Span.Type)
	assert.Equal("db-cluster", span.Span.Service)
	assert.Equal("SELECT * FROM users;", span.Span.Resource)
}

func TestSpanSetErrorTag(t *testing.T) {
	assert := assert.New(t)

	for _, tt := range []struct {
		name string      // span name
		val  interface{} // tag value
		msg  string      // error message
		typ  string      // error type
	}{
		{
			name: "error.error",
			val:  errors.New("some error"),
			msg:  "some error",
			typ:  "*errors.errorString",
		},
		{
			name: "error.string",
			val:  "some string error",
			msg:  "some string error",
			typ:  "*errors.errorString",
		},
		{
			name: "error.struct",
			val:  struct{ N int }{5},
			msg:  "{5}",
			typ:  "*errors.errorString",
		},
		{
			name: "error.other",
			val:  1,
			msg:  "1",
			typ:  "*errors.errorString",
		},
		{
			name: "error.nil",
			val:  nil,
			msg:  "",
			typ:  "",
		},
	} {
		span := NewSpan(tt.name)
		span.SetTag(Error, tt.val)

		assert.Equal(span.Meta["error.msg"], tt.msg)
		assert.Equal(span.Meta["error.type"], tt.typ)

		if tt.val != nil {
			assert.NotEqual(span.Meta["error.stack"], "")
		}
	}
}

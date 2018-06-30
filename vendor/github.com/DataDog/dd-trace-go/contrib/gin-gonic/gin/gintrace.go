// Package gin provides tracing middleware for the Gin web framework.
package gin

import (
	"fmt"
	"strconv"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/gin-gonic/gin"
)

// key is the string that we'll use to store spans in the tracer.
var key = "datadog_trace_span"

// Middleware returns middleware that will trace requests with the default
// tracer.
func Middleware(service string) gin.HandlerFunc {
	return MiddlewareTracer(service, tracer.DefaultTracer)
}

// MiddlewareTracer returns middleware that will trace requests with the given
// tracer.
func MiddlewareTracer(service string, t *tracer.Tracer) gin.HandlerFunc {
	t.SetServiceInfo(service, "gin-gonic", ext.AppTypeWeb)
	mw := newMiddleware(service, t)
	return mw.Handle
}

// middleware implements gin middleware.
type middleware struct {
	service string
	trc     *tracer.Tracer
}

func newMiddleware(service string, trc *tracer.Tracer) *middleware {
	return &middleware{
		service: service,
		trc:     trc,
	}
}

// Handle is a gin HandlerFunc that will add tracing to the given request.
func (m *middleware) Handle(c *gin.Context) {

	// bail if not enabled
	if !m.trc.Enabled() {
		c.Next()
		return
	}

	// FIXME[matt] the handler name is a bit unwieldy and uses reflection
	// under the hood. might be better to tackle this task and do it right
	// so we can end up with "user/:user/whatever" instead of
	// "github.com/foobar/blah"
	//
	// See here: https://github.com/gin-gonic/gin/issues/649
	resource := c.HandlerName()

	// Create our span and patch it to the context for downstream.
	span := m.trc.NewRootSpan("gin.request", m.service, resource)
	c.Set(key, span)

	// Pass along the request.
	c.Next()

	// Set http tags.
	span.SetMeta(ext.HTTPCode, strconv.Itoa(c.Writer.Status()))
	span.SetMeta(ext.HTTPMethod, c.Request.Method)
	span.SetMeta(ext.HTTPURL, c.Request.URL.Path)

	// Set any error information.
	var err error
	if len(c.Errors) > 0 {
		span.SetMeta("gin.errors", c.Errors.String()) // set all errors
		err = c.Errors[0]                             // but use the first for standard fields
	}
	span.FinishWithErr(err)
}

// Span returns the Span stored in the given Context and true. If it doesn't exist,
// it will returns (nil, false)
func Span(c *gin.Context) (*tracer.Span, bool) {
	if c == nil {
		return nil, false
	}

	s, ok := c.Get(key)
	if !ok {
		return nil, false
	}
	switch span := s.(type) {
	case *tracer.Span:
		return span, true
	}

	return nil, false
}

// SpanDefault returns the span stored in the given Context. If none exists,
// it will return an empty span.
func SpanDefault(c *gin.Context) *tracer.Span {
	span, ok := Span(c)
	if !ok {
		return &tracer.Span{}
	}
	return span
}

// NewChildSpan will create a span that is the child of the span stored in
// the context.
func NewChildSpan(name string, c *gin.Context) *tracer.Span {
	span, ok := Span(c)
	if !ok {
		return &tracer.Span{}
	}
	return span.Tracer().NewChildSpan(name, span)
}

// HTML will trace the rendering of the template as a child of the span in the
// given context.
func HTML(c *gin.Context, code int, name string, obj interface{}) {
	span, _ := Span(c)
	if span == nil {
		c.HTML(code, name, obj)
		return
	}

	child := span.Tracer().NewChildSpan("gin.render.html", span)
	child.SetMeta("go.template", name)
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("error rendering tmpl:%s: %s", name, r)
			child.FinishWithErr(err)
			panic(r)
		} else {
			child.Finish()
		}
	}()

	// render
	c.HTML(code, name, obj)
}

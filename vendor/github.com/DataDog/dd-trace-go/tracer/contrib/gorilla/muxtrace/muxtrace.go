// Package muxtrace provides tracing functions for the Gorilla Mux framework.
package muxtrace

import (
	"net/http"
	"strconv"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
	"github.com/gorilla/mux"
)

// MuxTracer is used to trace requests in a mux server.
type MuxTracer struct {
	tracer  *tracer.Tracer
	service string
}

// NewMuxTracer creates a MuxTracer for the given service and tracer.
func NewMuxTracer(service string, t *tracer.Tracer) *MuxTracer {
	t.SetServiceInfo(service, "gorilla", ext.AppTypeWeb)
	return &MuxTracer{
		tracer:  t,
		service: service,
	}
}

// TraceHandleFunc will return a HandlerFunc that will wrap tracing around the
// given handler func.
func (m *MuxTracer) TraceHandleFunc(handler http.HandlerFunc) http.HandlerFunc {

	return func(writer http.ResponseWriter, req *http.Request) {

		// bail our if tracing isn't enabled.
		if !m.tracer.Enabled() {
			handler(writer, req)
			return
		}

		// trace the request
		tracedRequest, span := m.trace(req)
		defer span.Finish()

		// trace the response
		tracedWriter := newTracedResponseWriter(span, writer)

		// run the request
		handler(tracedWriter, tracedRequest)
	}
}

// HandleFunc will add a traced version of the given handler to the router.
func (m *MuxTracer) HandleFunc(router *mux.Router, pattern string, handler http.HandlerFunc) *mux.Route {
	return router.HandleFunc(pattern, m.TraceHandleFunc(handler))
}

// span will create a span for the given request.
func (m *MuxTracer) trace(req *http.Request) (*http.Request, *tracer.Span) {
	route := mux.CurrentRoute(req)
	path, err := route.GetPathTemplate()
	if err != nil {
		// when route doesn't define a path
		path = "unknown"
	}

	resource := req.Method + " " + path

	span := m.tracer.NewRootSpan("mux.request", m.service, resource)
	span.Type = ext.HTTPType
	span.SetMeta(ext.HTTPMethod, req.Method)
	span.SetMeta(ext.HTTPURL, path)

	// patch the span onto the request context.
	treq := SetRequestSpan(req, span)
	return treq, span
}

// tracedResponseWriter is a small wrapper around an http response writer that will
// intercept and store the status of a request.
type tracedResponseWriter struct {
	span   *tracer.Span
	w      http.ResponseWriter
	status int
}

func newTracedResponseWriter(span *tracer.Span, w http.ResponseWriter) *tracedResponseWriter {
	return &tracedResponseWriter{
		span: span,
		w:    w}
}

func (t *tracedResponseWriter) Header() http.Header {
	return t.w.Header()
}

func (t *tracedResponseWriter) Write(b []byte) (int, error) {
	if t.status == 0 {
		t.WriteHeader(http.StatusOK)
	}
	return t.w.Write(b)
}

func (t *tracedResponseWriter) WriteHeader(status int) {
	t.w.WriteHeader(status)
	t.status = status
	t.span.SetMeta(ext.HTTPCode, strconv.Itoa(status))
	if status >= 500 && status < 600 {
		t.span.Error = 1
	}
}

// SetRequestSpan sets the span on the request's context.
func SetRequestSpan(r *http.Request, span *tracer.Span) *http.Request {
	if r == nil || span == nil {
		return r
	}

	ctx := tracer.ContextWithSpan(r.Context(), span)
	return r.WithContext(ctx)
}

// GetRequestSpan will return the span associated with the given request. It
// will return nil/false if it doesn't exist.
func GetRequestSpan(r *http.Request) (*tracer.Span, bool) {
	span, ok := tracer.SpanFromContext(r.Context())
	return span, ok
}

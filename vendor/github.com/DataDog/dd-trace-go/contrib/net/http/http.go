package http

import (
	"net/http"
	"strconv"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
)

// ServeMux is an HTTP request multiplexer that traces all the incoming requests.
type ServeMux struct {
	*http.ServeMux
	*tracer.Tracer
	service string
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux(service string, t *tracer.Tracer) *ServeMux {
	if t == nil {
		t = tracer.DefaultTracer
	}
	t.SetServiceInfo(service, "net/http", ext.AppTypeWeb)
	return &ServeMux{http.NewServeMux(), t, service}
}

// ServeHTTP dispatches the request to the handler whose
// pattern most closely matches the request URL.
// We only needed to rewrite this method to be able to trace the multiplexer.
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// bail out if tracing isn't enabled
	if !mux.Tracer.Enabled() {
		mux.ServeMux.ServeHTTP(w, r)
		return
	}

	// get the route associated to this request
	_, route := mux.Handler(r)

	// create a new span
	resource := r.Method + " " + route
	span := mux.Tracer.NewRootSpan("http.request", mux.service, resource)
	defer span.Finish()

	span.Type = ext.HTTPType
	span.SetMeta(ext.HTTPMethod, r.Method)
	span.SetMeta(ext.HTTPURL, r.URL.Path)

	// pass the span through the request context
	ctx := span.Context(r.Context())
	traceRequest := r.WithContext(ctx)

	// trace the response to get the status code
	traceWriter := NewResponseWriter(w, span)

	// serve the request to the underlying multiplexer
	mux.ServeMux.ServeHTTP(traceWriter, traceRequest)
}

// ResponseWriter is a small wrapper around an http response writer that will
// intercept and store the status of a request.
// It implements the ResponseWriter interface.
type ResponseWriter struct {
	http.ResponseWriter
	span   *tracer.Span
	status int
}

// New ResponseWriter allocateds and returns a new ResponseWriter.
func NewResponseWriter(w http.ResponseWriter, span *tracer.Span) *ResponseWriter {
	return &ResponseWriter{w, span, 0}
}

// Write writes the data to the connection as part of an HTTP reply.
// We explicitely call WriteHeader with the 200 status code
// in order to get it reported into the span.
func (w *ResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader sends an HTTP response header with status code.
// It also sets the status code to the span.
func (w *ResponseWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	w.status = status
	w.span.SetMeta(ext.HTTPCode, strconv.Itoa(status))
	if status >= 500 && status < 600 {
		w.span.Error = 1
	}
}

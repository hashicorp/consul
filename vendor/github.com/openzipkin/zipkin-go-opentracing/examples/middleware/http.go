// +build go1.7

// Package middleware provides some usable transport middleware to deal with
// propagating Zipkin traces across service boundaries.
package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	"github.com/openzipkin/zipkin-go-opentracing/thrift/gen-go/zipkincore"
)

// RequestFunc is a middleware function for outgoing HTTP requests.
type RequestFunc func(req *http.Request) *http.Request

// ToHTTPRequest returns a RequestFunc that injects an OpenTracing Span found in
// context into the HTTP Headers. If no such Span can be found, the RequestFunc
// is a noop.
func ToHTTPRequest(tracer opentracing.Tracer) RequestFunc {
	return func(req *http.Request) *http.Request {
		// Retrieve the Span from context.
		if span := opentracing.SpanFromContext(req.Context()); span != nil {

			// We are going to use this span in a client request, so mark as such.
			ext.SpanKindRPCClient.Set(span)

			// Add some standard OpenTracing tags, useful in an HTTP request.
			ext.HTTPMethod.Set(span, req.Method)
			span.SetTag(zipkincore.HTTP_HOST, req.URL.Host)
			span.SetTag(zipkincore.HTTP_PATH, req.URL.Path)
			ext.HTTPUrl.Set(
				span,
				fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.Path),
			)

			// Add information on the peer service we're about to contact.
			if host, portString, err := net.SplitHostPort(req.URL.Host); err == nil {
				ext.PeerHostname.Set(span, host)
				if port, err := strconv.Atoi(portString); err != nil {
					ext.PeerPort.Set(span, uint16(port))
				}
			} else {
				ext.PeerHostname.Set(span, req.URL.Host)
			}

			// Inject the Span context into the outgoing HTTP Request.
			if err := tracer.Inject(
				span.Context(),
				opentracing.TextMap,
				opentracing.HTTPHeadersCarrier(req.Header),
			); err != nil {
				fmt.Printf("error encountered while trying to inject span: %+v\n", err)
			}
		}
		return req
	}
}

// HandlerFunc is a middleware function for incoming HTTP requests.
type HandlerFunc func(next http.Handler) http.Handler

// FromHTTPRequest returns a Middleware HandlerFunc that tries to join with an
// OpenTracing trace found in the HTTP request headers and starts a new Span
// called `operationName`. If no trace could be found in the HTTP request
// headers, the Span will be a trace root. The Span is incorporated in the
// HTTP Context object and can be retrieved with
// opentracing.SpanFromContext(ctx).
func FromHTTPRequest(tracer opentracing.Tracer, operationName string,
) HandlerFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Try to join to a trace propagated in `req`.
			wireContext, err := tracer.Extract(
				opentracing.TextMap,
				opentracing.HTTPHeadersCarrier(req.Header),
			)
			if err != nil {
				fmt.Printf("error encountered while trying to extract span: %+v\n", err)
			}

			// create span
			span := tracer.StartSpan(operationName, ext.RPCServerOption(wireContext))
			defer span.Finish()

			// store span in context
			ctx := opentracing.ContextWithSpan(req.Context(), span)

			// update request context to include our new span
			req = req.WithContext(ctx)

			// next middleware or actual request handler
			next.ServeHTTP(w, req)
		})
	}
}

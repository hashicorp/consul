// Package elastictrace provides tracing for the Elastic Elasticsearch client.
// Supports v3 (gopkg.in/olivere/elastic.v3), v5 (gopkg.in/olivere/elastic.v5)
// but with v3 you must use `DoC` on all requests to capture the request context.
package elastictrace

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/DataDog/dd-trace-go/tracer"
	"github.com/DataDog/dd-trace-go/tracer/ext"
)

// MaxContentLength is the maximum content length for which we'll read and capture
// the contents of the request body. Anything larger will still be traced but the
// body will not be captured as trace metadata.
const MaxContentLength = 500 * 1024

// TracedTransport is a traced HTTP transport that captures Elasticsearch spans.
type TracedTransport struct {
	service string
	tracer  *tracer.Tracer
	*http.Transport
}

// RoundTrip satisfies the RoundTripper interface, wraps the sub Transport and
// captures a span of the Elasticsearch request.
func (t *TracedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	span := t.tracer.NewChildSpanFromContext("elasticsearch.query", req.Context())
	span.Service = t.service
	span.Type = ext.AppTypeDB
	defer span.Finish()
	span.SetMeta("elasticsearch.method", req.Method)
	span.SetMeta("elasticsearch.url", req.URL.Path)
	span.SetMeta("elasticsearch.params", req.URL.Query().Encode())

	contentLength, _ := strconv.Atoi(req.Header.Get("Content-Length"))
	if req.Body != nil && contentLength < MaxContentLength {
		buf, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		span.SetMeta("elasticsearch.body", string(buf))
		req.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	}

	// Run the request using the standard transport.
	res, err := t.Transport.RoundTrip(req)
	if res != nil {
		span.SetMeta(ext.HTTPCode, strconv.Itoa(res.StatusCode))
	}

	if err != nil {
		span.SetError(err)
	} else if res.StatusCode < 200 || res.StatusCode > 299 {
		buf, err := ioutil.ReadAll(res.Body)
		if err != nil {
			// Status text is best we can do if if we can't read the body.
			span.SetError(errors.New(http.StatusText(res.StatusCode)))
		} else {
			span.SetError(errors.New(string(buf)))
		}
		res.Body = ioutil.NopCloser(bytes.NewBuffer(buf))
	}
	Quantize(span)

	return res, err
}

// NewTracedHTTPClient returns a new TracedTransport that traces HTTP requests.
func NewTracedHTTPClient(service string, tracer *tracer.Tracer) *http.Client {
	return &http.Client{
		Transport: &TracedTransport{service, tracer, &http.Transport{}},
	}
}

// NewTracedHTTPClientWithTransport returns a new TracedTransport that traces HTTP requests
// and takes in a Transport to use something other than the default.
func NewTracedHTTPClientWithTransport(service string, tracer *tracer.Tracer, transport *http.Transport) *http.Client {
	return &http.Client{
		Transport: &TracedTransport{service, tracer, transport},
	}
}

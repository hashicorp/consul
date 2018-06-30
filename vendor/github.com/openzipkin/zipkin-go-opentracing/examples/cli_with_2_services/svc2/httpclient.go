// +build go1.7

package svc2

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/openzipkin/zipkin-go-opentracing/examples/middleware"
)

// client is our actual client implementation
type client struct {
	baseURL      string
	httpClient   *http.Client
	tracer       opentracing.Tracer
	traceRequest middleware.RequestFunc
}

// Sum implements our Service interface.
func (c *client) Sum(ctx context.Context, a int64, b int64) (int64, error) {
	// create new span using span found in context as parent (if none is found,
	// our span becomes the trace root).
	span, ctx := opentracing.StartSpanFromContext(ctx, "Sum")
	defer span.Finish()

	// assemble URL query
	url := fmt.Sprintf("%s/sum/?a=%d&b=%d", c.baseURL, a, b)

	// create the HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	// use our middleware to propagate our trace
	req = c.traceRequest(req.WithContext(ctx))

	// execute the HTTP request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// annotate our span with the error condition
		span.SetTag("error", err.Error())
		return 0, err
	}
	defer resp.Body.Close()

	// read the http response body
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// annotate our span with the error condition
		span.SetTag("error", err.Error())
		return 0, err
	}

	// convert html response to expected result type (int64)
	result, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		// annotate our span with the error condition
		span.SetTag("error", err.Error())
		return 0, err
	}

	// return the result
	return result, nil
}

// NewHTTPClient returns a new client instance to our svc2 using the HTTP
// transport.
func NewHTTPClient(tracer opentracing.Tracer, baseURL string) Service {
	return &client{
		baseURL:      baseURL,
		httpClient:   &http.Client{},
		tracer:       tracer,
		traceRequest: middleware.ToHTTPRequest(tracer),
	}
}

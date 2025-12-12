// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

// QueryOptions are used to parameterize a query
type QueryOptions struct {
	// Namespace overrides the `default` namespace
	// Note: Namespaces are available only in Consul Enterprise
	Namespace string

	// Partition overrides the `default` partition
	// Note: Partitions are available only in Consul Enterprise
	Partition string

	// Providing a peer name in the query option
	Peer string

	// RequireConsistent forces the read to be fully consistent.
	// This is more expensive but prevents ever performing a stale
	// read.
	RequireConsistent bool

	// ctx is an optional context pass through to the underlying HTTP
	// request layer. Use Context() and WithContext() to manage this.
	ctx context.Context

	// Token is used to provide a per-request ACL token
	// which overrides the agent's default token.
	Token string
}

// Client provides a client to the Consul API
type HttpClient struct {
	modifyLock sync.RWMutex
	headers    http.Header

	config api.Config
}

// Headers gets the current set of headers used for requests. This returns a
// copy; to modify it call AddHeader or SetHeaders.
func (c *HttpClient) Headers() http.Header {
	c.modifyLock.RLock()
	defer c.modifyLock.RUnlock()

	if c.headers == nil {
		return nil
	}

	ret := make(http.Header)
	for k, v := range c.headers {
		ret[k] = append(ret[k], v...)
	}

	return ret
}

// NewClient returns a new client
func NewClient(config *api.Config) (*HttpClient, error) {
	// bootstrap the config
	defConfig := api.DefaultConfig()

	if config.Address == "" {
		config.Address = defConfig.Address
	}

	if config.Scheme == "" {
		config.Scheme = defConfig.Scheme
	}

	if config.Transport == nil {
		config.Transport = defConfig.Transport
	}

	if config.HttpClient == nil {
		var err error
		config.HttpClient, err = NewHttpClient(config.Transport)
		if err != nil {
			return nil, err
		}
	}

	return &HttpClient{config: *config, headers: make(http.Header)}, nil
}

// NewHttpClient returns an http client configured with the given Transport and TLS
// config.
func NewHttpClient(transport *http.Transport) (*http.Client, error) {
	client := &http.Client{
		Transport: transport,
	}

	return client, nil
}

// request is used to help build up a request
type request struct {
	config *api.Config
	method string
	url    *url.URL
	params url.Values
	body   io.Reader
	header http.Header
	Obj    interface{}
	ctx    context.Context
}

// setQueryOptions is used to annotate the request with
// additional query options
func (r *request) SetQueryOptions(q *QueryOptions) {
	if q == nil {
		return
	}
	if q.Namespace != "" {
		// For backwards-compatibility with existing tests,
		// use the short-hand query param name "ns"
		// rather than the alternative long-hand "namespace"
		r.params.Set("ns", q.Namespace)
	}
	if q.Partition != "" {
		// For backwards-compatibility with existing tests,
		// use the long-hand query param name "partition"
		// rather than the alternative short-hand "ap"
		r.params.Set("partition", q.Partition)
	}
	if q.Peer != "" {
		r.params.Set("peer", q.Peer)
	}

	if q.RequireConsistent {
		r.params.Set("consistent", "")
	}

	if q.Token != "" {
		r.header.Set("X-Consul-Token", q.Token)
	}

	r.ctx = q.ctx
}

// toHTTP converts the request to an HTTP request
func (r *request) toHTTP() (*http.Request, error) {
	// Encode the query parameters
	r.url.RawQuery = r.params.Encode()

	// Check if we should encode the body
	if r.body == nil && r.Obj != nil {
		b, err := encodeBody(r.Obj)
		if err != nil {
			return nil, err
		}
		r.body = b
	}

	// Create the HTTP request
	req, err := http.NewRequest(r.method, r.url.RequestURI(), r.body)
	if err != nil {
		return nil, err
	}

	// validate that socket communications that do not use the host, detect
	// slashes in the host name and replace it with local host.
	// this is required since go started validating req.host in 1.20.6 and 1.19.11.
	// prior to that they would strip out the slashes for you.  They removed that
	// behavior and added more strict validation as part of a CVE.
	// This issue is being tracked by the Go team:
	// https://github.com/golang/go/issues/61431
	// If there is a resolution in this issue, we will remove this code.
	// In the time being, this is the accepted workaround.
	if strings.HasPrefix(r.url.Host, "/") {
		r.url.Host = "localhost"
	}

	req.URL.Host = r.url.Host
	req.URL.Scheme = r.url.Scheme
	req.Host = r.url.Host
	req.Header = r.header

	// Content-Type must always be set when a body is present
	// See https://github.com/hashicorp/consul/issues/10011
	if req.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Setup auth
	if r.config.HttpAuth != nil {
		req.SetBasicAuth(r.config.HttpAuth.Username, r.config.HttpAuth.Password)
	}
	if r.ctx != nil {
		return req.WithContext(r.ctx), nil
	}

	return req, nil
}

// newRequest is used to create a new request
func (c *HttpClient) NewRequest(method, path string) *request {
	r := &request{
		config: &c.config,
		method: method,
		url: &url.URL{
			Scheme: c.config.Scheme,
			Host:   c.config.Address,
			Path:   c.config.PathPrefix + path,
		},
		params: make(map[string][]string),
		header: c.Headers(),
	}

	if c.config.Namespace != "" {
		r.params.Set("ns", c.config.Namespace)
	}
	if c.config.Partition != "" {
		r.params.Set("partition", c.config.Partition)
	}
	if c.config.Token != "" {
		r.header.Set("X-Consul-Token", r.config.Token)
	}
	return r
}

// doRequest runs a request with our client
func (c *HttpClient) DoRequest(r *request) (time.Duration, *http.Response, error) {
	req, err := r.toHTTP()
	if err != nil {
		return 0, nil, err
	}
	start := time.Now()
	resp, err := c.config.HttpClient.Do(req)
	diff := time.Since(start)
	return diff, resp, err
}

// DecodeBody is used to JSON decode a body
func DecodeBody(resp *http.Response, out interface{}) error {
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

// encodeBody is used to encode a request body
func encodeBody(obj interface{}) (io.Reader, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(obj); err != nil {
		return nil, err
	}
	return buf, nil
}

// requireOK is used to wrap doRequest and check for a 200
func RequireOK(resp *http.Response) error {
	return RequireHttpCodes(resp, 200)
}

// requireHttpCodes checks for the "allowable" http codes for a response
func RequireHttpCodes(resp *http.Response, httpCodes ...int) error {
	// if there is an http code that we require, return w no error
	for _, httpCode := range httpCodes {
		if resp.StatusCode == httpCode {
			return nil
		}
	}

	// if we reached here, then none of the http codes in resp matched any that we expected
	// so err out
	return generateUnexpectedResponseCodeError(resp)
}

// closeResponseBody reads resp.Body until EOF, and then closes it. The read
// is necessary to ensure that the http.Client's underlying RoundTripper is able
// to re-use the TCP connection. See godoc on net/http.Client.Do.
func CloseResponseBody(resp *http.Response) error {
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

type StatusError struct {
	Code int
	Body string
}

func (e StatusError) Error() string {
	return fmt.Sprintf("Unexpected response code: %d (%s)", e.Code, e.Body)
}

// generateUnexpectedResponseCodeError consumes the rest of the body, closes
// the body stream and generates an error indicating the status code was
// unexpected.
func generateUnexpectedResponseCodeError(resp *http.Response) error {
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	CloseResponseBody(resp)

	trimmed := strings.TrimSpace(buf.String())
	return StatusError{Code: resp.StatusCode, Body: trimmed}
}

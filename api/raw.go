package api

import "net/http"

// Raw can be used to do raw queries against custom endpoints
type Raw struct {
	c *Client
}

// Raw returns a handle to query endpoints
func (c *Client) Raw() *Raw {
	return &Raw{c}
}

// Query is used to do a GET request against an endpoint
// and deserialize the response into an interface using
// standard Consul conventions.
func (raw *Raw) Query(endpoint string, out interface{}, q *QueryOptions) (*QueryMeta, error) {
	return raw.c.query(endpoint, out, q)
}

// QueryRaw is used to do a GET request against an endpoint
// without deserializing the response. The caller is responsible to close
// response body.
func (raw *Raw) QueryRaw(endpoint string, q *QueryOptions) (*http.Response, *QueryMeta, error) {
	return raw.c.queryRaw(endpoint, q)
}

// Write is used to do a PUT request against an endpoint
// and serialize/deserialized using the standard Consul conventions.
func (raw *Raw) Write(endpoint string, in, out interface{}, q *WriteOptions) (*WriteMeta, error) {
	return raw.c.write(endpoint, in, out, q)
}

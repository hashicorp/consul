package connect

import (
	"fmt"
	"net/url"
)

type SpiffeIDServer struct {
	Host       string
	Datacenter string
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDServer) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = fmt.Sprintf("/agent/server/dc/%s", id.Datacenter)
	return &result
}

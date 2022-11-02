package connect

import (
	"net/url"

	"github.com/hashicorp/consul/acl"
)

// SpiffeIDService is the structure to represent the SPIFFE ID for an agent.
type SpiffeIDAgent struct {
	Host       string
	Partition  string
	Datacenter string
	Agent      string
}

func (id SpiffeIDAgent) PartitionOrDefault() string {
	return acl.PartitionOrDefault(id.Partition)
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDAgent) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = id.uriPath()
	return &result
}

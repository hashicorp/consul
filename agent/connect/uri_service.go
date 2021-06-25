package connect

import (
	"net/url"

	"github.com/hashicorp/consul/agent/structs"
)

// SpiffeIDService is the structure to represent the SPIFFE ID for a service.
type SpiffeIDService struct {
	Host       string
	Partition  string
	Namespace  string
	Datacenter string
	Service    string
}

func (id SpiffeIDService) NamespaceOrDefault() string {
	return structs.NamespaceOrDefault(id.Namespace)
}

func (id SpiffeIDService) MatchesPartition(partition string) bool {
	return id.PartitionOrDefault() == structs.PartitionOrDefault(partition)
}

func (id SpiffeIDService) PartitionOrDefault() string {
	return structs.PartitionOrDefault(id.Partition)
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDService) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = id.uriPath()
	return &result
}

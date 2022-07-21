package connect

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/consul/acl"
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
	return acl.NamespaceOrDefault(id.Namespace)
}

func (id SpiffeIDService) MatchesPartition(partition string) bool {
	return id.PartitionOrDefault() == acl.PartitionOrDefault(partition)
}

// URI returns the *url.URL for this SPIFFE ID.
func (id SpiffeIDService) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = id.uriPath()
	return &result
}

func (id SpiffeIDService) uriPath() string {
	path := fmt.Sprintf("/ns/%s/dc/%s/svc/%s",
		id.NamespaceOrDefault(),
		id.Datacenter,
		id.Service,
	)

	// Although OSS has no support for partitions, it still needs to be able to
	// handle exportedPartition from peered Consul Enterprise clusters in order
	// to generate the correct SpiffeID.
	// We intentionally avoid using pbpartition.DefaultName here to be OSS friendly.
	if ap := id.PartitionOrDefault(); ap != "" && ap != "default" {
		return "/ap/" + ap + path
	}
	return path
}

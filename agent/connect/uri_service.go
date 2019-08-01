package connect

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/consul/agent/structs"
)

// SpiffeIDConsulService is the structure to represent the SPIFFE ID for a
// Consul service (in contrast to the ID for an external service).
type SpiffeIDConsulService struct {
	Host       string
	Namespace  string
	Datacenter string
	Service    string
}

// URI returns the *url.URL for this SPIFFE ID.
func (id *SpiffeIDConsulService) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = fmt.Sprintf("/ns/%s/dc/%s/svc/%s",
		id.Namespace, id.Datacenter, id.Service)
	return &result
}

// CertURI impl.
func (id *SpiffeIDConsulService) Authorize(ixn *structs.Intention) (bool, bool) {
	if ixn.SourceNS != structs.IntentionWildcard && ixn.SourceNS != id.Namespace {
		// Non-matching namespace
		return false, false
	}

	if ixn.SourceName != structs.IntentionWildcard && ixn.SourceName != id.Service {
		// Non-matching name
		return false, false
	}

	// Match, return allow value
	return ixn.Action == structs.IntentionActionAllow, true
}

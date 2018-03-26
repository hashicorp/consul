package connect

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/consul/agent/structs"
)

// SpiffeIDSigning is the structure to represent the SPIFFE ID for a
// signing certificate (not a leaf service).
type SpiffeIDSigning struct {
	ClusterID string // Unique cluster ID
	Domain    string // The domain, usually "consul"
}

// URI returns the *url.URL for this SPIFFE ID.
func (id *SpiffeIDSigning) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = fmt.Sprintf("%s.%s", id.ClusterID, id.Domain)
	return &result
}

// CertURI impl.
func (id *SpiffeIDSigning) Authorize(ixn *structs.Intention) (bool, bool) {
	// Never authorize as a client.
	return false, true
}

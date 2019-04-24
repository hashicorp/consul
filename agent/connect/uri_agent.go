package connect

import (
	"fmt"
	"net/url"

	"github.com/hashicorp/consul/agent/structs"
)

// SpiffeIDService is the structure to represent the SPIFFE ID for an agent.
type SpiffeIDAgent struct {
	Host  string
	Agent string
}

// URI returns the *url.URL for this SPIFFE ID.
func (id *SpiffeIDAgent) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = fmt.Sprintf("/agent/%s", id.Agent)
	return &result
}

// CertURI impl.
func (id *SpiffeIDAgent) Authorize(_ *structs.Intention) (bool, bool) {
	return true, true
}

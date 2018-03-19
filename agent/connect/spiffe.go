package connect

import (
	"fmt"
	"net/url"
	"regexp"
)

// SpiffeID represents a Connect-valid SPIFFE ID. The user should type switch
// on the various implementations in this package to determine the type of ID.
type SpiffeID interface {
	URI() *url.URL
}

var (
	spiffeIDServiceRegexp = regexp.MustCompile(
		`^/ns/(\w+)/dc/(\w+)/svc/(\w+)$`)
)

// ParseSpiffeID parses a SPIFFE ID from the input URI.
func ParseSpiffeID(input *url.URL) (SpiffeID, error) {
	if input.Scheme != "spiffe" {
		return nil, fmt.Errorf("SPIFFE ID must have 'spiffe' scheme")
	}

	// Test for service IDs
	if v := spiffeIDServiceRegexp.FindStringSubmatch(input.Path); v != nil {
		return &SpiffeIDService{
			Host:       input.Host,
			Namespace:  v[1],
			Datacenter: v[2],
			Service:    v[3],
		}, nil
	}

	return nil, fmt.Errorf("SPIFFE ID is not in the expected format")
}

// SpiffeIDService is the structure to represent the SPIFFE ID for a service.
type SpiffeIDService struct {
	Host       string
	Namespace  string
	Datacenter string
	Service    string
}

// URI returns the *url.URL for this SPIFFE ID.
func (id *SpiffeIDService) URI() *url.URL {
	var result url.URL
	result.Scheme = "spiffe"
	result.Host = id.Host
	result.Path = fmt.Sprintf("/ns/%s/dc/%s/svc/%s",
		id.Namespace, id.Datacenter, id.Service)
	return &result
}

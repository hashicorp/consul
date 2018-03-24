package connect

import (
	"fmt"
	"net/url"
	"regexp"
)

// CertURI represents a Connect-valid URI value for a TLS certificate.
// The user should type switch on the various implementations in this
// package to determine the type of URI and the data encoded within it.
//
// Note that the current implementations of this are all also SPIFFE IDs.
// However, we anticipate that we may accept URIs that are also not SPIFFE
// compliant and therefore the interface is named as such.
type CertURI interface {
	URI() *url.URL
}

var (
	spiffeIDServiceRegexp = regexp.MustCompile(
		`^/ns/(\w+)/dc/(\w+)/svc/(\w+)$`)
)

// ParseCertURI parses a the URI value from a TLS certificate.
func ParseCertURI(input *url.URL) (CertURI, error) {
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

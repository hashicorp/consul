package connect

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
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
		`^/ns/([^/]+)/dc/([^/]+)/svc/([^/]+)$`)
)

// ParseCertURI parses a the URI value from a TLS certificate.
func ParseCertURI(input *url.URL) (CertURI, error) {
	if input.Scheme != "spiffe" {
		return nil, fmt.Errorf("SPIFFE ID must have 'spiffe' scheme")
	}

	// Path is the raw value of the path without url decoding values.
	// RawPath is empty if there were no encoded values so we must
	// check both.
	path := input.Path
	if input.RawPath != "" {
		path = input.RawPath
	}

	// Test for service IDs
	if v := spiffeIDServiceRegexp.FindStringSubmatch(path); v != nil {
		// Determine the values. We assume they're sane to save cycles,
		// but if the raw path is not empty that means that something is
		// URL encoded so we go to the slow path.
		ns := v[1]
		dc := v[2]
		service := v[3]
		if input.RawPath != "" {
			var err error
			if ns, err = url.PathUnescape(v[1]); err != nil {
				return nil, fmt.Errorf("Invalid namespace: %s", err)
			}
			if dc, err = url.PathUnescape(v[2]); err != nil {
				return nil, fmt.Errorf("Invalid datacenter: %s", err)
			}
			if service, err = url.PathUnescape(v[3]); err != nil {
				return nil, fmt.Errorf("Invalid service: %s", err)
			}
		}

		return &SpiffeIDService{
			Host:       input.Host,
			Namespace:  ns,
			Datacenter: dc,
			Service:    service,
		}, nil
	}

	// Test for signing ID
	if input.Path == "" {
		idx := strings.Index(input.Host, ".")
		if idx > 0 {
			return &SpiffeIDSigning{
				ClusterID: input.Host[:idx],
				Domain:    input.Host[idx+1:],
			}, nil
		}
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

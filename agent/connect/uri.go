package connect

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// CertURI represents a Connect-valid URI value for a TLS certificate.
// The user should type switch on the various implementations in this
// package to determine the type of URI and the data encoded within it.
//
// Note that the current implementations of this are all also SPIFFE IDs.
// However, we anticipate that we may accept URIs that are also not SPIFFE
// compliant and therefore the interface is named as such.
type CertURI interface {
	// Authorize tests the authorization for this URI as a client
	// for the given intention. The return value `auth` is only valid if
	// the second value `match` is true. If the second value `match` is
	// false, then the intention doesn't match this client and any
	// result should be ignored.
	Authorize(*structs.Intention) (auth bool, match bool)

	// URI is the valid URI value used in the cert.
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

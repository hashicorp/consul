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
	// URI is the valid URI value used in the cert.
	URI() *url.URL
}

var (
	spiffeIDServiceRegexp = regexp.MustCompile(
		`^(?:/ap/([^/]+))?/ns/([^/]+)/dc/([^/]+)/svc/([^/]+)$`)
	spiffeIDAgentRegexp = regexp.MustCompile(
		`^(?:/ap/([^/]+))?/agent/client/dc/([^/]+)/id/([^/]+)$`)
	spiffeIDServerRegexp = regexp.MustCompile(
		`^/agent/server/dc/([^/]+)$`)
	spiffeIDMeshGatewayRegexp = regexp.MustCompile(
		`^(?:/ap/([^/]+))?/gateway/mesh/dc/([^/]+)$`)
)

// ParseCertURIFromString attempts to parse a string representation of a
// certificate URI as a convenience helper around ParseCertURI.
func ParseCertURIFromString(input string) (CertURI, error) {
	// Parse the certificate URI from the string
	uriRaw, err := url.Parse(input)
	if err != nil {
		return nil, err
	}
	return ParseCertURI(uriRaw)
}

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
		// Determine the values. We assume they're reasonable to save cycles,
		// but if the raw path is not empty that means that something is
		// URL encoded so we go to the slow path.
		ap := v[1]
		ns := v[2]
		dc := v[3]
		service := v[4]
		if input.RawPath != "" {
			var err error
			if ap, err = url.PathUnescape(v[1]); err != nil {
				return nil, fmt.Errorf("Invalid admin partition: %s", err)
			}
			if ns, err = url.PathUnescape(v[2]); err != nil {
				return nil, fmt.Errorf("Invalid namespace: %s", err)
			}
			if dc, err = url.PathUnescape(v[3]); err != nil {
				return nil, fmt.Errorf("Invalid datacenter: %s", err)
			}
			if service, err = url.PathUnescape(v[4]); err != nil {
				return nil, fmt.Errorf("Invalid service: %s", err)
			}
		}

		if ap == "" {
			ap = "default"
		}

		return &SpiffeIDService{
			Host:       input.Host,
			Partition:  ap,
			Namespace:  ns,
			Datacenter: dc,
			Service:    service,
		}, nil
	} else if v := spiffeIDAgentRegexp.FindStringSubmatch(path); v != nil {
		// Determine the values. We assume they're reasonable to save cycles,
		// but if the raw path is not empty that means that something is
		// URL encoded so we go to the slow path.
		ap := v[1]
		dc := v[2]
		agent := v[3]
		if input.RawPath != "" {
			var err error
			if ap, err = url.PathUnescape(v[1]); err != nil {
				return nil, fmt.Errorf("Invalid admin partition: %s", err)
			}
			if dc, err = url.PathUnescape(v[2]); err != nil {
				return nil, fmt.Errorf("Invalid datacenter: %s", err)
			}
			if agent, err = url.PathUnescape(v[3]); err != nil {
				return nil, fmt.Errorf("Invalid node: %s", err)
			}
		}

		if ap == "" {
			ap = "default"
		}

		return &SpiffeIDAgent{
			Host:       input.Host,
			Partition:  ap,
			Datacenter: dc,
			Agent:      agent,
		}, nil
	} else if v := spiffeIDMeshGatewayRegexp.FindStringSubmatch(path); v != nil {
		// Determine the values. We assume they're reasonable to save cycles,
		// but if the raw path is not empty that means that something is
		// URL encoded so we go to the slow path.
		ap := v[1]
		dc := v[2]
		if input.RawPath != "" {
			var err error
			if ap, err = url.PathUnescape(v[1]); err != nil {
				return nil, fmt.Errorf("Invalid admin partition: %s", err)
			}
			if dc, err = url.PathUnescape(v[2]); err != nil {
				return nil, fmt.Errorf("Invalid datacenter: %s", err)
			}
		}

		if ap == "" {
			ap = "default"
		}

		return &SpiffeIDMeshGateway{
			Host:       input.Host,
			Partition:  ap,
			Datacenter: dc,
		}, nil
	} else if v := spiffeIDServerRegexp.FindStringSubmatch(path); v != nil {
		dc := v[1]
		if input.RawPath != "" {
			var err error
			if dc, err = url.PathUnescape(v[1]); err != nil {
				return nil, fmt.Errorf("Invalid datacenter: %s", err)
			}
		}

		return &SpiffeIDServer{
			Host:       input.Host,
			Datacenter: dc,
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

	return nil, fmt.Errorf("SPIFFE ID is not in the expected format: %s", input.String())
}

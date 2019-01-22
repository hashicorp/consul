package connect

import (
	"fmt"
	"net/url"
	"strings"

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
	result.Host = id.Host()
	return &result
}

// Host is the canonical representation as a DNS-compatible hostname.
func (id *SpiffeIDSigning) Host() string {
	return strings.ToLower(fmt.Sprintf("%s.%s", id.ClusterID, id.Domain))
}

// CertURI impl.
func (id *SpiffeIDSigning) Authorize(ixn *structs.Intention) (bool, bool) {
	// Never authorize as a client.
	return false, true
}

// CanSign takes any CertURI and returns whether or not this signing entity is
// allowed to sign CSRs for that entity (i.e. represents the trust domain for
// that entity).
//
// I choose to make this a fixed centralised method here for now rather than a
// method on CertURI interface since we don't intend this to be extensible
// outside and it's easier to reason about the security properties when they are
// all in one place with "whitelist" semantics.
func (id *SpiffeIDSigning) CanSign(cu CertURI) bool {
	switch other := cu.(type) {
	case *SpiffeIDSigning:
		// We can only sign other CA certificates for the same trust domain. Note
		// that we could open this up later for example to support external
		// federation of roots and cross-signing external roots that have different
		// URI structure but it's simpler to start off restrictive.
		return id.URI().String() == other.URI().String()
	case *SpiffeIDService:
		// The host component of the service must be an exact match for now under
		// ascii case folding (since hostnames are case-insensitive). Later we might
		// worry about Unicode domains if we start allowing customisation beyond the
		// built-in cluster ids.
		return strings.ToLower(other.Host) == id.Host()
	default:
		return false
	}
}

// SpiffeIDSigningForCluster returns the SPIFFE signing identifier (trust
// domain) representation of the given CA config. If config is nil this function
// will panic.
//
// NOTE(banks): we intentionally fix the tld `.consul` for now rather than tie
// this to the `domain` config used for DNS because changing DNS domain can't
// break all certificate validation. That does mean that DNS prefix might not
// match the identity URIs and so the trust domain might not actually resolve
// which we would like but don't actually need.
func SpiffeIDSigningForCluster(config *structs.CAConfiguration) *SpiffeIDSigning {
	return &SpiffeIDSigning{ClusterID: config.ClusterID, Domain: "consul"}
}

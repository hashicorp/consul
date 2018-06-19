package ca

import (
	"crypto/x509"
)

// Provider is the interface for Consul to interact with
// an external CA that provides leaf certificate signing for
// given SpiffeIDServices.
type Provider interface {
	// Active root returns the currently active root CA for this
	// provider. This should be a parent of the certificate returned by
	// ActiveIntermediate()
	ActiveRoot() (string, error)

	// ActiveIntermediate returns the current signing cert used by this provider
	// for generating SPIFFE leaf certs. Note that this must not change except
	// when Consul requests the change via GenerateIntermediate. Changing the
	// signing cert will break Consul's assumptions about which validation paths
	// are active.
	ActiveIntermediate() (string, error)

	// GenerateIntermediate returns a new intermediate signing cert and sets it to
	// the active intermediate. If multiple intermediates are needed to complete
	// the chain from the signing certificate back to the active root, they should
	// all by bundled here.
	GenerateIntermediate() (string, error)

	// Sign signs a leaf certificate used by Connect proxies from a CSR. The PEM
	// returned should include only the leaf certificate as all Intermediates
	// needed to validate it will be added by Consul based on the active
	// intemediate and any cross-signed intermediates managed by Consul.
	Sign(*x509.CertificateRequest) (string, error)

	// CrossSignCA must accept a CA certificate from another CA provider
	// and cross sign it exactly as it is such that it forms a chain back the the
	// CAProvider's current root. Specifically, the Distinguished Name, Subject
	// Alternative Name, SubjectKeyID and other relevant extensions must be kept.
	// The resulting certificate must have a distinct Serial Number and the
	// AuthorityKeyID set to the CAProvider's current signing key as well as the
	// Issuer related fields changed as necessary. The resulting certificate is
	// returned as a PEM formatted string.
	CrossSignCA(*x509.Certificate) (string, error)

	// Cleanup performs any necessary cleanup that should happen when the provider
	// is shut down permanently, such as removing a temporary PKI backend in Vault
	// created for an intermediate CA.
	Cleanup() error
}

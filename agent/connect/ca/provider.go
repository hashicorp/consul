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

	// ActiveIntermediate returns the current signing cert used by this
	// provider for generating SPIFFE leaf certs.
	ActiveIntermediate() (string, error)

	// GenerateIntermediate returns a new intermediate signing cert and
	// sets it to the active intermediate.
	GenerateIntermediate() (string, error)

	// Sign signs a leaf certificate used by Connect proxies from a CSR.
	Sign(*x509.CertificateRequest) (string, error)

	// CrossSignCA must accept a CA certificate signed by another CA's key
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

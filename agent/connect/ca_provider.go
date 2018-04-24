package connect

import (
	"crypto/x509"

	"github.com/hashicorp/consul/agent/structs"
)

// CAProvider is the interface for Consul to interact with
// an external CA that provides leaf certificate signing for
// given SpiffeIDServices.
type CAProvider interface {
	// Active root returns the currently active root CA for this
	// provider. This should be a parent of the certificate returned by
	// ActiveIntermediate()
	ActiveRoot() (*structs.CARoot, error)

	// ActiveIntermediate returns the current signing cert used by this
	// provider for generating SPIFFE leaf certs.
	ActiveIntermediate() (*structs.CARoot, error)

	// GenerateIntermediate returns a new intermediate signing cert, a
	// cross-signing CSR for it and sets it to the active intermediate.
	GenerateIntermediate() (*structs.CARoot, *x509.CertificateRequest, error)

	// Sign signs a leaf certificate used by Connect proxies from a CSR.
	Sign(*SpiffeIDService, *x509.CertificateRequest) (*structs.IssuedCert, error)

	// SignCA signs a CA CSR and returns the resulting cross-signed cert.
	SignCA(*x509.CertificateRequest) (string, error)

	// Cleanup performs any necessary cleanup that should happen when the provider
	// is shut down permanently, such as removing a temporary PKI backend in Vault
	// created for an intermediate CA.
	Cleanup() error
}

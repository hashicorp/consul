package ca

import (
	"crypto/x509"
	"log"
)

//go:generate mockery -name Provider -inpkg

// Provider is the interface for Consul to interact with
// an external CA that provides leaf certificate signing for
// given SpiffeIDServices.
type Provider interface {
	// Configure initializes the provider based on the given cluster ID, root
	// status and configuration values. rawConfig contains the user-provided
	// Config. State contains a the State the same provider last persisted on a
	// restart or reconfiguration. The provider must not modify `rawConfig` or
	// `state` maps directly as it may be being read from other goroutines.
	Configure(clusterID string, isRoot bool, rawConfig map[string]interface{}, state map[string]string) error

	// State returns the current provider state. If the provider doesn't need to
	// store anything other than what the user configured this can return nil. It
	// is called after any config change before the new active config is stored in
	// the state store and the most recent value returned by the provider is given
	// in subsequent `Configure` calls provided that the current provider is the
	// same type as the new provider instance being configured. This provides a
	// simple way for providers to persist information like UUIDs of resources
	// they manage. This state is visible to anyone with operator:read via the API
	// so it's not intended for storing secrets like root private keys. Only
	// strings are permitted since this has to pass through msgpack and so
	// interface values will end up mangled in many cases which is ugly for all
	// provider code to have to remember to reason about.
	//
	// Note that the map returned will be accessed (read-only) in other goroutines
	// - for example passed to Configure in the Connect CA Config RPC endpoint -
	// so it must not just be a pointer to a map that may internally be modified.
	// If the Provider only writes to it during Configure it's safe to return
	// as-is, but otherwise it's assumed the map returned is a copy of the state
	// in the Provider struct so it won't change after being returned.
	State() (map[string]string, error)

	// GenerateRoot causes the creation of a new root certificate for this provider.
	// This can also be a no-op if a root certificate already exists for the given
	// config. If isRoot is false, calling this method is an error.
	GenerateRoot() error

	// ActiveRoot returns the currently active root CA for this
	// provider. This should be a parent of the certificate returned by
	// ActiveIntermediate()
	ActiveRoot() (string, error)

	// GenerateIntermediateCSR generates a CSR for an intermediate CA
	// certificate, to be signed by the root of another datacenter. If isRoot was
	// set to true with Configure(), calling this is an error.
	GenerateIntermediateCSR() (string, error)

	// SetIntermediate sets the provider to use the given intermediate certificate
	// as well as the root it was signed by. This completes the initialization for
	// a provider where isRoot was set to false in Configure().
	SetIntermediate(intermediatePEM, rootPEM string) error

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

	// SignIntermediate will validate the CSR to ensure the trust domain in the
	// URI SAN matches the local one and that basic constraints for a CA certificate
	// are met. It should return a signed CA certificate with a path length constraint
	// of 0 to ensure that the certificate cannot be used to generate further CA certs.
	SignIntermediate(*x509.CertificateRequest) (string, error)

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

// NeedsLogger is an optional interface that allows a CA provider to use the
// Consul logger to output diagnostic messages.
type NeedsLogger interface {
	// SetLogger will pass a configured Logger to the provider.
	// TODO(hclog) convert this to an hclog.Logger.
	SetLogger(logger *log.Logger)
}

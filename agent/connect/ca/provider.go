// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"crypto/x509"
	"errors"
)

//go:generate mockery --name Provider --inpackage

// ErrRateLimited is a sentinel error value Providers may return from any method
// to indicate that the operation can't complete due to a temporary rate limit.
// In the case of signing new certificates, Consul clients will respect this and
// intelligently backoff to optimize rotation rollout time while reducing load
// on servers and CA provider.
var ErrRateLimited = errors.New("operation rate limited by CA provider")

// PrimaryUsesIntermediate is an optional interface that CA providers may implement
// to indicate that they use an intermediate cert in the primary datacenter as
// well as the secondary. This is used when determining whether to run the
// intermediate renewal routine in the primary.
type PrimaryUsesIntermediate interface {
	// GenerateLeafSigningCert returns a new intermediate signing cert and sets it to
	// the active intermediate. If multiple intermediates are needed to complete
	// the chain from the signing certificate back to the active root, they should
	// all by bundled here.
	GenerateLeafSigningCert() (string, error)
}

// ProviderConfig encapsulates all the data Consul passes to `Configure` on a
// new provider instance. The provider must treat this as read-only and make
// copies of any map or slice if it might modify them internally.
type ProviderConfig struct {
	// ClusterID is the current Consul cluster ID.
	ClusterID string

	// Datacenter is the current Consul datacenter.
	Datacenter string

	// IsPrimary is true when the CA instance is in the primary DC typically it
	// may choose to act as a root in this case while secondaries are typically
	// intermediate CAs. In some case the primary DC in Consul is an intermediate
	// signed by some external CA along with that CA's public cert so the old name
	// of `IsRoot` was misleading.
	IsPrimary bool

	// RawConfig is the user configuration for the provider and is
	// provider-specific to be interpreted as the provider wishes.
	RawConfig map[string]interface{}

	// State contains the State the same provider last persisted. It is provided
	// after a restart or reconfiguration, or on a leader election on a new server
	// to maintain operation. It MUST NOT be used for secret storage since it is
	// visible in the API to operators. It's intended use is to store small bits
	// of state like UUIDs of external resources that the provider has created and
	// needs to continue to manage.
	State map[string]string
}

// Provider is the interface for Consul to interact with
// an external CA that provides leaf certificate signing for
// given SpiffeIDServices.
type Provider interface {
	// Configure initializes the provider based on the given cluster ID, root
	// status and configuration values. rawConfig contains the user-provided
	// Config. State contains a the State the same provider last persisted on a
	// restart or reconfiguration. The provider must not modify `rawConfig` or
	// `state` maps directly as it may be being read from other goroutines.
	Configure(cfg ProviderConfig) error

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

	// ActiveLeafSigningCert returns the current signing cert used by this provider
	// for generating SPIFFE leaf certs. Note that this must not change except
	// when Consul requests the change via GenerateLeafSigningCert. Changing the
	// signing cert will break Consul's assumptions about which validation paths
	// are active.
	ActiveLeafSigningCert() (string, error)

	// Sign signs a leaf certificate used by Connect proxies from a CSR. The PEM
	// returned should include only the leaf certificate as all Intermediates
	// needed to validate it will be added by Consul based on the active
	// intermediate and any cross-signed intermediates managed by Consul. Note that
	// providers should return ErrRateLimited if they are unable to complete the
	// operation due to upstream rate limiting so that clients can intelligently
	// backoff.
	Sign(*x509.CertificateRequest) (string, error)

	// Cleanup performs any necessary cleanup that should happen when the provider
	// is shut down permanently, such as removing a temporary PKI backend in Vault
	// created for an intermediate CA. Whether the CA provider type is changing
	// and the other providers raw configuration is passed along so that the provider
	// instance can determine which cleanup steps to perform. For example, when the
	// Vault provider is in use and there is no type change occuring, the Vault
	// provider should check if the intermediate PKI path is changing. If it is not
	// changing then the provider should not remove that path from Vault.
	Cleanup(providerTypeChange bool, otherConfig map[string]interface{}) error

	// TODO: when CAManager has separate types for primary/secondary invert this
	// relationship so that PrimaryProvider/SecondaryProvider embed Provider

	PrimaryProvider
	SecondaryProvider
}

type PrimaryProvider interface {
	// GenerateCAChain is called:
	//   * to initialize the CA system when a server is elected as a raft leader
	//   * when the CA configuration is updated in a way that might require
	//     generating a new root certificate.
	//
	// In both cases GenerateCAChain is always called on a newly created provider
	// after calling Provider.Configure, and before any other calls to the
	// provider.
	//
	// Depending on the provider and its configuration, GenerateCAChain may return
	// a single root certificate or a chain of certs. The provider should return an
	// existing CA chain if one exists or generate a new one and return it.
	GenerateCAChain() (CAChainResult, error)

	// SignIntermediate will validate the CSR to ensure the trust domain in the
	// URI SAN matches the local one and that basic constraints for a CA
	// certificate are met. It should return a signed CA certificate with a path
	// length constraint of 0 to ensure that the certificate cannot be used to
	// generate further CA certs. Note that providers should return ErrRateLimited
	// if they are unable to complete the operation due to upstream rate limiting
	// so that clients can intelligently backoff.
	SignIntermediate(*x509.CertificateRequest) (string, error)

	// CrossSignCA must accept a CA certificate from another CA provider and cross
	// sign it exactly as it is such that it forms a chain back the the
	// CAProvider's current root. Specifically, the Distinguished Name, Subject
	// Alternative Name, SubjectKeyID and other relevant extensions must be kept.
	// The resulting certificate must have a distinct Serial Number and the
	// AuthorityKeyID set to the CAProvider's current signing key as well as the
	// Issuer related fields changed as necessary. The resulting certificate is
	// returned as a PEM formatted string.
	//
	// If the CA provider does not support this operation, it may return an error
	// provided `SupportsCrossSigning` also returns false. Note that
	// providers should return ErrRateLimited if they are unable to complete the
	// operation due to upstream rate limiting so that clients can intelligently
	// backoff.
	CrossSignCA(*x509.Certificate) (string, error)

	// SupportsCrossSigning should indicate whether the CA provider supports
	// cross-signing an external root to provide a seamless rotation. If the CA
	// does not support this, the user will have to force an upgrade when that CA
	// provider is the current CA as the upgrade may cause interruptions to
	// connectivity during the rollout.
	SupportsCrossSigning() (bool, error)
}

type SecondaryProvider interface {
	// GenerateIntermediateCSR should return a CSR for an intermediate CA
	// certificate. The intermediate CA will be signed by the primary CA and
	// should be used by the provider to sign leaf certificates in the local
	// datacenter.
	//
	// After the certificate is signed, SecondaryProvider.SetIntermediate will
	// be called to store the intermediate CA.
	//
	// The second return value is an opaque string meant to be passed back to
	// the subsequent call to SetIntermediate.
	GenerateIntermediateCSR() (string, string, error)

	// SetIntermediate is called to store a newly signed leaf signing certificate and
	// the chain of certificates back to the root CA certificate.
	//
	// The provider should save the certificates and use them to
	// Provider.Sign leaf certificates.
	SetIntermediate(intermediatePEM, rootPEM, opaque string) error
}

// CAChainResult is the result returned by PrimaryProvider.GenerateCAChain.
type CAChainResult struct {
	// PEM encoded bundle of CA certificates. The first certificate must be the
	// primary CA used to sign intermediates for secondary datacenters, and the
	// last certificate must be the trusted CA.
	//
	// If there is only a single certificate in the bundle then it will be used
	// as both the primary CA and the trusted CA.
	PEM string

	// IntermediatePEM is an encoded bundle of CA certificates used only by
	// providers that use an intermediate CA to sign leaf certificates (e.g.
	// Vault). Its issuer should form a chain leading to the trusted CA in PEM.
	IntermediatePEM string
}

// NeedsStop is an optional interface that allows a CA to define a function
// to be called when the CA instance is no longer in use. This is different
// from Cleanup(), as only the local provider instance is being shut down
// such as in the case of a leader change.
type NeedsStop interface {
	Stop()
}

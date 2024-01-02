// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/consul/lib/stringslice"

	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/lib"
)

const (
	DefaultLeafCertTTL         = "72h"
	DefaultIntermediateCertTTL = "8760h"  // ~ 1 year = 365 * 24h
	DefaultRootCertTTL         = "87600h" // ~ 10 years = 365 * 24h * 10
)

// IndexedCARoots is the list of currently trusted CA Roots.
type IndexedCARoots struct {
	// ActiveRootID is the ID of a root in Roots that is the active CA root.
	// Other roots are still valid if they're in the Roots list but are in
	// the process of being rotated out.
	ActiveRootID string

	// TrustDomain is the identification root for this Consul cluster. All
	// certificates signed by the cluster's CA must have their identifying URI in
	// this domain.
	//
	// This does not include the protocol (currently spiffe://) since we may
	// implement other protocols in future with equivalent semantics. It should be
	// compared against the "authority" section of a URI (i.e. host:port).
	//
	// We need to support migrating a cluster between trust domains to support
	// Multi-DC migration in Enterprise. In this case the current trust domain is
	// here but entries in Roots may also have ExternalTrustDomain set to a
	// non-empty value implying they were previous roots that are still trusted
	// but under a different trust domain.
	//
	// Note that we DON'T validate trust domain during AuthZ since it causes
	// issues of loss of connectivity during migration between trust domains. The
	// only time the additional validation adds value is where the cluster shares
	// an external root (e.g. organization-wide root) with another distinct Consul
	// cluster or PKI system. In this case, x509 Name Constraints can be added to
	// enforce that Consul's CA can only validly sign or trust certs within the
	// same trust-domain. Name constraints as enforced by TLS handshake also allow
	// seamless rotation between trust domains thanks to cross-signing.
	TrustDomain string

	// Roots is a list of root CA certs to trust.
	Roots []*CARoot

	// QueryMeta contains the meta sent via a header. We ignore for JSON
	// so this whole structure can be returned.
	QueryMeta `json:"-"`
}

func (r IndexedCARoots) Active() *CARoot {
	for _, root := range r.Roots {
		if root.ID == r.ActiveRootID {
			return root
		}
	}
	return nil
}

// CARoot represents a root CA certificate that is trusted.
type CARoot struct {
	// ID is a globally unique ID (UUID) representing this CA chain. It is
	// calculated from the SHA1 of the primary CA certificate.
	ID string

	// Name is a human-friendly name for this CA root. This value is
	// opaque to Consul and is not used for anything internally.
	Name string

	// SerialNumber is the x509 serial number of the primary CA certificate.
	SerialNumber uint64

	// SigningKeyID is the connect.HexString encoded id of the public key that
	// corresponds to the private key used to sign leaf certificates in the
	// local datacenter.
	//
	// The value comes from x509.Certificate.SubjectKeyId of the local leaf
	// signing cert.
	//
	// See https://www.rfc-editor.org/rfc/rfc3280#section-4.2.1.1 for more detail.
	SigningKeyID string

	// ExternalTrustDomain is the trust domain this root was generated under. It
	// is usually empty implying "the current cluster trust-domain". It is set
	// only in the case that a cluster changes trust domain and then all old roots
	// that are still trusted have the old trust domain set here.
	//
	// We currently DON'T validate these trust domains explicitly anywhere, see
	// IndexedRoots.TrustDomain doc. We retain this information for debugging and
	// future flexibility.
	ExternalTrustDomain string

	// NotBefore is the x509.Certificate.NotBefore value of the primary CA
	// certificate. This value should generally be a time in the past.
	NotBefore time.Time
	// NotAfter is the  x509.Certificate.NotAfter value of the primary CA
	// certificate. This is the time when the certificate will expire.
	NotAfter time.Time

	// RootCert is the PEM-encoded public certificate for the root CA. The
	// certificate is the same for all federated clusters.
	RootCert string

	// IntermediateCerts is a list of PEM-encoded intermediate certs to
	// attach to any leaf certs signed by this CA. The list may include a
	// certificate cross-signed by an old root CA, any subordinate CAs below the
	// root CA, and the intermediate CA used to sign leaf certificates in the
	// local Datacenter.
	//
	// If the provider which created this root uses an intermediate to sign
	// leaf certificates (Vault provider), or this is a secondary Datacenter then
	// the intermediate used to sign leaf certificates will be the last in the
	// list.
	IntermediateCerts []string

	// SigningCert is the PEM-encoded signing certificate and SigningKey
	// is the PEM-encoded private key for the signing certificate. These
	// may actually be empty if the CA plugin in use manages these for us.
	SigningCert string `json:",omitempty"`
	SigningKey  string `json:",omitempty"`

	// Active is true if this is the current active CA. This must only
	// be true for exactly one CA. For any method that modifies roots in the
	// state store, tests should be written to verify that multiple roots
	// cannot be active.
	Active bool

	// RotatedOutAt is the time at which this CA was removed from the state.
	// This will only be set on roots that have been rotated out from being the
	// active root.
	RotatedOutAt time.Time `json:"-"`

	// PrivateKeyType is the type of the private key used to sign certificates. It
	// may be "rsa" or "ec". This is provided as a convenience to avoid parsing
	// the public key to from the certificate to infer the type.
	PrivateKeyType string

	// PrivateKeyBits is the length of the private key used to sign certificates.
	// This is provided as a convenience to avoid parsing the public key from the
	// certificate to infer the type.
	PrivateKeyBits int

	RaftIndex
}

func (c *CARoot) Clone() *CARoot {
	if c == nil {
		return nil
	}

	newCopy := *c
	newCopy.IntermediateCerts = stringslice.CloneStringSlice(c.IntermediateCerts)
	return &newCopy
}

// CARoots is a list of CARoot structures.
type CARoots []*CARoot

// Active returns the single CARoot that is marked as active, or nil if there
// is no active root (ex: when they are no roots).
func (c CARoots) Active() *CARoot {
	if c == nil {
		return nil
	}
	for _, r := range c {
		if r.Active {
			return r
		}
	}
	return nil
}

// CASignRequest is the request for signing a service certificate.
type CASignRequest struct {
	// Datacenter is the target for this request.
	Datacenter string

	// CSR is the PEM-encoded CSR.
	CSR string

	// WriteRequest is a common struct containing ACL tokens and other
	// write-related common elements for requests.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (q *CASignRequest) RequestDatacenter() string {
	return q.Datacenter
}

// IssuedCert is a certificate that has been issued by a Connect CA.
type IssuedCert struct {
	// SerialNumber is the unique serial number for this certificate.
	// This is encoded in standard hex separated by :.
	SerialNumber string

	// CertPEM is a PEM encoded bundle of a leaf certificate, optionally followed
	// by one or more intermediate certificates that will form a chain of trust
	// back to a root CA.
	//
	// This field is not persisted in the state store, but is present in the
	// sign API response.
	CertPEM string `json:",omitempty"`
	// PrivateKeyPEM is the PEM encoded private key associated with CertPEM.
	PrivateKeyPEM string `json:",omitempty"`

	// Service is the name of the service for which the cert was issued.
	Service string `json:",omitempty"`
	// ServiceURI is the cert URI value.
	ServiceURI string `json:",omitempty"`

	// Agent is the name of the node for which the cert was issued.
	Agent string `json:",omitempty"`
	// AgentURI is the cert URI value.
	AgentURI string `json:",omitempty"`

	// ServerURI is the URI value of a cert issued for a server agent.
	// The same URI is shared by all servers in a Consul datacenter.
	ServerURI string `json:",omitempty"`

	// Kind is the kind of service for which the cert was issued.
	Kind ServiceKind `json:",omitempty"`
	// KindURI is the cert URI value.
	KindURI string `json:",omitempty"`

	// ValidAfter and ValidBefore are the validity periods for the
	// certificate.
	ValidAfter  time.Time
	ValidBefore time.Time

	// EnterpriseMeta is the Consul Enterprise specific metadata
	acl.EnterpriseMeta

	RaftIndex
}

// CAOp is the operation for a request related to intentions.
type CAOp string

const (
	CAOpSetRoots                      CAOp = "set-roots"
	CAOpSetConfig                     CAOp = "set-config"
	CAOpSetProviderState              CAOp = "set-provider-state"
	CAOpDeleteProviderState           CAOp = "delete-provider-state"
	CAOpSetRootsAndConfig             CAOp = "set-roots-config"
	CAOpIncrementProviderSerialNumber CAOp = "increment-provider-serial"
)

// CARequest is used to modify connect CA data. This is used by the
// FSM (agent/consul/fsm) to apply changes.
type CARequest struct {
	// Op is the type of operation being requested. This determines what
	// other fields are required.
	Op CAOp

	// Datacenter is the target for this request.
	Datacenter string

	// Index is used by CAOpSetRoots and CAOpSetConfig for a CAS operation.
	Index uint64

	// Roots is a list of roots. This is used for CAOpSet. One root must
	// always be active.
	Roots []*CARoot

	// Config is the configuration for the current CA plugin.
	Config *CAConfiguration

	// ProviderState is the state for the builtin CA provider.
	ProviderState *CAConsulProviderState

	// WriteRequest is a common struct containing ACL tokens and other
	// write-related common elements for requests.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (q *CARequest) RequestDatacenter() string {
	return q.Datacenter
}

const (
	ConsulCAProvider = "consul"
	VaultCAProvider  = "vault"
	AWSCAProvider    = "aws-pca"
)

// CAConfiguration is the configuration for the current CA plugin.
type CAConfiguration struct {
	// ClusterID is a unique identifier for the cluster
	ClusterID string `json:"-"`

	// Provider is the CA provider implementation to use.
	Provider string

	// Configuration is arbitrary configuration for the provider. This
	// should only contain primitive values and containers (such as lists
	// and maps).
	Config map[string]interface{}

	// State is optionally used by the provider to persist information it needs
	// between reloads like UUIDs of resources it manages. It only supports string
	// values to avoid gotchas with interface{} since this is encoded through
	// msgpack when it's written through raft. For example if providers used a
	// custom struct or even a simple `int` type, msgpack with loose type
	// information during encode/decode and providers will end up getting back
	// different types have have to remember to test multiple variants of state
	// handling to account for cases where it's been through msgpack or not.
	// Keeping this as strings only forces compatibility and leaves the input
	// Providers have to work with unambiguous - they can parse ints or other
	// types as they need. We expect this only to be used to store a handful of
	// identifiers anyway so this is simpler.
	State map[string]string

	// ForceWithoutCrossSigning indicates that the CA reconfiguration should go
	// ahead even if the current CA is unable to cross sign certificates. This
	// risks temporary connection failures during the rollout as new leafs will be
	// rejected by proxies that have not yet observed the new root cert but is the
	// only option if a CA that doesn't support cross signing needs to be
	// reconfigured or mirated away from.
	ForceWithoutCrossSigning bool

	RaftIndex
}

func (c *CAConfiguration) UnmarshalJSON(data []byte) (err error) {
	type Alias CAConfiguration

	aux := &struct {
		ForceWithoutCrossSigningSnake bool `json:"force_without_cross_signing"`

		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if aux.ForceWithoutCrossSigningSnake {
		c.ForceWithoutCrossSigning = aux.ForceWithoutCrossSigningSnake
	}

	return nil
}

func (c *CAConfiguration) GetCommonConfig() (*CommonCAProviderConfig, error) {
	if c == nil {
		return nil, fmt.Errorf("config map was nil")
	}

	var config CommonCAProviderConfig

	// Set Defaults
	config.CSRMaxPerSecond = 50 // See doc comment for rationale here.

	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook:       ParseDurationFunc(),
		Result:           &config,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(c.Config); err != nil {
		return nil, fmt.Errorf("error decoding config: %s", err)
	}

	return &config, nil
}

type CommonCAProviderConfig struct {
	LeafCertTTL time.Duration
	RootCertTTL time.Duration

	// IntermediateCertTTL is only valid in the primary datacenter, and determines
	// the duration that any signed intermediates are valid for.
	IntermediateCertTTL time.Duration

	SkipValidate bool

	// CSRMaxPerSecond is a rate limit on processing Connect Certificate Signing
	// Requests on the servers. It applies to all CA providers so can be used to
	// limit rate to an external CA too. 0 disables the rate limit. Defaults to 50
	// which is low enough to prevent overload of a reasonably sized production
	// server while allowing a cluster with 1000 service instances to complete a
	// rotation in 20 seconds. For reference a quad-core 2017 MacBook pro can
	// process 100 signing RPCs a second while using less than half of one core.
	// For large clusters with powerful servers it's advisable to increase this
	// rate or to disable this limit and instead rely on CSRMaxConcurrent to only
	// consume a subset of the server's cores.
	CSRMaxPerSecond float32

	// CSRMaxConcurrent is a limit on how many concurrent CSR signing requests
	// will be processed in parallel. New incoming signing requests will try for
	// `consul.csrSemaphoreWait` (currently 500ms) for a slot before being
	// rejected with a "rate limited" backpressure response. This effectively sets
	// how many CPU cores can be occupied by Connect CA signing activity and
	// should be a (small) subset of your server's available cores to allow other
	// tasks to complete when a barrage of CSRs come in (e.g. after a CA root
	// rotation). Setting to 0 disables the limit, attempting to sign certs
	// immediately in the RPC goroutine. This is 0 by default and CSRMaxPerSecond
	// is used. This is ignored if CSRMaxPerSecond is non-zero.
	CSRMaxConcurrent int

	// PrivateKeyType specifies which type of key the CA should generate. It only
	// applies when the provider is generating its own key and is ignored if the
	// provider already has a key or an external key is provided. Supported values
	// are "ec" or "rsa". "ec" is the default and will generate a NIST P-256
	// Elliptic key.
	PrivateKeyType string

	// PrivateKeyBits specifies the number of bits the CA's private key should
	// use. For RSA, supported values are 2048 and 4096. For EC, supported values
	// are 224, 256, 384 and 521 and correspond to the NIST P-* curve of the same
	// name. As with PrivateKeyType this is only relevant whan the provier is
	// generating new CA keys (root or intermediate).
	PrivateKeyBits int
}

var MinLeafCertTTL = time.Hour
var MaxLeafCertTTL = 365 * 24 * time.Hour

// intermediateCertRenewInterval is the interval at which the expiration
// of the intermediate cert is checked and renewed if necessary.
var IntermediateCertRenewInterval = time.Hour

func (c CommonCAProviderConfig) Validate() error {
	if c.SkipValidate {
		return nil
	}

	// it's sufficient to check that the root cert ttl >= intermediate cert ttl
	// since intermediate cert ttl >= 3* leaf cert ttl; so root cert ttl >= 3 * leaf cert ttl > leaf cert ttl
	if c.RootCertTTL < c.IntermediateCertTTL {
		return fmt.Errorf("root cert TTL is set and is not greater than intermediate cert ttl. root cert ttl: %s, intermediate cert ttl: %s", c.RootCertTTL, c.IntermediateCertTTL)
	}

	if c.LeafCertTTL < MinLeafCertTTL {
		return fmt.Errorf("leaf cert TTL must be greater or equal than %s", MinLeafCertTTL)
	}

	if c.LeafCertTTL > MaxLeafCertTTL {
		return fmt.Errorf("leaf cert TTL must be less than %s", MaxLeafCertTTL)
	}

	if c.IntermediateCertTTL < (3 * IntermediateCertRenewInterval) {
		// Intermediate Certificates are checked every
		// hour(intermediateCertRenewInterval) if they are about to
		// expire. Recreating an intermediate certs is started once
		// more than half its lifetime has passed.
		// If it would be 2h, worst case is that the check happens
		// right before half time and when the check happens again, the
		// certificate is very close to expiring, leaving only a small
		// timeframe to renew. 3h leaves more than 30min to recreate.
		// Right now the minimum LeafCertTTL is 1h, which means this
		// check not strictly needed, because the same thing is covered
		// in the next check too. But just in case minimum LeafCertTTL
		// changes at some point, this validation must still be
		// performed.
		return fmt.Errorf("Intermediate Cert TTL must be greater or equal than %dh", 3*int(IntermediateCertRenewInterval.Hours()))
	}
	if c.IntermediateCertTTL < (3 * c.LeafCertTTL) {
		// Intermediate Certificates are being sent to the proxy when
		// the Leaf Certificate changes because they are bundled
		// together.
		// That means that the Intermediate Certificate TTL must be at
		// a minimum of 3 * Leaf Certificate TTL to ensure that the new
		// Intermediate is being set together with the Leaf Certificate
		// before it expires.
		return fmt.Errorf("Intermediate Cert TTL must be greater or equal than 3 * LeafCertTTL (>=%s).", 3*c.LeafCertTTL)
	}

	switch c.PrivateKeyType {
	case "ec":
		if c.PrivateKeyBits != 224 && c.PrivateKeyBits != 256 && c.PrivateKeyBits != 384 && c.PrivateKeyBits != 521 {
			return fmt.Errorf("EC key length must be one of (224, 256, 384, 521) bits")
		}
	case "rsa":
		if c.PrivateKeyBits != 2048 && c.PrivateKeyBits != 4096 {
			return fmt.Errorf("RSA key length must be 2048 or 4096 bits")
		}
	default:
		return fmt.Errorf("private key type must be either 'ec' or 'rsa'")
	}

	return nil
}

type ConsulCAProviderConfig struct {
	CommonCAProviderConfig `mapstructure:",squash"`

	PrivateKey string
	RootCert   string

	// DisableCrossSigning is really only useful in test code to use the built in
	// provider while exercising logic that depends on the CA provider ability to
	// cross sign. We don't document this config field publicly or make any
	// attempt to parse it from snake case unlike other fields here.
	DisableCrossSigning bool
}

func (c *ConsulCAProviderConfig) Validate() error {
	return nil
}

// CAConsulProviderState is used to track the built-in Consul CA provider's state.
type CAConsulProviderState struct {
	ID               string
	PrivateKey       string
	RootCert         string
	IntermediateCert string

	RaftIndex
}

type VaultCAProviderConfig struct {
	CommonCAProviderConfig `mapstructure:",squash"`

	Address                  string
	Token                    string
	RootPKIPath              string
	RootPKINamespace         string
	IntermediatePKIPath      string
	IntermediatePKINamespace string
	Namespace                string

	CAFile        string
	CAPath        string
	CertFile      string
	KeyFile       string
	TLSServerName string
	TLSSkipVerify bool

	AuthMethod *VaultAuthMethod `alias:"auth_method"`
}

type VaultAuthMethod struct {
	Type      string
	MountPath string `alias:"mount_path"`
	Params    map[string]interface{}
}

type AWSCAProviderConfig struct {
	CommonCAProviderConfig `mapstructure:",squash"`

	ExistingARN  string
	DeleteOnExit bool
}

// CALeafOp is the operation for a request related to leaf certificates.
type CALeafOp string

const (
	CALeafOpIncrementIndex CALeafOp = "increment-index"
)

// CALeafRequest is used to modify connect CA leaf data. This is used by the
// FSM (agent/consul/fsm) to apply changes.
type CALeafRequest struct {
	// Op is the type of operation being requested. This determines what
	// other fields are required.
	Op CALeafOp

	// Datacenter is the target for this request.
	Datacenter string

	// WriteRequest is a common struct containing ACL tokens and other
	// write-related common elements for requests.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (q *CALeafRequest) RequestDatacenter() string {
	return q.Datacenter
}

// ParseDurationFunc is a mapstructure hook for decoding a string or
// []uint8 into a time.Duration value.
func ParseDurationFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		var v time.Duration
		if t != reflect.TypeOf(v) {
			return data, nil
		}

		switch {
		case f.Kind() == reflect.String:
			if dur, err := time.ParseDuration(data.(string)); err != nil {
				return nil, err
			} else {
				v = dur
			}
			return v, nil
		case f == reflect.SliceOf(reflect.TypeOf(uint8(0))):
			s := Uint8ToString(data.([]uint8))
			if dur, err := time.ParseDuration(s); err != nil {
				return nil, err
			} else {
				v = dur
			}
			return v, nil
		default:
			return data, nil
		}
	}
}

func Uint8ToString(bs []uint8) string {
	b := make([]byte, len(bs))
	for i, v := range bs {
		b[i] = v
	}
	return string(b)
}

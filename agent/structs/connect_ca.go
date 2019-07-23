package structs

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
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

// CARoot represents a root CA certificate that is trusted.
type CARoot struct {
	// ID is a globally unique ID (UUID) representing this CA root.
	ID string

	// Name is a human-friendly name for this CA root. This value is
	// opaque to Consul and is not used for anything internally.
	Name string

	// SerialNumber is the x509 serial number of the certificate.
	SerialNumber uint64

	// SigningKeyID is the ID of the public key that corresponds to the private
	// key used to sign the certificate. Is is the HexString format of the raw
	// AuthorityKeyID bytes.
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

	// Time validity bounds.
	NotBefore time.Time
	NotAfter  time.Time

	// RootCert is the PEM-encoded public certificate.
	RootCert string

	// IntermediateCerts is a list of PEM-encoded intermediate certs to
	// attach to any leaf certs signed by this CA.
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

	// Type of private key used to create the CA cert.
	PrivateKeyType string
	PrivateKeyBits int

	RaftIndex
}

// CARoots is a list of CARoot structures.
type CARoots []*CARoot

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

	// CertPEM and PrivateKeyPEM are the PEM-encoded certificate and private
	// key for that cert, respectively. This should not be stored in the
	// state store, but is present in the sign API response.
	CertPEM       string `json:",omitempty"`
	PrivateKeyPEM string `json:",omitempty"`

	// Service is the name of the service for which the cert was issued.
	// ServiceURI is the cert URI value.
	Service    string `json:",omitempty"`
	ServiceURI string `json:",omitempty"`

	// Agent is the name of the node for which the cert was issued.
	// AgentURI is the cert URI value.
	Agent    string `json:",omitempty"`
	AgentURI string `json:",omitempty"`

	// ValidAfter and ValidBefore are the validity periods for the
	// certificate.
	ValidAfter  time.Time
	ValidBefore time.Time

	RaftIndex
}

// CAOp is the operation for a request related to intentions.
type CAOp string

const (
	CAOpSetRoots            CAOp = "set-roots"
	CAOpSetConfig           CAOp = "set-config"
	CAOpSetProviderState    CAOp = "set-provider-state"
	CAOpDeleteProviderState CAOp = "delete-provider-state"
	CAOpSetRootsAndConfig   CAOp = "set-roots-config"
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

	RaftIndex
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

	PrivateKeyType string
	PrivateKeyBits int
}

func (c CommonCAProviderConfig) Validate() error {
	if c.SkipValidate {
		return nil
	}

	if c.LeafCertTTL < time.Hour {
		return fmt.Errorf("leaf cert TTL must be greater than 1h")
	}

	if c.LeafCertTTL > 365*24*time.Hour {
		return fmt.Errorf("leaf cert TTL must be less than 1 year")
	}

	switch c.PrivateKeyType {
	case "ec":
		if c.PrivateKeyBits != 224 && c.PrivateKeyBits != 256 && c.PrivateKeyBits != 384 && c.PrivateKeyBits != 521 {
			return fmt.Errorf("ECDSA key length must be one of (224, 256, 384, 521) bits")
		}
	case "rsa":
		if c.PrivateKeyBits != 2048 && c.PrivateKeyBits != 4096 {
			return fmt.Errorf("RSA key length must be 2048 or 4096 bits")
		}
	default:
		return fmt.Errorf("private key type must be either 'ecdsa' or 'rsa'")
	}

	return nil
}

type ConsulCAProviderConfig struct {
	CommonCAProviderConfig `mapstructure:",squash"`

	PrivateKey     string
	RootCert       string
	RotationPeriod time.Duration
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

	Address             string
	Token               string
	RootPKIPath         string
	IntermediatePKIPath string

	CAFile        string
	CAPath        string
	CertFile      string
	KeyFile       string
	TLSServerName string
	TLSSkipVerify bool
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
		b[i] = byte(v)
	}
	return string(b)
}

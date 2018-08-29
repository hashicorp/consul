package structs

import (
	"fmt"
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
	// NOTE(banks): Later we may support explicitly trusting external domains
	// which may be encoded into the CARoot struct or a separate list but this
	// domain identifier should be immutable and cluster-wide so deserves to be at
	// the root of this response rather than duplicated through all CARoots that
	// are not externally trusted entities.
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

	// SigningKeyID is the ID of the public key that corresponds to the
	// private key used to sign the certificate.
	SigningKeyID string

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
	Service    string
	ServiceURI string

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
	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
		Result:     &config,
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
	ID         string
	PrivateKey string
	RootCert   string

	RaftIndex
}

type VaultCAProviderConfig struct {
	CommonCAProviderConfig `mapstructure:",squash"`

	Address             string
	Token               string
	RootPKIPath         string
	IntermediatePKIPath string
}

package structs

import (
	"math/big"
	"time"
)

// IndexedCARoots is the list of currently trusted CA Roots.
type IndexedCARoots struct {
	// ActiveRootID is the ID of a root in Roots that is the active CA root.
	// Other roots are still valid if they're in the Roots list but are in
	// the process of being rotated out.
	ActiveRootID string

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

	// RootCert is the PEM-encoded public certificate.
	RootCert string

	// SigningCert is the PEM-encoded signing certificate and SigningKey
	// is the PEM-encoded private key for the signing certificate. These
	// may actually be empty if the CA plugin in use manages these for us.
	SigningCert string
	SigningKey  string

	// Active is true if this is the current active CA. This must only
	// be true for exactly one CA. For any method that modifies roots in the
	// state store, tests should be written to verify that multiple roots
	// cannot be active.
	Active bool

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
	SerialNumber *big.Int

	// CertPEM and PrivateKeyPEM are the PEM-encoded certificate and private
	// key for that cert, respectively. This should not be stored in the
	// state store, but is present in the sign API response.
	CertPEM       string `json:",omitempty"`
	PrivateKeyPEM string

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
	CAOpSet CAOp = "set"
)

// CARequest is used to modify connect CA data. This is used by the
// FSM (agent/consul/fsm) to apply changes.
type CARequest struct {
	// Op is the type of operation being requested. This determines what
	// other fields are required.
	Op CAOp

	// Index is used by CAOpSet for a CAS operation.
	Index uint64

	// Roots is a list of roots. This is used for CAOpSet. One root must
	// always be active.
	Roots []*CARoot
}

package structs

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

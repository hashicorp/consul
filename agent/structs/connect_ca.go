package structs

// IndexedCARoots is the list of currently trusted CA Roots.
type IndexedCARoots struct {
	// ActiveRootID is the ID of a root in Roots that is the active CA root.
	// Other roots are still valid if they're in the Roots list but are in
	// the process of being rotated out.
	ActiveRootID string

	// Roots is a list of root CA certs to trust.
	Roots []*CARoot

	QueryMeta
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
	// is the PEM-encoded private key for the signing certificate.
	SigningCert string
	SigningKey  string

	RaftIndex
}

// CARoots is a list of CARoot structures.
type CARoots []*CARoot

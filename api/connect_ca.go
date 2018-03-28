package api

import (
	"time"
)

// CARootList is the structure for the results of listing roots.
type CARootList struct {
	ActiveRootID string
	Roots        []*CARoot
}

// CARoot represents a root CA certificate that is trusted.
type CARoot struct {
	// ID is a globally unique ID (UUID) representing this CA root.
	ID string

	// Name is a human-friendly name for this CA root. This value is
	// opaque to Consul and is not used for anything internally.
	Name string

	// RootCertPEM is the PEM-encoded public certificate.
	RootCertPEM string `json:"RootCert"`

	// Active is true if this is the current active CA. This must only
	// be true for exactly one CA. For any method that modifies roots in the
	// state store, tests should be written to verify that multiple roots
	// cannot be active.
	Active bool

	CreateIndex uint64
	ModifyIndex uint64
}

// LeafCert is a certificate that has been issued by a Connect CA.
type LeafCert struct {
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

	CreateIndex uint64
	ModifyIndex uint64
}

// CARoots queries the list of available roots.
func (h *Connect) CARoots(q *QueryOptions) (*CARootList, *QueryMeta, error) {
	r := h.c.newRequest("GET", "/v1/connect/ca/roots")
	r.setQueryOptions(q)
	rtt, resp, err := requireOK(h.c.doRequest(r))
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	qm := &QueryMeta{}
	parseQueryMeta(resp, qm)
	qm.RequestTime = rtt

	var out CARootList
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}
	return &out, qm, nil
}

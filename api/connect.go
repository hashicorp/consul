package api

import (
	"time"
)

// CARootList is the structure for the results of listing roots.
type CARootList struct {
	ActiveRootID string
	Roots        []*CARoot
}

// CARoot is a single CA within Connect.
type CARoot struct {
	ID          string
	Name        string
	RootCert    string
	Active      bool
	CreateIndex uint64
	ModifyIndex uint64
}

type IssuedCert struct {
	SerialNumber  string
	CertPEM       string
	PrivateKeyPEM string
	Service       string
	ServiceURI    string
	ValidAfter    time.Time
	ValidBefore   time.Time
	CreateIndex   uint64
	ModifyIndex   uint64
}

// Connect can be used to work with endpoints related to Connect, the
// feature for securely connecting services within Consul.
type Connect struct {
	c *Client
}

// Health returns a handle to the health endpoints
func (c *Client) Connect() *Connect {
	return &Connect{c}
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

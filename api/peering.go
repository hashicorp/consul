package api

import (
	"context"
	"fmt"
)

// PeeringState enumerates all the states a peering can be in
type PeeringState string

const (
	// PeeringStateUndefined represents an unset value for PeeringState during
	// writes.
	PeeringStateUndefined PeeringState = "UNDEFINED"

	// PeeringStateInitial means a Peering has been initialized and is awaiting
	// acknowledgement from a remote peer.
	PeeringStateInitial PeeringState = "INITIAL"

	// PeeringStateActive means that the peering connection is active and
	// healthy.
	PeeringStateActive PeeringState = "ACTIVE"

	// PeeringStateFailing means the peering connection has been interrupted
	// but has not yet been terminated.
	PeeringStateFailing PeeringState = "FAILING"

	// PeeringStateTerminated means the peering relationship has been removed.
	PeeringStateTerminated PeeringState = "TERMINATED"
)

type Peering struct {
	// ID is a datacenter-scoped UUID for the peering.
	ID string
	// Name is the local alias for the peering relationship.
	Name string
	// Partition is the local partition connecting to the peer.
	Partition string `json:",omitempty"`
	// Meta is a mapping of some string value to any other string value
	Meta map[string]string `json:",omitempty"`
	// State is one of the valid PeeringState values to represent the status of
	// peering relationship.
	State PeeringState
	// PeerID is the ID that our peer assigned to this peering. This ID is to
	// be used when dialing the peer, so that it can know who dialed it.
	PeerID string `json:",omitempty"`
	// PeerCAPems contains all the CA certificates for the remote peer.
	PeerCAPems []string `json:",omitempty"`
	// PeerServerName is the name of the remote server as it relates to TLS.
	PeerServerName string `json:",omitempty"`
	// PeerServerAddresses contains all the connection addresses for the remote peer.
	PeerServerAddresses []string `json:",omitempty"`
	// CreateIndex is the Raft index at which the Peering was created.
	CreateIndex uint64
	// ModifyIndex is the latest Raft index at which the Peering. was modified.
	ModifyIndex uint64
}

type PeeringReadResponse struct {
	Peering *Peering
}

type PeeringGenerateTokenRequest struct {
	// PeerName is the name of the remote peer.
	PeerName string
	// Partition to be peered.
	Partition  string `json:",omitempty"`
	Datacenter string `json:",omitempty"`
	Token      string `json:",omitempty"`
	// Meta is a mapping of some string value to any other string value
	Meta map[string]string `json:",omitempty"`
}

type PeeringGenerateTokenResponse struct {
	// PeeringToken is an opaque string provided to the remote peer for it to complete
	// the peering initialization handshake.
	PeeringToken string
}

type PeeringInitiateRequest struct {
	// Name of the remote peer.
	PeerName string
	// The peering token returned from the peer's GenerateToken endpoint.
	PeeringToken string `json:",omitempty"`
	Datacenter   string `json:",omitempty"`
	Token        string `json:",omitempty"`
	// Meta is a mapping of some string value to any other string value
	Meta map[string]string `json:",omitempty"`
}

type PeeringInitiateResponse struct {
}

type PeeringListRequest struct {
	// future proofing in case we extend List functionality
}

type Peerings struct {
	c *Client
}

// Peerings returns a handle to the operator endpoints.
func (c *Client) Peerings() *Peerings {
	return &Peerings{c: c}
}

func (p *Peerings) Read(ctx context.Context, name string, q *QueryOptions) (*Peering, *QueryMeta, error) {
	if name == "" {
		return nil, nil, fmt.Errorf("peering name cannot be empty")
	}

	req := p.c.newRequest("GET", fmt.Sprintf("/v1/peering/%s", name))
	req.setQueryOptions(q)
	req.ctx = ctx

	rtt, resp, err := p.c.doRequest(req)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	found, resp, err := requireNotFoundOrOK(resp)
	if err != nil {
		return nil, nil, err
	}

	qm := &QueryMeta{}
	parseQueryMeta(resp, qm)
	qm.RequestTime = rtt

	if !found {
		return nil, qm, nil
	}

	var out Peering
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}

	return &out, qm, nil
}

func (p *Peerings) Delete(ctx context.Context, name string, q *WriteOptions) (*WriteMeta, error) {
	if name == "" {
		return nil, fmt.Errorf("peering name cannot be empty")
	}

	req := p.c.newRequest("DELETE", fmt.Sprintf("/v1/peering/%s", name))
	req.setWriteOptions(q)
	req.ctx = ctx

	rtt, resp, err := p.c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, err
	}

	wm := &WriteMeta{RequestTime: rtt}
	return wm, nil
}

// TODO(peering): verify this is the ultimate signature we want
func (p *Peerings) GenerateToken(ctx context.Context, g PeeringGenerateTokenRequest, wq *WriteOptions) (*PeeringGenerateTokenResponse, *WriteMeta, error) {
	if g.PeerName == "" {
		return nil, nil, fmt.Errorf("peer name cannot be empty")
	}

	req := p.c.newRequest("POST", fmt.Sprint("/v1/peering/token"))
	req.setWriteOptions(wq)
	req.ctx = ctx
	req.obj = g

	rtt, resp, err := p.c.doRequest(req)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	wm := &WriteMeta{RequestTime: rtt}

	var out PeeringGenerateTokenResponse
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}

	return &out, wm, nil
}

// TODO(peering): verify this is the ultimate signature we want
func (p *Peerings) Initiate(ctx context.Context, i PeeringInitiateRequest, wq *WriteOptions) (*PeeringInitiateResponse, *WriteMeta, error) {
	req := p.c.newRequest("POST", fmt.Sprint("/v1/peering/initiate"))
	req.setWriteOptions(wq)
	req.ctx = ctx
	req.obj = i

	rtt, resp, err := p.c.doRequest(req)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	wm := &WriteMeta{RequestTime: rtt}

	var out PeeringInitiateResponse
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}

	return &out, wm, nil
}

func (p *Peerings) List(ctx context.Context, q *QueryOptions) ([]*Peering, *QueryMeta, error) {
	req := p.c.newRequest("GET", "/v1/peerings")
	req.setQueryOptions(q)
	req.ctx = ctx

	rtt, resp, err := p.c.doRequest(req)
	if err != nil {
		return nil, nil, err
	}
	defer closeResponseBody(resp)
	if err := requireOK(resp); err != nil {
		return nil, nil, err
	}

	qm := &QueryMeta{}
	parseQueryMeta(resp, qm)
	qm.RequestTime = rtt

	var out []*Peering
	if err := decodeBody(resp, &out); err != nil {
		return nil, nil, err
	}

	return out, qm, nil
}

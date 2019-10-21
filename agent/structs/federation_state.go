package structs

import (
	"sort"
	"time"
)

type FederationStateOp string

const (
	FederationStateUpsert FederationStateOp = "upsert"
	FederationStateDelete FederationStateOp = "delete"
)

type FederationStateRequest struct {
	Datacenter string
	Op         FederationStateOp
	State      *FederationState

	WriteRequest
}

func (c *FederationStateRequest) RequestDatacenter() string {
	return c.Datacenter
}

type FederationStates []*FederationState

func (listings FederationStates) Sort() {
	sort.Slice(listings, func(i, j int) bool {
		return listings[i].Datacenter < listings[j].Datacenter
	})
}

type FederationState struct {
	Datacenter         string
	MeshGateways       CheckServiceNodes `json:",omitempty"`
	UpdatedAt          time.Time
	PrimaryModifyIndex uint64 // raft data from the primary
	RaftIndex                 // local raft data
}

// TODO:
func (c *FederationState) IsSame(other *FederationState) bool {
	if c.Datacenter != other.Datacenter {
		return false
	}

	// We don't include the UpdatedAt field in this comparison because that is
	// only updated when we re-persist.

	if len(c.MeshGateways) != len(other.MeshGateways) {
		return false
	}

	// TODO: we don't bother to sort these since the order is going to be
	// already defined by how the catalog returns results which should be
	// stable enough.

	for i := 0; i < len(c.MeshGateways); i++ {
		a := c.MeshGateways[i]
		b := other.MeshGateways[i]

		if !a.Node.IsSame(b.Node) {
			return false
		}
		if !a.Service.IsSame(b.Service) {
			return false
		}

		if len(a.Checks) != len(b.Checks) {
			return false
		}

		for j := 0; j < len(a.Checks); j++ {
			ca := a.Checks[j]
			cb := b.Checks[j]

			if !ca.IsSame(cb) {
				return false
			}
		}
	}

	return true
}

type FederationStateQuery struct {
	Datacenter string

	TargetDatacenter string
	QueryOptions
}

type FederationStateResponse struct {
	State *FederationState
	QueryMeta
}

type IndexedFederationStates struct {
	States FederationStates
	QueryMeta
}

func (c *FederationStateQuery) RequestDatacenter() string {
	return c.TargetDatacenter
}

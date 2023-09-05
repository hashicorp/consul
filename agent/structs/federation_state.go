// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"sort"
	"time"
)

// FederationStateOp is the operation for a request related to federation states.
type FederationStateOp string

const (
	FederationStateUpsert FederationStateOp = "upsert"
	FederationStateDelete FederationStateOp = "delete"
)

// FederationStateRequest is used to upsert and delete federation states.
type FederationStateRequest struct {
	// Datacenter is the target for this request.
	Datacenter string

	// Op is the type of operation being requested.
	Op FederationStateOp

	// State is the federation state to upsert or in the case of a delete
	// only the State.Datacenter field should be set.
	State *FederationState

	// WriteRequest is a common struct containing ACL tokens and other
	// write-related common elements for requests.
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given request.
func (c *FederationStateRequest) RequestDatacenter() string {
	return c.Datacenter
}

// FederationStates is a list of federation states.
type FederationStates []*FederationState

// Sort sorts federation states by their datacenter.
func (listings FederationStates) Sort() {
	sort.Slice(listings, func(i, j int) bool {
		return listings[i].Datacenter < listings[j].Datacenter
	})
}

// FederationState defines some WAN federation related state that should be
// cross-shared between all datacenters joined on the WAN. One record exists
// per datacenter.
type FederationState struct {
	// Datacenter is the name of the datacenter.
	Datacenter string

	// MeshGateways is a snapshot of the catalog state for all mesh gateways in
	// this datacenter.
	MeshGateways CheckServiceNodes `json:",omitempty"`

	// UpdatedAt keeps track of when this record was modified.
	UpdatedAt time.Time

	// PrimaryModifyIndex is the ModifyIndex of the original data as it exists
	// in the primary datacenter.
	PrimaryModifyIndex uint64

	// RaftIndex is local raft data.
	RaftIndex
}

// IsSame is used to compare two federation states for the purposes of
// anti-entropy.
func (c *FederationState) IsSame(other *FederationState) bool {
	if c.Datacenter != other.Datacenter {
		return false
	}

	// We don't include the UpdatedAt field in this comparison because that is
	// only updated when we re-persist.

	if len(c.MeshGateways) != len(other.MeshGateways) {
		return false
	}

	// NOTE: we don't bother to sort these since the order is going to be
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

// FederationStateQuery is used to query federation states.
type FederationStateQuery struct {
	// Datacenter is the target this request is intended for.
	Datacenter string

	// TargetDatacenter is the name of a datacenter to fetch the federation state for.
	TargetDatacenter string

	// Options for queries
	QueryOptions
}

// RequestDatacenter returns the datacenter for a given request.
func (c *FederationStateQuery) RequestDatacenter() string {
	return c.TargetDatacenter
}

// FederationStateResponse is the response to a FederationStateQuery request.
type FederationStateResponse struct {
	State *FederationState
	QueryMeta
}

// IndexedFederationStates represents the list of all federation states.
type IndexedFederationStates struct {
	States FederationStates
	QueryMeta
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/api"
	multierror "github.com/hashicorp/go-multierror"
)

// TxnKVOp is used to define a single operation on the KVS inside a
// transaction.
type TxnKVOp struct {
	Verb   api.KVOp
	DirEnt DirEntry
}

// TxnKVResult is used to define the result of a single operation on the KVS
// inside a transaction.
type TxnKVResult *DirEntry

// TxnNodeOp is used to define a single operation on a node in the catalog inside
// a transaction.
type TxnNodeOp struct {
	Verb api.NodeOp
	Node Node
}

// TxnNodeResult is used to define the result of a single operation on a node
// in the catalog inside a transaction.
type TxnNodeResult *Node

// TxnServiceOp is used to define a single operation on a service in the catalog inside
// a transaction.
type TxnServiceOp struct {
	Verb    api.ServiceOp
	Node    string
	Service NodeService
}

// TxnServiceResult is used to define the result of a single operation on a service
// in the catalog inside a transaction.
type TxnServiceResult *NodeService

// TxnCheckOp is used to define a single operation on a health check inside a
// transaction.
type TxnCheckOp struct {
	Verb  api.CheckOp
	Check HealthCheck
}

// TxnCheckResult is used to define the result of a single operation on a
// session inside a transaction.
type TxnCheckResult *HealthCheck

// TxnSessionOp is used to define a single operation on a session inside a
// transaction.
type TxnSessionOp struct {
	Verb    api.SessionOp
	Session Session
}

// TxnIntentionOp is used to define a single operation on an Intention inside a
// transaction.
//
// Deprecated: see TxnOp.Intention description
type TxnIntentionOp IntentionRequest

// TxnOp is used to define a single operation inside a transaction. Only one
// of the types should be filled out per entry.
type TxnOp struct {
	KV      *TxnKVOp
	Node    *TxnNodeOp
	Service *TxnServiceOp
	Check   *TxnCheckOp
	Session *TxnSessionOp

	// Intention was an internal-only (not exposed in API or RPC)
	// implementation detail of legacy intention replication. This is
	// deprecated but retained for backwards compatibility with versions
	// of consul pre-dating 1.9.0. We need it for two reasons:
	//
	// 1. If a secondary DC is upgraded first, we need to continue to
	//    replicate legacy intentions UNTIL the primary DC is upgraded.
	//    Legacy intention replication exclusively writes using a TxnOp.
	// 2. If we attempt to reprocess raft-log contents pre-dating 1.9.0
	//    (such as when updating a secondary DC) we need to be able to
	//    recreate the state machine from the snapshot and whatever raft logs are
	//    present.
	Intention *TxnIntentionOp
}

// TxnOps is a list of operations within a transaction.
type TxnOps []*TxnOp

// TxnRequest is used to apply multiple operations to the state store in a
// single transaction
type TxnRequest struct {
	Datacenter string
	Ops        TxnOps
	WriteRequest
}

func (r *TxnRequest) RequestDatacenter() string {
	return r.Datacenter
}

// TxnReadRequest is used as a fast path for read-only transactions that don't
// modify the state store.
type TxnReadRequest struct {
	Datacenter string
	Ops        TxnOps
	QueryOptions
}

func (r *TxnReadRequest) RequestDatacenter() string {
	return r.Datacenter
}

// TxnError is used to return information about an error for a specific
// operation.
type TxnError struct {
	OpIndex int
	What    string
}

// Error returns the string representation of an atomic error.
func (e TxnError) Error() string {
	return fmt.Sprintf("op %d: %s", e.OpIndex, e.What)
}

// TxnErrors is a list of TxnError entries.
type TxnErrors []*TxnError

// TxnResult is used to define the result of a given operation inside a
// transaction. Only one of the types should be filled out per entry.
type TxnResult struct {
	KV      TxnKVResult      `json:",omitempty"`
	Node    TxnNodeResult    `json:",omitempty"`
	Service TxnServiceResult `json:",omitempty"`
	Check   TxnCheckResult   `json:",omitempty"`
}

// TxnResults is a list of TxnResult entries.
type TxnResults []*TxnResult

// TxnResponse is the structure returned by a TxnRequest.
type TxnResponse struct {
	Results TxnResults
	Errors  TxnErrors
}

// Error returns an aggregate of all errors in this TxnResponse.
func (r TxnResponse) Error() error {
	var errs error
	for _, err := range r.Errors {
		errs = multierror.Append(errs, errors.New(err.Error()))
	}
	return errs
}

// TxnReadResponse is the structure returned by a TxnReadRequest.
type TxnReadResponse struct {
	TxnResponse
	QueryMeta
}

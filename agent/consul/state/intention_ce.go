// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package state

import (
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func intentionListTxn(tx ReadTxn, _ *acl.EnterpriseMeta) (memdb.ResultIterator, error) {
	// Get all intentions
	return tx.Get(tableConnectIntentions, "id")
}

func getSimplifiedIntentions(
	tx ReadTxn,
	ws memdb.WatchSet,
	ixns structs.Intentions,
) (uint64, structs.Intentions, error) {
	return 0, ixns, nil
}

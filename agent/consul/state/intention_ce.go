// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
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

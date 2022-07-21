//go:build !consulent
// +build !consulent

package state

import (
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
)

func intentionListTxn(tx ReadTxn, _ *acl.EnterpriseMeta) (memdb.ResultIterator, error) {
	// Get all intentions
	return tx.Get(tableConnectIntentions, "id")
}

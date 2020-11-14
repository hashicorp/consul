// +build !consulent

package state

import (
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

func intentionListTxn(tx ReadTxn, _ *structs.EnterpriseMeta) (memdb.ResultIterator, error) {
	// Get all intentions
	return tx.Get(intentionsTableName, "id")
}

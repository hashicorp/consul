//go:build !consulent
// +build !consulent

package state

import (
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
)

func getCompoundWithTxn(tx ReadTxn, table, index string,
	_ *acl.EnterpriseMeta, idxVals ...interface{}) (memdb.ResultIterator, error) {

	return tx.Get(table, index, idxVals...)
}

// +build !consulent

package state

import (
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/structs"
)

func firstWithTxn(tx ReadTxn,
	table, index, idxVal string, entMeta *structs.EnterpriseMeta) (interface{}, error) {

	return tx.First(table, index, idxVal)
}

func firstWatchWithTxn(tx ReadTxn,
	table, index, idxVal string, entMeta *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {

	return tx.FirstWatch(table, index, idxVal)
}

func getWithTxn(tx ReadTxn,
	table, index, idxVal string, entMeta *structs.EnterpriseMeta) (memdb.ResultIterator, error) {

	return tx.Get(table, index, idxVal)
}

func getCompoundWithTxn(tx ReadTxn, table, index string,
	_ *structs.EnterpriseMeta, idxVals ...interface{}) (memdb.ResultIterator, error) {

	return tx.Get(table, index, idxVals...)
}

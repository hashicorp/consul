// +build !consulent

package state

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

func firstWithTxn(tx ReadTxn,
	table, index, idxVal string, entMeta *structs.EnterpriseMeta) (interface{}, error) {

	return tx.First(table, index, idxVal)
}

func firstWatchWithTxn(tx ReadTxn,
	table, index, idxVal string, entMeta *structs.EnterpriseMeta) (<-chan struct{}, interface{}, error) {

	return tx.FirstWatch(table, index, idxVal)
}

func firstWatchCompoundWithTxn(tx ReadTxn,
	table, index string, _ *structs.EnterpriseMeta, idxVals ...interface{}) (<-chan struct{}, interface{}, error) {
	return tx.FirstWatch(table, index, idxVals...)
}

func getWithTxn(tx ReadTxn,
	table, index, idxVal string, entMeta *structs.EnterpriseMeta) (memdb.ResultIterator, error) {

	return tx.Get(table, index, idxVal)
}

func getCompoundWithTxn(tx ReadTxn, table, index string,
	_ *structs.EnterpriseMeta, idxVals ...interface{}) (memdb.ResultIterator, error) {

	return tx.Get(table, index, idxVals...)
}

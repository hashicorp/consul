// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package inmem

import "github.com/hashicorp/go-memdb"

type meta struct {
	Key   string
	Value any
}

func incrementEventIndex(tx *memdb.Txn) (uint64, error) {
	idx, err := currentEventIndex(tx)
	if err != nil {
		return 0, err
	}

	idx++
	if err := tx.Insert(tableNameMetadata, meta{Key: metaKeyEventIndex, Value: idx}); err != nil {
		return 0, nil
	}
	return idx, nil
}

func currentEventIndex(tx *memdb.Txn) (uint64, error) {
	v, err := tx.First(tableNameMetadata, indexNameID, metaKeyEventIndex)
	if err != nil {
		return 0, err
	}
	if v == nil {
		// 0 and 1 index are reserved for special use in the stream package.
		return 2, nil
	}
	return v.(meta).Value.(uint64), nil
}

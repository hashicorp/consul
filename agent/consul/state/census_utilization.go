// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

const censusTable = "census_snapshots"

type CensusSnapshot struct {
	ID          string
	TS          time.Time
	TSUnix      int64
	Data        []byte
	CreateIndex uint64
	ModifyIndex uint64
}

func censusTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: censusTable,
		Indexes: map[string]*memdb.IndexSchema{
			"id":     {Name: "id", Unique: true, Indexer: &memdb.StringFieldIndex{Field: "ID"}},
			"ts":     {Name: "ts", Unique: false, Indexer: &memdb.IntFieldIndex{Field: "TSUnix"}},
			"create": {Name: "create", Unique: false, Indexer: &memdb.UintFieldIndex{Field: "CreateIndex"}},
			"modify": {Name: "modify", Unique: false, Indexer: &memdb.UintFieldIndex{Field: "ModifyIndex"}},
		},
	}
}

// Register in the DB schema builder:
//   Tables[censusTable] = censusTableSchema()

// Write: insert one snapshot at raft index idx.
func (s *Store) CensusPut(idx uint64, req *structs.CensusRequest) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()
	row := &CensusSnapshot{
		ID:          req.ID,
		TS:          req.TS.UTC(),
		TSUnix:      req.TS.UTC().Unix(),
		Data:        append([]byte(nil), req.Data...),
		CreateIndex: idx,
		ModifyIndex: idx,
	}

	if err := tx.Insert(censusTable, row); err != nil {
		return err
	}
	if err := tx.Insert(tableIndex, &IndexEntry{censusTable, idx}); err != nil {
		return err
	}
	return tx.Commit()
}

// Read: list all (already pruned, so this is “everything we keep”).
func (s *Store) CensusListAll() (uint64, []*CensusSnapshot, error) {
	txn := s.db.ReadTxn()
	defer txn.Abort()
	it, err := txn.Get(censusTable, "ts")
	if err != nil {
		return 0, nil, err
	}
	var out []*CensusSnapshot
	for obj := it.Next(); obj != nil; obj = it.Next() {
		out = append(out, obj.(*CensusSnapshot))
	}
	return s.maxIndex(censusTable), out, nil
}

// Write: prune anything with TS < cutoff. Returns count.
func (s *Store) CensusPrune(idx uint64, cutoff time.Time) (int, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()
	cut := cutoff.UTC().Unix()

	it, err := tx.Get(censusTable, "ts")
	if err != nil {
		return 0, err
	}

	c := 0
	for obj := it.Next(); obj != nil; obj = it.Next() {
		row := obj.(*CensusSnapshot)
		if row.TSUnix < cut {
			if err := tx.Delete(censusTable, row); err != nil {
				return c, err
			}
			c++
		} else {
			// rows are ordered by TS; once we reach >= cut we can bail if desired
			break
		}
	}

	if err := tx.Insert(tableIndex, &IndexEntry{censusTable, idx}); err != nil {
		return c, err
	}
	return c, tx.Commit()
}

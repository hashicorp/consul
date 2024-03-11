// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/raft"

	raftstorage "github.com/hashicorp/consul/internal/storage/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

type consulFSM interface {
	raft.FSM
	State() *state.Store
	Restore(io.ReadCloser) error
	RestoreWithFilter(io.ReadCloser) error
}

type FSMHelper struct {
	path string

	logger hclog.Logger

	// consul state
	store *raftboltdb.BoltStore
	snaps *raft.FileSnapshotStore
	fsm   consulFSM

	// raft
	logFirstIdx uint64
	logLastIdx  uint64
	nextIdx     uint64
}

func NewFSM(p string) (*FSMHelper, error) {
	store, firstIdx, lastIdx, err := RaftStateInfo(filepath.Join(p, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to open raft database %v: %v", p, err)
	}

	logger := hclog.L()

	snaps, err := raft.NewFileSnapshotStoreWithLogger(p, 1000, logger)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to open snapshot dir: %v", err)
	}

	fsm, err := dummyFSM(logger)
	if err != nil {
		store.Close()
		return nil, err
	}

	return &FSMHelper{
		path:   p,
		logger: logger,
		store:  store,
		fsm:    fsm,
		snaps:  snaps,

		logFirstIdx: firstIdx,
		logLastIdx:  lastIdx,
		nextIdx:     uint64(1),
	}, nil
}

func dummyFSM(logger hclog.Logger) (consulFSM, error) {
	// It's safe to pass nil as the handle argument here because we won't call
	// the backend's data access methods (only Apply, Snapshot, and Restore).
	backend, err := raftstorage.NewBackend(nil, hclog.NewNullLogger())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go backend.Run(ctx)

	return fsm.NewFromDeps(fsm.Deps{
		Logger: logger,
		NewStateStore: func() *state.Store {
			return state.NewStateStore(nil)
		},
		StorageBackend: backend,
	}), nil
}

func (f *FSMHelper) Close() {
	f.store.Close()
}

func (f *FSMHelper) State() *state.Store {
	return f.fsm.State()
}

func (f *FSMHelper) StateAsMap() map[string][]interface{} {
	return StateAsMap(f.fsm.State())
}

// StateAsMap converts Consul's state store to a JSON-able map format, similar to the Nomad example.
func StateAsMap(store *state.Store) map[string][]interface{} {
	result := map[string][]interface{}{
		"Nodes":    nil,
		"Services": nil,
	}

	for _, table := range AllTableNames {
		result = append(result, toArray(store.SnapshotState(nil, string(table))))
	}
	// Example structure, adjust according to actual Consul StateStore structure and methods

	// Optionally insert any enterprise-specific state components if applicable
	return result
}

func toArray(iter memdb.ResultIterator, err error) []interface{} {
	if err != nil {
		return []interface{}{err}
	}

	var r []interface{}

	if iter != nil {
		item := iter.Next()
		for item != nil {
			r = append(r, item)
			item = iter.Next()
		}
	}

	return r
}

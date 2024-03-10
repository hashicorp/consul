// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"fmt"
	"io"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/snapshot"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
)

func RestoreFromArchive(archive io.Reader) (*state.Store, *raft.SnapshotMeta, error) {
	logger := hclog.L()

	fsm, err := dummyFSM(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	// r is closed by RestoreFiltered, w is closed by CopySnapshot
	r, w := io.Pipe()

	errCh := make(chan error)
	metaCh := make(chan *raft.SnapshotMeta)

	go func() {
		meta, err := snapshot.CopySnapshot(archive, w)
		if err != nil {
			errCh <- fmt.Errorf("failed to read snapshot: %w", err)
		} else {
			metaCh <- meta
		}
	}()

	err = fsm.RestoreWithFilter(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to restore from snapshot: %w", err)
	}

	select {
	case err := <-errCh:
		return nil, nil, err
	case meta := <-metaCh:
		return fsm.State(), meta, nil
	}
}

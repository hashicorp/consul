// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package fsm

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/hashicorp/go-raftchunking"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-wal/verifier"

	"github.com/hashicorp/consul/agent/structs"
)

type logVerificationChunkingShim struct {
	chunker *raftchunking.ChunkingFSM
}

var logVerifierMagicBytes [8]byte

func init() {
	binary.LittleEndian.PutUint64(logVerifierMagicBytes[:], verifier.ExtensionMagicPrefix)
}

// Apply implements raft.FSM.
func (s *logVerificationChunkingShim) Apply(l *raft.Log) interface{} {
	// This is a hack because raftchunking doesn't play nicely with lower-level
	// usage of Extensions field like we need for LogStore verification. We might
	// change that instead but just seeing if I can get this to work here without
	// upstream changes for now.

	// We rely on the way we encode a checkpoint message being distinguishable
	// from any valid chunked log entry. The type byte alone or the fact there is
	// only one byte of data is not quite enough because it's just possible that
	// chunking might split a larger log such that its final chunk was just a
	// single byte, and if so there is a 1 in 256 chance it collides with our type
	// byte! But we specially chose a magic value for verifier.LogStore to use
	// that would never be the first 8 bytes of a valid proto encoding. See the
	// docs on that value for more detail on why not. Note the data length for a
	// checkpoint is actually 2 because msgpack encodes the nil slice as a typed
	// nil byte (0xc0).
	if len(l.Data) == 2 &&
		structs.MessageType(l.Data[0]) == (structs.RaftLogVerifierCheckpoint|structs.IgnoreUnknownTypeFlag) &&
		len(l.Extensions) > 8 &&
		bytes.Equal(logVerifierMagicBytes[:], l.Extensions[0:8]) {
		// Handle the checkpoint here since the lower level FSM doesn't know
		// anything about it! The LogStore has already done what we need, we just
		// need to return the index so that the caller can know which index the
		// checkpoint ended up at.
		return l.Index
	}
	return s.chunker.Apply(l)
}

// Snapshot implements raft.FSM
func (s *logVerificationChunkingShim) Snapshot() (raft.FSMSnapshot, error) {
	return s.chunker.Snapshot()
}

// Restore implements raft.FSM
func (s *logVerificationChunkingShim) Restore(snapshot io.ReadCloser) error {
	return s.chunker.Restore(snapshot)
}

// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"encoding/binary"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-wal/verifier"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestIsLogVerifyCheckpoint(t *testing.T) {
	var verifierExtensions [24]byte
	binary.LittleEndian.PutUint64(verifierExtensions[:8], verifier.ExtensionMagicPrefix)

	tests := map[string]struct {
		log  raft.Log
		want bool
	}{
		"checkpoint": {
			log: raft.Log{
				Data:       []byte{byte(structs.RaftLogVerifierCheckpoint | structs.IgnoreUnknownTypeFlag), 0xc0},
				Extensions: verifierExtensions[:],
			},
			want: true,
		},
		"chunk payload begins with checkpoint type": {
			log: raft.Log{
				Data:       []byte{byte(structs.RaftLogVerifierCheckpoint), 0x00},
				Extensions: []byte{0x0a, 0x08, 0x01}, // protobuf-encoded ChunkInfo
			},
		},
		"short extension": {
			log: raft.Log{
				Data:       []byte{byte(structs.RaftLogVerifierCheckpoint | structs.IgnoreUnknownTypeFlag), 0xc0},
				Extensions: []byte{0x01},
			},
		},
		"non-checkpoint entry": {
			log: raft.Log{
				Data:       []byte{byte(structs.KVSRequestType), 0xc0},
				Extensions: verifierExtensions[:],
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := isLogVerifyCheckpoint(&tc.log)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

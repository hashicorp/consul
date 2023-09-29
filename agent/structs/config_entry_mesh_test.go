// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMeshConfigEntry_PeerThroughMeshGateways(t *testing.T) {
	tests := map[string]struct {
		input *MeshConfigEntry
		want  bool
	}{
		"nil entry": {
			input: nil,
			want:  false,
		},
		"nil peering config": {
			input: &MeshConfigEntry{
				Peering: nil,
			},
			want: false,
		},
		"not peering through gateways": {
			input: &MeshConfigEntry{
				Peering: &PeeringMeshConfig{
					PeerThroughMeshGateways: false,
				},
			},
			want: false,
		},
		"peering through gateways": {
			input: &MeshConfigEntry{
				Peering: &PeeringMeshConfig{
					PeerThroughMeshGateways: true,
				},
			},
			want: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equalf(t, tc.want, tc.input.PeerThroughMeshGateways(), "PeerThroughMeshGateways()")
		})
	}
}

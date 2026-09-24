// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package pbconfigentry

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/structs"
)

func TestExtProcOverridesProtoRoundTripPreservesGRPCInitialMetadata(t *testing.T) {
	src := &structs.HTTPRouteConfigEntry{
		Kind: structs.HTTPRoute,
		Name: "camp-credential-binding",
		Rules: []structs.HTTPRouteRule{{
			Filters: structs.HTTPFilters{
				ExtProc: []structs.ExtProcFilter{{
					Mode: "override",
					Overrides: &structs.ExtProcOverrides{
						GRPCInitialMetadata: []structs.ExtProcMetadataKV{{
							Key:   "x-camp-auth-binding",
							Value: "openai-a",
						}},
					},
				}},
			},
		}},
	}

	pb := ConfigEntryFromStructs(src)
	encoded, err := proto.Marshal(pb)
	require.NoError(t, err)

	var decoded ConfigEntry
	require.NoError(t, proto.Unmarshal(encoded, &decoded))

	got, ok := ConfigEntryToStructs(&decoded).(*structs.HTTPRouteConfigEntry)
	require.True(t, ok)
	require.Equal(t, src.Rules[0].Filters.ExtProc[0].Overrides.GRPCInitialMetadata,
		got.Rules[0].Filters.ExtProc[0].Overrides.GRPCInitialMetadata)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
)

func TestStore_prepareImportedServicesResponse(t *testing.T) {

	importedServices := []importedService{
		{
			service: "db",
			peer:    "east",
		},
		{
			service: "web",
			peer:    "west",
		},
	}

	resp := prepareImportedServicesResponse(importedServices, nil)

	expected := []*pbconfigentry.ResolvedImportedService{
		{
			Service:    "db",
			SourcePeer: "east",
		},
		{
			Service:    "web",
			SourcePeer: "west",
		},
	}

	require.Equal(t, expected, resp)
}

func TestStore_ResolvedImportedServices(t *testing.T) {
	s := NewStateStore(nil)
	var c indexCounter

	// Create service intentions with peer sources (which represent imported services)
	{
		entry := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "db",
			Sources: []*structs.SourceIntention{
				{
					Name:   "web",
					Action: structs.IntentionActionAllow,
					Peer:   "east",
				},
				{
					Name:   "api",
					Action: structs.IntentionActionAllow,
					Peer:   "west",
				},
			},
		}

		require.NoError(t, s.EnsureConfigEntry(c.Next(), entry))
	}

	{
		entry := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "cache",
			Sources: []*structs.SourceIntention{
				{
					Name:   "service1",
					Action: structs.IntentionActionAllow,
					Peer:   "east",
				},
			},
		}

		require.NoError(t, s.EnsureConfigEntry(c.Next(), entry))
	}

	// Query for imported services
	entMeta := acl.EnterpriseMeta{}
	_, importedServices, err := s.ResolvedImportedServices(nil, &entMeta)

	require.NoError(t, err)
	require.Len(t, importedServices, 3)

	expected := []*pbconfigentry.ResolvedImportedService{
		{
			Service:    "cache",
			SourcePeer: "east",
		},
		{
			Service:    "db",
			SourcePeer: "east",
		},
		{
			Service:    "db",
			SourcePeer: "west",
		},
	}

	require.Equal(t, expected, importedServices)
}

func TestStore_ResolvedImportedServices_NoDuplicates(t *testing.T) {
	s := NewStateStore(nil)
	var c indexCounter

	// Create multiple intentions with the same peer importing the same service
	{
		entry := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "db",
			Sources: []*structs.SourceIntention{
				{
					Name:   "web",
					Action: structs.IntentionActionAllow,
					Peer:   "east",
				},
				{
					Name:   "api",
					Action: structs.IntentionActionAllow,
					Peer:   "east",
				},
			},
		}

		require.NoError(t, s.EnsureConfigEntry(c.Next(), entry))
	}

	// Query for imported services
	entMeta := acl.EnterpriseMeta{}
	_, importedServices, err := s.ResolvedImportedServices(nil, &entMeta)

	require.NoError(t, err)
	// Should only have one entry despite two sources from the same peer
	require.Len(t, importedServices, 1)

	expected := []*pbconfigentry.ResolvedImportedService{
		{
			Service:    "db",
			SourcePeer: "east",
		},
	}

	require.Equal(t, expected, importedServices)
}

func TestStore_ResolvedImportedServices_NoImports(t *testing.T) {
	s := NewStateStore(nil)
	var c indexCounter

	// Create intention without peer source (local intention)
	{
		entry := &structs.ServiceIntentionsConfigEntry{
			Kind: structs.ServiceIntentions,
			Name: "db",
			Sources: []*structs.SourceIntention{
				{
					Name:   "web",
					Action: structs.IntentionActionAllow,
				},
			},
		}

		require.NoError(t, s.EnsureConfigEntry(c.Next(), entry))
	}

	// Query for imported services
	entMeta := acl.EnterpriseMeta{}
	_, importedServices, err := s.ResolvedImportedServices(nil, &entMeta)

	require.NoError(t, err)
	require.Len(t, importedServices, 0)
}

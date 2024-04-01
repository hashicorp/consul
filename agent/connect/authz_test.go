// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAuthorizeIntentionTarget(t *testing.T) {
	cases := []struct {
		name       string
		target     string
		targetNS   string
		targetAP   string
		targetPeer string
		ixn        *structs.Intention
		matchType  structs.IntentionMatchType
		auth       bool
		match      bool
	}{
		// Source match type
		{
			name:   "match exact source, not matching name",
			target: "web",
			ixn: &structs.Intention{
				SourceName: "db",
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     false,
		},
		{
			name:   "match exact source, allow",
			target: "web",
			ixn: &structs.Intention{
				SourceName: "web",
				Action:     structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchSource,
			auth:      true,
			match:     true,
		},
		{
			name:   "match exact source, deny",
			target: "web",
			ixn: &structs.Intention{
				SourceName: "web",
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     true,
		},
		{
			name:     "match wildcard service, deny",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				SourceName: structs.WildcardSpecifier,
				SourceNS:   structs.IntentionDefaultNamespace,
				Action:     structs.IntentionActionDeny,
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     true,
		},
		{
			name:   "match wildcard service, allow",
			target: "web",
			ixn: &structs.Intention{
				SourceName: structs.WildcardSpecifier,
				Action:     structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchSource,
			auth:      true,
			match:     true,
		},

		// Destination match type
		{
			name:   "match exact destination, not matching name",
			target: "web",
			ixn: &structs.Intention{
				DestinationName: "db",
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     false,
		},
		{
			name:   "match exact destination, allow",
			target: "web",
			ixn: &structs.Intention{
				DestinationName: "web",
				Action:          structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      true,
			match:     true,
		},
		{
			name:   "match exact destination, deny",
			target: "web",
			ixn: &structs.Intention{
				DestinationName: "web",
				Action:          structs.IntentionActionDeny,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     true,
		},
		{
			name:   "match wildcard service, deny",
			target: "web",
			ixn: &structs.Intention{
				DestinationName: structs.WildcardSpecifier,
				Action:          structs.IntentionActionDeny,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     true,
		},
		{
			name:   "match wildcard service, allow",
			target: "web",
			ixn: &structs.Intention{
				DestinationName: structs.WildcardSpecifier,
				Action:          structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      true,
			match:     true,
		},
		{
			name:   "unknown match type",
			target: "web",
			ixn: &structs.Intention{
				DestinationName: structs.WildcardSpecifier,
				Action:          structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchType("unknown"),
			auth:      false,
			match:     false,
		},
		{
			name:       "match peer",
			target:     "web",
			targetNS:   structs.IntentionDefaultNamespace,
			targetPeer: "cluster-01",
			ixn: &structs.Intention{
				SourceName: "web",
				SourceNS:   structs.IntentionDefaultNamespace,
				SourcePeer: "cluster-01",
				Action:     structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchSource,
			auth:      true,
			match:     true,
		},
		{
			name:       "no peer match",
			target:     "web",
			targetNS:   structs.IntentionDefaultNamespace,
			targetPeer: "cluster-02",
			ixn: &structs.Intention{
				SourceName: "web",
				SourceNS:   structs.IntentionDefaultNamespace,
				SourcePeer: "cluster-01",
				Action:     structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth, match := AuthorizeIntentionTarget(tc.target, tc.targetNS, tc.targetAP, tc.targetPeer, tc.ixn, tc.matchType)
			assert.Equal(t, tc.auth, auth)
			assert.Equal(t, tc.match, match)
		})
	}
}

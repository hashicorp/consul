package connect

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAuthorizeIntentionTarget(t *testing.T) {
	cases := []struct {
		name      string
		target    string
		targetNS  string
		targetAP  string
		ixn       *structs.Intention
		matchType structs.IntentionMatchType
		auth      bool
		match     bool
	}{
		// Source match type
		{
			name:     "matching source target and namespace, but not partition",
			target:   "db",
			targetNS: structs.IntentionDefaultNamespace,
			targetAP: "foo",
			ixn: &structs.Intention{
				SourceName:      "db",
				SourceNS:        structs.IntentionDefaultNamespace,
				SourcePartition: "not-foo",
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     false,
		},
		{
			name:     "match exact source, not matching namespace",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				SourceName: "db",
				SourceNS:   "different",
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     false,
		},
		{
			name:     "match exact source, not matching name",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				SourceName: "db",
				SourceNS:   structs.IntentionDefaultNamespace,
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     false,
		},
		{
			name:     "match exact source, allow",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				SourceName: "web",
				SourceNS:   structs.IntentionDefaultNamespace,
				Action:     structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchSource,
			auth:      true,
			match:     true,
		},
		{
			name:     "match exact source, deny",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				SourceName: "web",
				SourceNS:   structs.IntentionDefaultNamespace,
				Action:     structs.IntentionActionDeny,
			},
			matchType: structs.IntentionMatchSource,
			auth:      false,
			match:     true,
		},
		{
			name:     "match exact sourceNS for wildcard service, deny",
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
			name:     "match exact sourceNS for wildcard service, allow",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				SourceName: structs.WildcardSpecifier,
				SourceNS:   structs.IntentionDefaultNamespace,
				Action:     structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchSource,
			auth:      true,
			match:     true,
		},

		// Destination match type
		{
			name:     "matching destination target and namespace, but not partition",
			target:   "db",
			targetNS: structs.IntentionDefaultNamespace,
			targetAP: "foo",
			ixn: &structs.Intention{
				SourceName:      "db",
				SourceNS:        structs.IntentionDefaultNamespace,
				SourcePartition: "not-foo",
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     false,
		},
		{
			name:     "match exact destination, not matching namespace",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				DestinationName: "db",
				DestinationNS:   "different",
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     false,
		},
		{
			name:     "match exact destination, not matching name",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				DestinationName: "db",
				DestinationNS:   structs.IntentionDefaultNamespace,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     false,
		},
		{
			name:     "match exact destination, allow",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				DestinationName: "web",
				DestinationNS:   structs.IntentionDefaultNamespace,
				Action:          structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      true,
			match:     true,
		},
		{
			name:     "match exact destination, deny",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				DestinationName: "web",
				DestinationNS:   structs.IntentionDefaultNamespace,
				Action:          structs.IntentionActionDeny,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     true,
		},
		{
			name:     "match exact destinationNS for wildcard service, deny",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				DestinationName: structs.WildcardSpecifier,
				DestinationNS:   structs.IntentionDefaultNamespace,
				Action:          structs.IntentionActionDeny,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      false,
			match:     true,
		},
		{
			name:     "match exact destinationNS for wildcard service, allow",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				DestinationName: structs.WildcardSpecifier,
				DestinationNS:   structs.IntentionDefaultNamespace,
				Action:          structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchDestination,
			auth:      true,
			match:     true,
		},
		{
			name:     "unknown match type",
			target:   "web",
			targetNS: structs.IntentionDefaultNamespace,
			ixn: &structs.Intention{
				DestinationName: structs.WildcardSpecifier,
				DestinationNS:   structs.IntentionDefaultNamespace,
				Action:          structs.IntentionActionAllow,
			},
			matchType: structs.IntentionMatchType("unknown"),
			auth:      false,
			match:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth, match := AuthorizeIntentionTarget(tc.target, tc.targetNS, tc.targetAP, tc.ixn, tc.matchType)
			assert.Equal(t, tc.auth, auth)
			assert.Equal(t, tc.match, match)
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func TestServerIntentionUpstreams(t *testing.T) {
	const serviceName = "web"

	var index uint64
	getIndex := func() uint64 {
		index++
		return index
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	store := state.NewStateStore(nil)
	disableLegacyIntentions(t, store)

	// Register api and db services.
	for _, service := range []string{"api", "db"} {
		err := store.EnsureRegistration(getIndex(), &structs.RegisterRequest{
			Node: "node-1",
			Service: &structs.NodeService{
				Service: service,
			},
		})
		require.NoError(t, err)
	}

	createIntention := func(destination string) {
		t.Helper()

		err := store.EnsureConfigEntry(getIndex(), &structs.ServiceIntentionsConfigEntry{
			Name: destination,
			Sources: []*structs.SourceIntention{
				{
					Name:   serviceName,
					Action: structs.IntentionActionAllow,
					Type:   structs.IntentionSourceConsul,
				},
			},
		})
		require.NoError(t, err)
	}

	// Create an allow intention for the api service. This should be filtered out
	// because the ACL token doesn't have read access on it.
	createIntention("api")

	authz := policyAuthorizer(t, `service "db" { policy = "read" }`)

	dataSource := ServerIntentionUpstreams(ServerDataSourceDeps{
		ACLResolver: newStaticResolver(authz),
		GetStore:    func() Store { return store },
	})

	ch := make(chan proxycfg.UpdateEvent)
	err := dataSource.Notify(ctx, &structs.ServiceSpecificRequest{ServiceName: serviceName}, "", ch)
	require.NoError(t, err)

	result := getEventResult[*structs.IndexedServiceList](t, ch)
	require.Len(t, result.Services, 0)

	// Create an allow intention for the db service. This should *not* be filtered
	// out because the ACL token *does* have read access on it.
	createIntention("db")

	result = getEventResult[*structs.IndexedServiceList](t, ch)
	require.Len(t, result.Services, 1)
	require.Equal(t, "db", result.Services[0].Name)
}

func disableLegacyIntentions(t *testing.T, store *state.Store) {
	t.Helper()

	require.NoError(t, store.SystemMetadataSet(0, &structs.SystemMetadataEntry{
		Key:   structs.SystemMetadataIntentionFormatKey,
		Value: structs.SystemMetadataIntentionFormatConfigValue,
	}))
}

func policyAuthorizer(t *testing.T, policyHCL string) acl.Authorizer {
	policy, err := acl.NewPolicyFromSource(policyHCL, nil, nil)
	require.NoError(t, err)

	authz, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	return authz
}

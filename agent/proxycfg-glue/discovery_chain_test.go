// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func TestServerCompiledDiscoveryChain(t *testing.T) {
	t.Run("remote queries are delegated to the remote source", func(t *testing.T) {
		var (
			ctx           = context.Background()
			req           = &structs.DiscoveryChainRequest{Datacenter: "dc2"}
			correlationID = "correlation-id"
			ch            = make(chan<- proxycfg.UpdateEvent)
			result        = errors.New("KABOOM")
		)

		remoteSource := newMockCompiledDiscoveryChain(t)
		remoteSource.On("Notify", ctx, req, correlationID, ch).Return(result)

		dataSource := ServerCompiledDiscoveryChain(ServerDataSourceDeps{Datacenter: "dc1"}, remoteSource)
		err := dataSource.Notify(ctx, req, correlationID, ch)
		require.Equal(t, result, err)
	})

	t.Run("local queries are served from the state store", func(t *testing.T) {
		const (
			serviceName = "web"
			datacenter  = "dc1"
			index       = 123
		)

		store := state.NewStateStore(nil)
		require.NoError(t, store.CASetConfig(index, &structs.CAConfiguration{ClusterID: "cluster-id"}))
		require.NoError(t, store.EnsureConfigEntry(index, &structs.ServiceConfigEntry{
			Name: serviceName,
			Kind: structs.ServiceDefaults,
		}))

		req := &structs.DiscoveryChainRequest{
			Name:       serviceName,
			Datacenter: datacenter,
		}

		resolver := newStaticResolver(
			policyAuthorizer(t, fmt.Sprintf(`service "%s" { policy = "read" }`, serviceName)),
		)

		dataSource := ServerCompiledDiscoveryChain(ServerDataSourceDeps{
			ACLResolver: resolver,
			Datacenter:  datacenter,
			GetStore:    func() Store { return store },
		}, nil)

		eventCh := make(chan proxycfg.UpdateEvent)
		err := dataSource.Notify(context.Background(), req, "", eventCh)
		require.NoError(t, err)

		// Check we get an event with the initial state.
		result := getEventResult[*structs.DiscoveryChainResponse](t, eventCh)
		require.NotNil(t, result.Chain)

		// Change the protocol to HTTP and check we get a recompiled chain.
		require.NoError(t, store.EnsureConfigEntry(index+1, &structs.ServiceConfigEntry{
			Name:     serviceName,
			Kind:     structs.ServiceDefaults,
			Protocol: "http",
		}))

		result = getEventResult[*structs.DiscoveryChainResponse](t, eventCh)
		require.NotNil(t, result.Chain)
		require.Equal(t, "http", result.Chain.Protocol)

		// Revoke access to the service.
		resolver.SwapAuthorizer(acl.DenyAll())

		// Write another config entry.
		require.NoError(t, store.EnsureConfigEntry(index+2, &structs.ServiceConfigEntry{
			Name:                  serviceName,
			Kind:                  structs.ServiceDefaults,
			MaxInboundConnections: 1,
		}))

		// Should no longer receive events for this service.
		expectNoEvent(t, eventCh)
	})
}

func newMockCompiledDiscoveryChain(t *testing.T) *mockCompiledDiscoveryChain {
	mock := &mockCompiledDiscoveryChain{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

type mockCompiledDiscoveryChain struct {
	mock.Mock
}

func (m *mockCompiledDiscoveryChain) Notify(ctx context.Context, req *structs.DiscoveryChainRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return m.Called(ctx, req, correlationID, ch).Error(0)
}

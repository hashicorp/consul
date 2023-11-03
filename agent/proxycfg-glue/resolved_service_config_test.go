// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestServerResolvedServiceConfig(t *testing.T) {
	t.Run("remote queries are delegated to the remote source", func(t *testing.T) {
		var (
			ctx           = context.Background()
			req           = &structs.ServiceConfigRequest{Datacenter: "dc2"}
			correlationID = "correlation-id"
			ch            = make(chan<- proxycfg.UpdateEvent)
			result        = errors.New("KABOOM")
		)

		remoteSource := newMockResolvedServiceConfig(t)
		remoteSource.On("Notify", ctx, req, correlationID, ch).Return(result)

		dataSource := ServerResolvedServiceConfig(ServerDataSourceDeps{Datacenter: "dc1"}, remoteSource)
		err := dataSource.Notify(ctx, req, correlationID, ch)
		require.Equal(t, result, err)
	})

	t.Run("local queries are served from the state store", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		const (
			serviceName = "web"
			datacenter  = "dc1"
		)

		store := state.NewStateStore(nil)
		nextIndex := indexGenerator()

		require.NoError(t, store.EnsureConfigEntry(nextIndex(), &structs.ServiceConfigEntry{
			Name:     serviceName,
			Protocol: "http",
		}))

		authz := newStaticResolver(
			policyAuthorizer(t, fmt.Sprintf(`service "%s" { policy = "read" }`, serviceName)),
		)

		dataSource := ServerResolvedServiceConfig(ServerDataSourceDeps{
			Datacenter:  datacenter,
			ACLResolver: authz,
			GetStore:    func() Store { return store },
		}, nil)

		eventCh := make(chan proxycfg.UpdateEvent)
		require.NoError(t, dataSource.Notify(ctx, &structs.ServiceConfigRequest{Datacenter: datacenter, Name: serviceName}, "", eventCh))

		testutil.RunStep(t, "initial state", func(t *testing.T) {
			result := getEventResult[*structs.ServiceConfigResponse](t, eventCh)
			require.Equal(t, map[string]any{"protocol": "http"}, result.ProxyConfig)
		})

		testutil.RunStep(t, "write proxy defaults", func(t *testing.T) {
			require.NoError(t, store.EnsureConfigEntry(nextIndex(), &structs.ProxyConfigEntry{
				Name: structs.ProxyConfigGlobal,
				Mode: structs.ProxyModeDirect,
			}))
			result := getEventResult[*structs.ServiceConfigResponse](t, eventCh)
			require.Equal(t, structs.ProxyModeDirect, result.Mode)
		})

		testutil.RunStep(t, "delete service config", func(t *testing.T) {
			require.NoError(t, store.DeleteConfigEntry(nextIndex(), structs.ServiceDefaults, serviceName, nil))

			result := getEventResult[*structs.ServiceConfigResponse](t, eventCh)
			require.Empty(t, result.ProxyConfig)
		})

		testutil.RunStep(t, "revoke access", func(t *testing.T) {
			authz.SwapAuthorizer(acl.DenyAll())

			require.NoError(t, store.EnsureConfigEntry(nextIndex(), &structs.ServiceConfigEntry{
				Name:     serviceName,
				Protocol: "http",
			}))

			expectNoEvent(t, eventCh)
		})
	})
}

func newMockResolvedServiceConfig(t *testing.T) *mockResolvedServiceConfig {
	mock := &mockResolvedServiceConfig{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

type mockResolvedServiceConfig struct {
	mock.Mock
}

func (m *mockResolvedServiceConfig) Notify(ctx context.Context, req *structs.ServiceConfigRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return m.Called(ctx, req, correlationID, ch).Error(0)
}

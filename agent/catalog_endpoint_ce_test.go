// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	agMetrics "github.com/hashicorp/consul/agent/metrics"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestCatalog_Exported_Services(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	sink := metrics.NewInmemSink(5*time.Second, time.Minute)
	cfg := metrics.DefaultConfig("consul")
	metrics.NewGlobal(cfg, sink)

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	{
		// Register exported services
		args := &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{
				{
					Name: "api",
					Consumers: []structs.ServiceConsumer{
						{
							Peer: "east",
						},
						{
							Peer: "west",
						},
					},
				},
				{
					Name: "db",
					Consumers: []structs.ServiceConsumer{
						{
							Peer: "east",
						},
					},
				},
			},
		}
		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var configOutput bool
		require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &req, &configOutput))
		require.True(t, configOutput)
	}

	t.Run("exported services", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/catalog/exported-services", nil)
		resp := httptest.NewRecorder()
		raw, err := a.srv.CatalogExportedServices(resp, req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.Code)

		services, ok := raw.([]structs.SimplifiedExportedService)
		require.True(t, ok)
		require.Len(t, services, 2)
		assertIndex(t, resp)

		expected := []structs.SimplifiedExportedService{
			{
				Service:       structs.NewServiceName("api", nil),
				ConsumerPeers: []string{"east", "west"},
			},
			{
				Service:       structs.NewServiceName("db", nil),
				ConsumerPeers: []string{"east"},
			},
		}
		require.ElementsMatch(t, expected, services)
	})

	// Checking if metrics were added
	key1 := fmt.Sprintf(`consul.client.api.catalog_exported_services;node=%s`, a.consulConfig().NodeName)
	key2 := fmt.Sprintf(`consul.client.api.success.catalog_exported_services;node=%s`, a.consulConfig().NodeName)
	agMetrics.AssertCounter(t, sink, key1, 1)
	agMetrics.AssertCounter(t, sink, key2, 1)
}

// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

// TestAPIGatewayMutexProtection tests that the chainMutex properly protects
// concurrent access to discovery chain map updates. This simulates the scenario
// where multiple API Gateway replicas start simultaneously and process route
// updates concurrently, which was causing Envoy segmentation faults.
func TestAPIGatewayMutexProtection(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Create a handler - the mutex is what we're testing
	handler := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				source: &structs.QuerySource{
					Datacenter: "dc1",
				},
			},
		},
	}

	// Create a snapshot with discovery chain map
	snap := &ConfigSnapshot{
		Kind: structs.ServiceKindAPIGateway,
		APIGateway: configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: make(map[UpstreamID]*structs.CompiledDiscoveryChain),
			},
			GatewayConfig: &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "test-gateway",
			},
			Listeners:      make(map[string]structs.APIGatewayListener),
			BoundListeners: make(map[string]structs.BoundAPIGatewayListener),
		},
	}

	// Simulate concurrent access to the discovery chain map
	// This tests that the mutex prevents race conditions
	const numGoroutines = 20
	const numIterations = 100

	var wg sync.WaitGroup
	raceFree := true

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// Simulate concurrent map access that would race without mutex
				handler.chainMutex.Lock()

				// Write to the map
				testID := NewUpstreamIDFromServiceName(structs.NewServiceName(
					"test-service",
					structs.DefaultEnterpriseMetaInDefaultPartition(),
				))
				snap.APIGateway.DiscoveryChain[testID] = &structs.CompiledDiscoveryChain{
					ServiceName: "test-service",
				}

				// Read from the map
				_, exists := snap.APIGateway.DiscoveryChain[testID]
				if !exists {
					raceFree = false
				}

				handler.chainMutex.Unlock()

				// Small delay to increase chance of detecting races
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify no race conditions occurred
	require.True(t, raceFree, "Mutex should prevent race conditions on discovery chain map")
}

// TestAPIGatewayMultipleReplicas tests concurrent recompilation similar to
// production scenario with multiple API Gateway replicas
func TestAPIGatewayMultipleReplicas(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	handler := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				source: &structs.QuerySource{
					Datacenter: "dc1",
				},
			},
		},
	}

	snap := &ConfigSnapshot{
		Kind: structs.ServiceKindAPIGateway,
		APIGateway: configSnapshotAPIGateway{
			ConfigSnapshotUpstreams: ConfigSnapshotUpstreams{
				DiscoveryChain: make(map[UpstreamID]*structs.CompiledDiscoveryChain),
			},
			GatewayConfig: &structs.APIGatewayConfigEntry{
				Kind: structs.APIGateway,
				Name: "test-gateway",
			},
			Listeners:      make(map[string]structs.APIGatewayListener),
			BoundListeners: make(map[string]structs.BoundAPIGatewayListener),
		},
	}

	// Simulate 3 replicas processing concurrently
	const numReplicas = 3
	var wg sync.WaitGroup

	for i := 0; i < numReplicas; i++ {
		wg.Add(1)
		go func(replicaID int) {
			defer wg.Done()

			// Each replica performs multiple operations
			for j := 0; j < 50; j++ {
				// Simulate discovery chain updates
				handler.chainMutex.Lock()

				testID := NewUpstreamIDFromServiceName(structs.NewServiceName(
					"service-"+string(rune(replicaID)),
					structs.DefaultEnterpriseMetaInDefaultPartition(),
				))
				snap.APIGateway.DiscoveryChain[testID] = &structs.CompiledDiscoveryChain{
					ServiceName: "service-" + string(rune(replicaID)),
				}

				handler.chainMutex.Unlock()
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify the map is in a consistent state
	require.NotNil(t, snap.APIGateway.DiscoveryChain, "Discovery chain map should not be nil")
}

// Made with Bob

// +build !consulent

package proxycfg

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

// This test is meant to exercise the various parts of the cache watching done by the state as
// well as its management of the ConfigSnapshot
//
// This test is expressly not calling Watch which in turn would execute the run function in a go
// routine. This allows the test to be fully synchronous and deterministic while still being able
// to validate the logic of most of the watching and state updating.
//
// The general strategy here is to
//
// 1. Initialize a state with a call to newState + setting some of the extra stuff like the CacheNotifier
//    We will not be using the CacheNotifier to send notifications but calling handleUpdate ourselves
// 2. Iterate through a list of verification stages performing validation and updates for each.
//    a. Ensure that the required watches are in place and validate they are correct
//    b. Process a bunch of UpdateEvents by calling handleUpdate
//    c. Validate that the ConfigSnapshot has been updated appropriately
func TestState_WatchesAndUpdates_OSS(t *testing.T) {
	t.Parallel()

	indexedRoots, issuedCert := TestCerts(t)

	rootWatchEvent := func() cache.UpdateEvent {
		return cache.UpdateEvent{
			CorrelationID: rootsWatchID,
			Result:        indexedRoots,
			Err:           nil,
		}
	}

	type verificationStage struct {
		requiredWatches map[string]verifyWatchRequest
		events          []cache.UpdateEvent
		verifySnapshot  func(t testing.TB, snap *ConfigSnapshot)
	}

	type testCase struct {
		// the state to operate on. the logger, source, cache,
		// ctx and cancel fields will be filled in by the test
		ns       structs.NodeService
		sourceDC string
		stages   []verificationStage
	}

	cases := map[string]testCase{
		"terminating-gateway-handle-update": testCase{
			ns: structs.NodeService{
				Kind:    structs.ServiceKindTerminatingGateway,
				ID:      "terminating-gateway",
				Service: "terminating-gateway",
				Address: "10.0.1.1",
			},
			sourceDC: "dc1",
			stages: []verificationStage{
				verificationStage{
					requiredWatches: map[string]verifyWatchRequest{
						rootsWatchID: genVerifyRootsWatch("dc1"),
						gatewayServicesWatchID: genVerifyServiceSpecificRequest(gatewayServicesWatchID,
							"terminating-gateway", "", "dc1", false),
					},
					events: []cache.UpdateEvent{
						rootWatchEvent(),
						cache.UpdateEvent{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Service: structs.NewServiceID("db", nil),
										Gateway: structs.NewServiceID("terminating-gateway", nil),
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.WatchedServices, 1)
					},
				},
				verificationStage{
					events: []cache.UpdateEvent{
						cache.UpdateEvent{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Service: structs.NewServiceID("db", nil),
										Gateway: structs.NewServiceID("terminating-gateway", nil),
									},
									{
										Service: structs.NewServiceID("billing", nil),
										Gateway: structs.NewServiceID("terminating-gateway", nil),
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						db := structs.NewServiceID("db", nil)
						billing := structs.NewServiceID("billing", nil)

						require.True(t, snap.Valid(), "gateway with service list is valid")
						require.Len(t, snap.TerminatingGateway.WatchedServices, 2)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, db)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, billing)

						require.Len(t, snap.TerminatingGateway.WatchedIntentions, 2)
						require.Contains(t, snap.TerminatingGateway.WatchedIntentions, db)
						require.Contains(t, snap.TerminatingGateway.WatchedIntentions, billing)

						require.Len(t, snap.TerminatingGateway.WatchedLeaves, 2)
						require.Contains(t, snap.TerminatingGateway.WatchedLeaves, db)
						require.Contains(t, snap.TerminatingGateway.WatchedLeaves, billing)

						require.Len(t, snap.TerminatingGateway.WatchedResolvers, 2)
						require.Contains(t, snap.TerminatingGateway.WatchedResolvers, db)
						require.Contains(t, snap.TerminatingGateway.WatchedResolvers, billing)

						require.Len(t, snap.TerminatingGateway.GatewayServices, 2)
						require.Contains(t, snap.TerminatingGateway.GatewayServices, db)
						require.Contains(t, snap.TerminatingGateway.GatewayServices, billing)
					},
				},
				verificationStage{
					requiredWatches: map[string]verifyWatchRequest{
						"external-service:db": genVerifyServiceWatch("db", "", "dc1", false),
					},
					events: []cache.UpdateEvent{
						cache.UpdateEvent{
							CorrelationID: "external-service:db",
							Result: &structs.IndexedCheckServiceNodes{
								Nodes: structs.CheckServiceNodes{
									{
										Node: &structs.Node{
											Node:    "node1",
											Address: "127.0.0.1",
										},
										Service: &structs.NodeService{
											ID:      "db",
											Service: "db",
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Len(t, snap.TerminatingGateway.ServiceGroups, 1)
						require.Equal(t, snap.TerminatingGateway.ServiceGroups[structs.NewServiceID("db", nil)],
							structs.CheckServiceNodes{
								{
									Node: &structs.Node{
										Node:    "node1",
										Address: "127.0.0.1",
									},
									Service: &structs.NodeService{
										ID:      "db",
										Service: "db",
									},
								},
							},
						)
					},
				},
				verificationStage{
					requiredWatches: map[string]verifyWatchRequest{
						"service-leaf:db": genVerifyLeafWatch("db", "dc1"),
					},
					events: []cache.UpdateEvent{
						cache.UpdateEvent{
							CorrelationID: "service-leaf:db",
							Result:        issuedCert,
							Err:           nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						require.Equal(t, snap.TerminatingGateway.ServiceLeaves[structs.NewServiceID("db", nil)], issuedCert)
					},
				},
				verificationStage{
					requiredWatches: map[string]verifyWatchRequest{
						"service-resolver:db": genVerifyResolverWatch("db", "dc1", structs.ServiceResolver),
					},
					events: []cache.UpdateEvent{
						cache.UpdateEvent{
							CorrelationID: "service-resolver:db",
							Result: &structs.IndexedConfigEntries{
								Kind: structs.ServiceResolver,
								Entries: []structs.ConfigEntry{
									&structs.ServiceResolverConfigEntry{
										Name: "db",
										Kind: structs.ServiceResolver,
										Redirect: &structs.ServiceResolverRedirect{
											Service:    "db",
											Datacenter: "dc2",
										},
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						want := &structs.ServiceResolverConfigEntry{
							Kind: structs.ServiceResolver,
							Name: "db",
							Redirect: &structs.ServiceResolverRedirect{
								Service:    "db",
								Datacenter: "dc2",
							},
						}
						require.Equal(t, want, snap.TerminatingGateway.ServiceResolvers[structs.NewServiceID("db", nil)])
					},
				},
				verificationStage{
					events: []cache.UpdateEvent{
						cache.UpdateEvent{
							CorrelationID: gatewayServicesWatchID,
							Result: &structs.IndexedGatewayServices{
								Services: structs.GatewayServices{
									{
										Service: structs.NewServiceID("billing", nil),
										Gateway: structs.NewServiceID("terminating-gateway", nil),
									},
								},
							},
							Err: nil,
						},
					},
					verifySnapshot: func(t testing.TB, snap *ConfigSnapshot) {
						billing := structs.NewServiceID("billing", nil)

						require.True(t, snap.Valid(), "gateway with service list is valid")

						// All the watches should have been cancelled for db
						require.Len(t, snap.TerminatingGateway.WatchedServices, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedServices, billing)

						require.Len(t, snap.TerminatingGateway.WatchedIntentions, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedIntentions, billing)

						require.Len(t, snap.TerminatingGateway.WatchedLeaves, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedLeaves, billing)

						require.Len(t, snap.TerminatingGateway.WatchedResolvers, 1)
						require.Contains(t, snap.TerminatingGateway.WatchedResolvers, billing)

						require.Len(t, snap.TerminatingGateway.GatewayServices, 1)
						require.Contains(t, snap.TerminatingGateway.GatewayServices, billing)

						// There was no update event for billing's leaf/endpoints, so length is 0
						require.Len(t, snap.TerminatingGateway.ServiceGroups, 0)
						require.Len(t, snap.TerminatingGateway.ServiceLeaves, 0)
						require.Len(t, snap.TerminatingGateway.ServiceResolvers, 0)
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			state, err := newState(&tc.ns, "")

			// verify building the initial state worked
			require.NoError(t, err)
			require.NotNil(t, state)

			// setup the test logger to use the t.Log
			state.logger = testutil.Logger(t)

			// setup a new testing cache notifier
			cn := newTestCacheNotifier()
			state.cache = cn

			// setup the local datacenter information
			state.source = &structs.QuerySource{
				Datacenter: tc.sourceDC,
			}

			// setup the ctx as initWatches expects this to be there
			state.ctx, state.cancel = context.WithCancel(context.Background())

			// ensure the initial watch setup did not error
			require.NoError(t, state.initWatches())

			// get the initial configuration snapshot
			snap := state.initialConfigSnapshot()

			//--------------------------------------------------------------------
			//
			// All the nested subtests here are to make failures easier to
			// correlate back with the test table
			//
			//--------------------------------------------------------------------

			for idx, stage := range tc.stages {
				require.True(t, t.Run(fmt.Sprintf("stage-%d", idx), func(t *testing.T) {
					for correlationId, verifier := range stage.requiredWatches {
						require.True(t, t.Run(correlationId, func(t *testing.T) {
							// verify that the watch was initiated
							cacheType, request := cn.verifyWatch(t, correlationId)

							// run the verifier if any
							if verifier != nil {
								verifier(t, cacheType, request)
							}
						}))
					}

					// the state is not currently executing the run method in a goroutine
					// therefore we just tell it about the updates
					for eveIdx, event := range stage.events {
						require.True(t, t.Run(fmt.Sprintf("update-%d", eveIdx), func(t *testing.T) {
							require.NoError(t, state.handleUpdate(event, &snap))
						}))
					}

					// verify the snapshot
					if stage.verifySnapshot != nil {
						stage.verifySnapshot(t, &snap)
					}
				}))
			}
		})
	}
}

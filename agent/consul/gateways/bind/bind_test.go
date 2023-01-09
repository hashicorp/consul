package bind

import (
	"testing"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestBindGateways(t *testing.T) {
	type testCase struct {
		gateways                 []*structs.BoundAPIGatewayConfigEntry
		route                    BoundRouter
		expectedBoundAPIGateways []*structs.BoundAPIGatewayConfigEntry
		expectedReferenceErrors  map[structs.ResourceReference]error
		expectedError            error
	}

	cases := map[string]testCase{
		"TCP Route binds to gateway": {
			gateways: []*structs.BoundAPIGatewayConfigEntry{
				{
					Name:      "Test API Gateway",
					Listeners: []structs.BoundAPIGatewayListener{},
				},
			},
			route:                    &structs.TCPRouteConfigEntry{},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{},
			expectedReferenceErrors:  map[structs.ResourceReference]error{},
			expectedError:            nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create a store for the BoundAPIGateways and put them in it.
			store := state.NewStateStore(nil)
			for i, gateway := range tc.gateways {
				require.NoError(t, store.EnsureConfigEntry(uint64(i), gateway))
			}

			actualBoundAPIGateways, actualReferenceErrors, actualError := BindGateways(store, tc.route)

			require.Equal(t, tc.expectedBoundAPIGateways, actualBoundAPIGateways)
			require.Equal(t, tc.expectedReferenceErrors, actualReferenceErrors)
			require.Equal(t, tc.expectedError, actualError)
		})
	}
}

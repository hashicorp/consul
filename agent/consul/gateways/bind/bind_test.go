package bind

import (
	"testing"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestBindGateways(t *testing.T) {
	type testCase struct {
		store                    *state.Store
		route                    BoundRouter
		expectedBoundAPIGateways []*structs.BoundAPIGatewayConfigEntry
		expectedReferenceErrors  map[structs.ResourceReference]error
		expectedError            error
	}

	cases := map[string]testCase{
		"TCP Route binds to gateway": {
			store:                    state.NewStateStore(nil),
			route:                    structs.TCPRouteConfigEntry{},
			expectedBoundAPIGateways: []*structs.BoundAPIGatewayConfigEntry{},
			expectedReferenceErrors:  map[structs.ResourceReference]error{},
			expectedError:            nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			actualBoundAPIGateways, actualReferenceErrors, actualError := BindGateways(tc.store, tc.route)

			require.Equal(t, tc.expectedBoundAPIGateways, actualBoundAPIGateways)
			require.Equal(t, tc.expectedReferenceErrors, actualReferenceErrors)
			require.Equal(t, tc.expectedError, actualError)
		})
	}
}

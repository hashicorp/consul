// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// Test_FetchService tests the FetchService method in scenarios where the RPC
// call succeeds and fails.
func Test_FetchVirtualIP(t *testing.T) {
	// set these to confirm that RPC call does not use them for this particular RPC
	rc := &config.RuntimeConfig{
		DNSAllowStale:  true,
		DNSMaxStale:    100,
		DNSUseCache:    true,
		DNSCacheMaxAge: 100,
	}
	tests := []struct {
		name           string
		queryPayload   *QueryPayload
		context        Context
		expectedResult *Result
		expectedErr    error
	}{
		{
			name: "FetchVirtualIP returns result",
			queryPayload: &QueryPayload{
				Name: "db",
				Tenancy: QueryTenancy{
					Peer:           "test-peer",
					EnterpriseMeta: defaultEntMeta,
				},
			},
			context: Context{
				Token: "test-token",
			},
			expectedResult: &Result{
				Address: "192.168.10.10",
				Type:    ResultTypeVirtual,
			},
			expectedErr: nil,
		},
		{
			name: "FetchVirtualIP returns error",
			queryPayload: &QueryPayload{
				Name: "db",
				Tenancy: QueryTenancy{
					Peer:           "test-peer",
					EnterpriseMeta: defaultEntMeta,
				},
			},
			context: Context{
				Token: "test-token",
			},
			expectedResult: nil,
			expectedErr:    errors.New("test-error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := testutil.Logger(t)
			mockRPC := cachetype.NewMockRPC(t)
			mockRPC.On("RPC", mock.Anything, "Catalog.VirtualIPForService", mock.Anything, mock.Anything).
				Return(tc.expectedErr).
				Run(func(args mock.Arguments) {
					req := args.Get(2).(*structs.ServiceSpecificRequest)

					// validate RPC options are not set from config for the VirtuaLIPForService RPC
					require.False(t, req.AllowStale)
					require.Equal(t, time.Duration(0), req.MaxStaleDuration)
					require.False(t, req.UseCache)
					require.Equal(t, time.Duration(0), req.MaxAge)

					// validate RPC options are set correctly from the queryPayload and context
					require.Equal(t, tc.queryPayload.Tenancy.Peer, req.PeerName)
					require.Equal(t, tc.queryPayload.Tenancy.EnterpriseMeta, req.EnterpriseMeta)
					require.Equal(t, tc.context.Token, req.QueryOptions.Token)

					if tc.expectedErr == nil {
						// set the out parameter to ensure that it is used to formulate the result.Address
						reply := args.Get(3).(*string)
						*reply = tc.expectedResult.Address
					}
				})
			df := NewV1DataFetcher(rc, acl.DefaultEnterpriseMeta(), mockRPC.RPC, logger)

			result, err := df.FetchVirtualIP(tc.context, tc.queryPayload)
			require.Equal(t, tc.expectedErr, err)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

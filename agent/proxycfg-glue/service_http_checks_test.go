// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfgglue

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
)

func TestServerHTTPChecks(t *testing.T) {
	var (
		ctx           = context.Background()
		svcID         = "web-sidecar-proxy-1"
		correlationID = "correlation-id"
		ch            = make(chan<- proxycfg.UpdateEvent)
		cacheResult   = errors.New("KABOOM")
		nodeName      = "server-1"
	)

	type testCase struct {
		name                string
		serviceInLocalState bool
		req                 *cachetype.ServiceHTTPChecksRequest
		expectedResult      error
	}

	run := func(t *testing.T, tc testCase) {
		serviceID := structs.NewServiceID(svcID, nil)
		localState := testLocalState(t)
		mockCacheSource := newMockServiceHTTPChecks(t)
		if tc.serviceInLocalState {
			require.NoError(t, localState.AddServiceWithChecks(&structs.NodeService{ID: serviceID.ID}, nil, "", false))
		}
		if tc.req.NodeName == nodeName && tc.serviceInLocalState {
			mockCacheSource.On("Notify", ctx, tc.req, correlationID, ch).Return(cacheResult)
		} else {
			mockCacheSource.AssertNotCalled(t, "Notify")
		}

		dataSource := ServerHTTPChecks(ServerDataSourceDeps{Logger: hclog.NewNullLogger()}, nodeName, mockCacheSource, localState)
		err := dataSource.Notify(ctx, tc.req, correlationID, ch)
		require.Equal(t, tc.expectedResult, err)
	}

	testcases := []testCase{
		{
			name:                "delegate to cache source if service in local state of the server node",
			serviceInLocalState: true,
			req:                 &cachetype.ServiceHTTPChecksRequest{ServiceID: svcID, NodeName: nodeName},
			expectedResult:      cacheResult,
		},
		{
			name:                "no-op if service not in local state of server node",
			serviceInLocalState: false,
			req:                 &cachetype.ServiceHTTPChecksRequest{ServiceID: svcID, NodeName: nodeName},
			expectedResult:      nil,
		},
		{
			name:                "no-op if service with same ID in local state but belongs to different node",
			serviceInLocalState: true,
			req:                 &cachetype.ServiceHTTPChecksRequest{ServiceID: svcID, NodeName: "server-2"},
			expectedResult:      nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}

}

func newMockServiceHTTPChecks(t *testing.T) *mockServiceHTTPChecks {
	mock := &mockServiceHTTPChecks{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

type mockServiceHTTPChecks struct {
	mock.Mock
}

func (m *mockServiceHTTPChecks) Notify(ctx context.Context, req *cachetype.ServiceHTTPChecksRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	return m.Called(ctx, req, correlationID, ch).Error(0)
}

func testLocalState(t *testing.T) *local.State {
	t.Helper()

	l := local.NewState(local.Config{}, hclog.NewNullLogger(), &token.Store{})
	l.TriggerSyncChanges = func() {}
	return l
}

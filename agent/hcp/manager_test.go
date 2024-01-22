// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"io"
	"testing"
	"time"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestManager_Start(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}
	upsertManagementTokenCalled := make(chan struct{}, 1)
	upsertManagementTokenF := func(name, secretID string) error {
		upsertManagementTokenCalled <- struct{}{}
		return nil
	}
	updateCh := make(chan struct{}, 1)
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Once()

	cloudCfg := config.CloudConfig{
		ResourceID:      "organization/85702e73-8a3d-47dc-291c-379b783c5804/project/8c0547c0-10e8-1ea2-dffe-384bee8da634/hashicorp.consul.global-network-manager.cluster/test",
		NodeID:          "node-1",
		ManagementToken: "fake-token",
	}
	scadaM := scada.NewMockProvider(t)
	scadaM.EXPECT().UpdateHCPConfig(cloudCfg).Return(nil).Once()
	scadaM.EXPECT().UpdateMeta(
		map[string]string{
			"consul_server_id": string(cloudCfg.NodeID),
		},
	).Return().Once()
	scadaM.EXPECT().Start().Return(nil)

	telemetryProvider := &hcpProviderImpl{
		httpCfg: &httpCfg{},
		logger:  hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		cfg:     defaultDisabledCfg(),
	}

	mockTelemetryCfg, err := testTelemetryCfg(&testConfig{
		refreshInterval: 1 * time.Second,
	})
	require.NoError(t, err)
	client.EXPECT().FetchTelemetryConfig(mock.Anything).Return(
		mockTelemetryCfg, nil).Maybe()

	mgr := NewManager(ManagerConfig{
		Logger:                    hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn:                  statusF,
		ManagementTokenUpserterFn: upsertManagementTokenF,
		SCADAProvider:             scadaM,
		TelemetryProvider:         telemetryProvider,
	})
	mgr.testUpdateSent = updateCh
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr.UpdateConfig(client, cloudCfg)
	mgr.Start(ctx)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	// Make sure after manager has stopped no more statuses are pushed.
	cancel()
	client.AssertExpectations(t)
	require.Equal(t, client, telemetryProvider.hcpClient)
	require.NotNil(t, telemetryProvider.GetHeader())
	require.NotNil(t, telemetryProvider.GetHTTPClient())
	require.NotEmpty(t, upsertManagementTokenCalled, "upsert management token function not called")
}

func TestManager_SendUpdate(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}
	updateCh := make(chan struct{}, 1)

	// Expect two calls, once during run startup and again when SendUpdate is called
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Twice()
	mgr := NewManager(ManagerConfig{
		Client:   client,
		Logger:   hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn: statusF,
	})
	mgr.testUpdateSent = updateCh

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr.Start(ctx)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	mgr.SendUpdate()
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	client.AssertExpectations(t)
}

func TestManager_SendUpdate_Periodic(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}
	updateCh := make(chan struct{}, 1)

	// Expect two calls, once during run startup and again when SendUpdate is called
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Twice()
	mgr := NewManager(ManagerConfig{
		Client:      client,
		Logger:      hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn:    statusF,
		MaxInterval: time.Second,
		MinInterval: 100 * time.Millisecond,
	})
	mgr.testUpdateSent = updateCh

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr.Start(ctx)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	client.AssertExpectations(t)
}

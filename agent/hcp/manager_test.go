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

func TestManager_StartMultipleTimes(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}

	updateCh := make(chan struct{}, 1)
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Once()

	cloudCfg := config.CloudConfig{
		ResourceID:      "organization/85702e73-8a3d-47dc-291c-379b783c5804/project/8c0547c0-10e8-1ea2-dffe-384bee8da634/hashicorp.consul.global-network-manager.cluster/test",
		NodeID:          "node-1",
		ManagementToken: "fake-token",
	}

	mgr := NewManager(ManagerConfig{
		Logger:   hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn: statusF,
	})

	mgr.testUpdateSent = updateCh
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Start the manager for the first time, expect one update
	mgr.UpdateConfig(client, cloudCfg)
	mgr.Start(ctx)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	// Start the manager again, don't expect an update since already running
	mgr.Start(ctx)
	select {
	case <-updateCh:
		require.Fail(t, "manager sent an update when not expected")
	case <-time.After(time.Second):
	}
}

func TestManager_UpdateConfig(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}

	updateCh := make(chan struct{}, 1)

	cloudCfg := config.CloudConfig{
		ResourceID: "organization/85702e73-8a3d-47dc-291c-379b783c5804/project/8c0547c0-10e8-1ea2-dffe-384bee8da634/hashicorp.consul.global-network-manager.cluster/test",
		NodeID:     "node-1",
	}

	mgr := NewManager(ManagerConfig{
		Logger:      hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn:    statusF,
		CloudConfig: cloudCfg,
		Client:      client,
	})

	mgr.testUpdateSent = updateCh
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Start the manager, expect an initial status update
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Once()
	mgr.Start(ctx)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	// Update the cloud configuration, expect a status update
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Once()
	updatedCfg := cloudCfg
	updatedCfg.ManagementToken = "token"
	mgr.UpdateConfig(client, updatedCfg)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	// Update the client, expect a status update
	updatedClient := hcpclient.NewMockClient(t)
	updatedClient.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Once()
	mgr.UpdateConfig(updatedClient, updatedCfg)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	// Update with the same values, don't expect a status update
	mgr.UpdateConfig(updatedClient, updatedCfg)
	select {
	case <-updateCh:
		require.Fail(t, "manager sent an update when not expected")
	case <-time.After(time.Second):
	}
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

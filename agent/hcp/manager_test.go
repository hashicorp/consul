// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/hashicorp/go-hclog"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/consul/sdk/testutil"
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
		ResourceID:      "resource-id",
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

	telemetryM := NewMockTelemetryProvider(t)
	telemetryM.EXPECT().Start(
		mock.Anything, &HCPProviderCfg{
			HCPClient: client,
			HCPConfig: &cloudCfg,
		},
	).Return(nil).Once()

	mgr := NewManager(
		ManagerConfig{
			Logger:                    hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			StatusFn:                  statusF,
			ManagementTokenUpserterFn: upsertManagementTokenF,
			SCADAProvider:             scadaM,
			TelemetryProvider:         telemetryM,
		},
	)
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

	select {
	case <-upsertManagementTokenCalled:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not upsert management token in expected time")
	}

	// Make sure after manager has stopped no more statuses are pushed.
	cancel()
	client.AssertExpectations(t)
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

	mgr := NewManager(
		ManagerConfig{
			Logger:   hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			StatusFn: statusF,
		},
	)

	mgr.testUpdateSent = updateCh
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Start the manager twice concurrently, expect only one update
	mgr.UpdateConfig(client, cloudCfg)
	go mgr.Start(ctx)
	go mgr.Start(ctx)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	select {
	case <-updateCh:
		require.Fail(t, "manager sent an update when not expected")
	case <-time.After(time.Second):
	}

	// Try start the manager again, still don't expect an update since already running
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

	mgr := NewManager(
		ManagerConfig{
			Logger:      hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			StatusFn:    statusF,
			CloudConfig: cloudCfg,
			Client:      client,
		},
	)

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
	mgr := NewManager(
		ManagerConfig{
			Client:   client,
			Logger:   hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			StatusFn: statusF,
		},
	)
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
	mgr := NewManager(
		ManagerConfig{
			Client:      client,
			Logger:      hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			StatusFn:    statusF,
			MaxInterval: time.Second,
			MinInterval: 100 * time.Millisecond,
		},
	)
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

func TestManager_Stop(t *testing.T) {
	client := hcpclient.NewMockClient(t)

	// Configure status functions called in sendUpdate
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}
	updateCh := make(chan struct{}, 1)
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Twice()

	// Configure management token creation and cleanup
	token := "test-token"
	upsertManagementTokenCalled := make(chan struct{}, 1)
	upsertManagementTokenF := func(name, secretID string) error {
		upsertManagementTokenCalled <- struct{}{}
		if secretID != token {
			return fmt.Errorf("expected token %q, got %q", token, secretID)
		}
		return nil
	}
	deleteManagementTokenCalled := make(chan struct{}, 1)
	deleteManagementTokenF := func(secretID string) error {
		deleteManagementTokenCalled <- struct{}{}
		if secretID != token {
			return fmt.Errorf("expected token %q, got %q", token, secretID)
		}
		return nil
	}

	// Configure the SCADA provider
	scadaM := scada.NewMockProvider(t)
	scadaM.EXPECT().UpdateHCPConfig(mock.Anything).Return(nil).Once()
	scadaM.EXPECT().UpdateMeta(mock.Anything).Return().Once()
	scadaM.EXPECT().Start().Return(nil).Once()
	scadaM.EXPECT().Stop().Return(nil).Once()

	// Configure the telemetry provider
	telemetryM := NewMockTelemetryProvider(t)
	telemetryM.EXPECT().Start(mock.Anything, mock.Anything).Return(nil).Once()
	telemetryM.EXPECT().Stop().Return().Once()

	// Configure manager with all its dependencies
	mgr := NewManager(
		ManagerConfig{
			Logger:                    testutil.Logger(t),
			StatusFn:                  statusF,
			Client:                    client,
			ManagementTokenUpserterFn: upsertManagementTokenF,
			ManagementTokenDeleterFn:  deleteManagementTokenF,
			SCADAProvider:             scadaM,
			TelemetryProvider:         telemetryM,
			CloudConfig: config.CloudConfig{
				ManagementToken: token,
			},
		},
	)
	mgr.testUpdateSent = updateCh
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Start the manager
	err := mgr.Start(ctx)
	require.NoError(t, err)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	select {
	case <-upsertManagementTokenCalled:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not create token in expected time")
	}

	// Send an update to ensure the manager is running in its main loop
	mgr.SendUpdate()
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	// Stop the manager
	err = mgr.Stop()
	require.NoError(t, err)

	// Validate that the management token delete function is called
	select {
	case <-deleteManagementTokenCalled:
	case <-time.After(time.Millisecond * 100):
		require.Fail(t, "manager did not create token in expected time")
	}

	// Send an update, expect no update since manager is stopped
	mgr.SendUpdate()
	select {
	case <-updateCh:
		require.Fail(t, "manager sent update after stopped")
	case <-time.After(time.Second):
	}
}

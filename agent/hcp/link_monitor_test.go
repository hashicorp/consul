// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/hcp/bootstrap/constants"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	hcpctl "github.com/hashicorp/consul/internal/hcp"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMonitorHCPLink_Ok(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	linkWatchCh := make(chan *pbresource.WatchEvent)
	mgr := NewMockManager(t)

	mockHCPClient := hcpclient.NewMockClient(t)
	mockHcpClientFn := func(_ config.CloudConfig) (hcpclient.Client, error) {
		return mockHCPClient, nil
	}

	loadMgmtTokenFn := func(ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string) (string, error) {
		return "test-mgmt-token", nil
	}

	dataDir := testutil.TempDir(t, "test-link-controller")
	os.Mkdir(filepath.Join(dataDir, constants.SubDir), os.ModeDir)
	existingCfg := config.CloudConfig{
		AuthURL: "test.com",
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		MonitorHCPLink(
			ctx, hclog.New(&hclog.LoggerOptions{Output: io.Discard}), mgr, linkWatchCh, mockHcpClientFn,
			loadMgmtTokenFn, existingCfg, dataDir,
		)
	}()

	// Set up a link
	link := pbhcp.Link{
		ResourceId:   "abc",
		ClientId:     "def",
		ClientSecret: "ghi",
		AccessLevel:  pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_WRITE,
	}
	expectedCfg := config.CloudConfig{
		ResourceID:      link.ResourceId,
		ClientID:        link.ClientId,
		ClientSecret:    link.ClientSecret,
		AuthURL:         "test.com",
		ManagementToken: "test-mgmt-token",
	}
	linkResource, err := anypb.New(&link)
	require.NoError(t, err)

	// Create link, expect HCP manager to be updated and started
	mgr.EXPECT().Start(mock.Anything).Return(nil).Once()
	mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg)
	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_Upsert_{
			Upsert: &pbresource.WatchEvent_Upsert{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Name: "global",
						Type: pbhcp.LinkType,
					},
					Status: map[string]*pbresource.Status{
						hcpctl.StatusKey: {
							Conditions: []*pbresource.Condition{hcpctl.ConditionValidatedSuccess},
						},
					},
					Data: linkResource,
				},
			},
		},
	}

	// Delete link, expect HCP manager to be stopped
	mgr.EXPECT().Stop().Return(nil).Once()
	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_Delete_{
			Delete: &pbresource.WatchEvent_Delete{},
		},
	}

	// Wait for MonitorHCPLink to return before assertions run
	close(linkWatchCh)
	wg.Wait()

	// Ensure hcp-config directory is removed
	file := filepath.Join(dataDir, constants.SubDir)
	if _, err := os.Stat(file); err == nil || !os.IsNotExist(err) {
		require.Fail(t, "should have removed hcp-config directory")
	}
}

func TestMonitorHCPLink_ValidationError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	linkWatchCh := make(chan *pbresource.WatchEvent)
	mgr := NewMockManager(t)

	mockHCPClient := hcpclient.NewMockClient(t)
	mockHcpClientFn := func(_ config.CloudConfig) (hcpclient.Client, error) {
		return mockHCPClient, nil
	}

	loadMgmtTokenFn := func(ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string) (string, error) {
		return "test-mgmt-token", nil
	}

	dataDir := testutil.TempDir(t, "test-link-controller")
	os.Mkdir(filepath.Join(dataDir, constants.SubDir), os.ModeDir)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		MonitorHCPLink(
			ctx, hclog.New(&hclog.LoggerOptions{Output: io.Discard}), mgr, linkWatchCh, mockHcpClientFn,
			loadMgmtTokenFn, config.CloudConfig{}, dataDir,
		)
	}()

	// Set up a link
	link := pbhcp.Link{
		ResourceId:   "abc",
		ClientId:     "def",
		ClientSecret: "ghi",
		AccessLevel:  pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_WRITE,
	}
	linkResource, err := anypb.New(&link)
	require.NoError(t, err)

	// Create link, expect HCP manager to be updated and started
	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_Upsert_{
			Upsert: &pbresource.WatchEvent_Upsert{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Name: "global",
						Type: pbhcp.LinkType,
					},
					Status: map[string]*pbresource.Status{
						hcpctl.StatusKey: {
							Conditions: []*pbresource.Condition{hcpctl.ConditionValidatedFailed},
						},
					},
					Data: linkResource,
				},
			},
		},
	}

	// Delete link, expect HCP manager to be stopped
	mgr.EXPECT().Stop().Return(nil).Once()
	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_Delete_{
			Delete: &pbresource.WatchEvent_Delete{},
		},
	}

	// Wait for MonitorHCPLink to return before assertions run
	close(linkWatchCh)
	wg.Wait()

	// Ensure hcp-config directory is removed
	file := filepath.Join(dataDir, constants.SubDir)
	if _, err := os.Stat(file); err == nil || !os.IsNotExist(err) {
		require.Fail(t, "should have removed hcp-config directory")
	}
}

func TestMonitorHCPLink_Ok_ReadOnly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	linkWatchCh := make(chan *pbresource.WatchEvent)
	mgr := NewMockManager(t)

	mockHCPClient := hcpclient.NewMockClient(t)
	mockHcpClientFn := func(_ config.CloudConfig) (hcpclient.Client, error) {
		return mockHCPClient, nil
	}

	loadMgmtTokenFn := func(ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string) (string, error) {
		return "test-mgmt-token", nil
	}
	existingManagerCfg := config.CloudConfig{
		AuthURL: "test.com",
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		MonitorHCPLink(
			ctx, hclog.New(&hclog.LoggerOptions{Output: io.Discard}), mgr, linkWatchCh, mockHcpClientFn,
			loadMgmtTokenFn, existingManagerCfg, "",
		)
	}()

	// Set up a link with READ_ONLY AccessLevel
	// In this case, we don't expect the HCP manager to be updated with any management token
	link := pbhcp.Link{
		ResourceId:   "abc",
		ClientId:     "def",
		ClientSecret: "ghi",
		AccessLevel:  pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_ONLY,
	}
	expectedCfg := config.CloudConfig{
		ResourceID:      link.ResourceId,
		ClientID:        link.ClientId,
		ClientSecret:    link.ClientSecret,
		AuthURL:         "test.com",
		ManagementToken: "",
	}
	linkResource, err := anypb.New(&link)
	require.NoError(t, err)

	// Create link, expect HCP manager to be updated and started
	mgr.EXPECT().Start(mock.Anything).Return(nil).Once()
	mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg)
	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_Upsert_{
			Upsert: &pbresource.WatchEvent_Upsert{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Name: "global",
						Type: pbhcp.LinkType,
					},
					Status: map[string]*pbresource.Status{
						hcpctl.StatusKey: {
							Conditions: []*pbresource.Condition{hcpctl.ConditionValidatedSuccess},
						},
					},
					Data: linkResource,
				},
			},
		},
	}

	// Wait for MonitorHCPLink to return before assertions run
	close(linkWatchCh)
	wg.Wait()
}

func TestMonitorHCPLink_EndOfSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	linkWatchCh := make(chan *pbresource.WatchEvent)
	mgr := NewMockManager(t)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		MonitorHCPLink(
			ctx, hclog.New(&hclog.LoggerOptions{Output: io.Discard}), mgr, linkWatchCh, nil,
			nil, config.CloudConfig{}, "",
		)
	}()

	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_EndOfSnapshot_{
			EndOfSnapshot: &pbresource.WatchEvent_EndOfSnapshot{},
		},
	}

	// Wait for MonitorHCPLink to return before assertions run
	close(linkWatchCh)
	wg.Wait()
}

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
	existingManagerCfg := config.CloudConfig{
		AuthURL: "test.com",
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
	mgr.EXPECT().GetCloudConfig().Return(existingManagerCfg)
	mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg)
	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_Upsert_{
			Upsert: &pbresource.WatchEvent_Upsert{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Name: "global",
						Type: pbhcp.LinkType,
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

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		MonitorHCPLink(
			ctx, hclog.New(&hclog.LoggerOptions{Output: io.Discard}), mgr, linkWatchCh, mockHcpClientFn,
			loadMgmtTokenFn, config.CloudConfig{}, "",
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
	existingManagerCfg := config.CloudConfig{
		AuthURL: "test.com",
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
	mgr.EXPECT().GetCloudConfig().Return(existingManagerCfg)
	mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg)
	linkWatchCh <- &pbresource.WatchEvent{
		Event: &pbresource.WatchEvent_Upsert_{
			Upsert: &pbresource.WatchEvent_Upsert{
				Resource: &pbresource.Resource{
					Id: &pbresource.ID{
						Name: "global",
						Type: pbhcp.LinkType,
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

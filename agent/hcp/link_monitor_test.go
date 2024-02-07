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

func TestMonitorHCPLink(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mockHCPClient := hcpclient.NewMockClient(t)
	mockHcpClientFn := func(_ config.CloudConfig) (hcpclient.Client, error) {
		return mockHCPClient, nil
	}

	loadMgmtTokenFn := func(ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string) (string, error) {
		return "test-mgmt-token", nil
	}

	dataDir := testutil.TempDir(t, "test-link-controller")
	err := os.Mkdir(filepath.Join(dataDir, constants.SubDir), os.ModeDir)
	require.NoError(t, err)
	existingCfg := config.CloudConfig{
		AuthURL: "test.com",
	}

	type testCase struct {
		mutateLink              func(*pbhcp.Link)
		mutateUpsertEvent       func(*pbresource.WatchEvent_Upsert)
		applyMocksAndAssertions func(*testing.T, *MockManager, *pbhcp.Link)
	}

	testCases := map[string]testCase{
		// HCP manager should be started when link is created and stopped when link is deleted
		"Ok": {
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.EXPECT().Start(mock.Anything).Return(nil).Once()

				expectedCfg := config.CloudConfig{
					ResourceID:      link.ResourceId,
					ClientID:        link.ClientId,
					ClientSecret:    link.ClientSecret,
					AuthURL:         "test.com",
					ManagementToken: "test-mgmt-token",
				}
				mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg)

				mgr.EXPECT().Stop().Return(nil).Once()
			},
		},
		// HCP manager should not be updated with management token
		"ReadOnly": {
			mutateLink: func(link *pbhcp.Link) {
				link.AccessLevel = pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_ONLY
			},
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.EXPECT().Start(mock.Anything).Return(nil).Once()

				expectedCfg := config.CloudConfig{
					ResourceID:      link.ResourceId,
					ClientID:        link.ClientId,
					ClientSecret:    link.ClientSecret,
					AuthURL:         "test.com",
					ManagementToken: "",
				}
				mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg)

				mgr.EXPECT().Stop().Return(nil).Once()
			},
		},
		// HCP manager should not be started or updated if link is not validated
		"ValidationError": {
			mutateUpsertEvent: func(upsert *pbresource.WatchEvent_Upsert) {
				upsert.Resource.Status = map[string]*pbresource.Status{
					hcpctl.StatusKey: {
						Conditions: []*pbresource.Condition{hcpctl.ConditionValidatedFailed},
					},
				}
			},
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.AssertNotCalled(t, "Start", mock.Anything)
				mgr.AssertNotCalled(t, "UpdateConfig", mock.Anything, mock.Anything)
				mgr.EXPECT().Stop().Return(nil).Once()
			},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t2 *testing.T) {
			mgr := NewMockManager(t2)

			// Set up a link
			link := pbhcp.Link{
				ResourceId:   "abc",
				ClientId:     "def",
				ClientSecret: "ghi",
				AccessLevel:  pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_WRITE,
			}

			if test.mutateLink != nil {
				test.mutateLink(&link)
			}

			linkResource, err := anypb.New(&link)
			require.NoError(t2, err)

			if test.applyMocksAndAssertions != nil {
				test.applyMocksAndAssertions(t2, mgr, &link)
			}

			linkWatchCh := make(chan *pbresource.WatchEvent)

			// Start MonitorHCPLink
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				MonitorHCPLink(
					ctx, hclog.New(&hclog.LoggerOptions{Output: io.Discard}), mgr, linkWatchCh, mockHcpClientFn,
					loadMgmtTokenFn, existingCfg, dataDir,
				)
			}()

			upsertEvent := &pbresource.WatchEvent_Upsert{
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
			}
			if test.mutateUpsertEvent != nil {
				test.mutateUpsertEvent(upsertEvent)
			}

			linkWatchCh <- &pbresource.WatchEvent{
				Event: &pbresource.WatchEvent_Upsert_{
					Upsert: upsertEvent,
				},
			}

			// Delete link, expect HCP manager to be stopped
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
				require.Fail(t2, "should have removed hcp-config directory")
			}
		})
	}
}

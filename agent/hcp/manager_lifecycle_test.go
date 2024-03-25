// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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

func TestHCPManagerLifecycleFn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := hclog.New(&hclog.LoggerOptions{Output: io.Discard})

	mockHCPClient := hcpclient.NewMockClient(t)
	mockHcpClientFn := func(_ config.CloudConfig) (hcpclient.Client, error) {
		return mockHCPClient, nil
	}

	mockLoadMgmtTokenFn := func(ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string) (string, error) {
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
		hcpClientFn             func(config.CloudConfig) (hcpclient.Client, error)
		loadMgmtTokenFn         func(context.Context, hclog.Logger, hcpclient.Client, string) (string, error)
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
				mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg).Once()

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
				mgr.EXPECT().UpdateConfig(mockHCPClient, expectedCfg).Once()

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
		"Error_InvalidLink": {
			mutateUpsertEvent: func(upsert *pbresource.WatchEvent_Upsert) {
				upsert.Resource = nil
			},
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.AssertNotCalled(t, "Start", mock.Anything)
				mgr.AssertNotCalled(t, "UpdateConfig", mock.Anything, mock.Anything)
				mgr.EXPECT().Stop().Return(nil).Once()
			},
		},
		"Error_HCPManagerStop": {
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.EXPECT().Start(mock.Anything).Return(nil).Once()
				mgr.EXPECT().UpdateConfig(mock.Anything, mock.Anything).Return().Once()
				mgr.EXPECT().Stop().Return(errors.New("could not stop HCP manager")).Once()
			},
		},
		"Error_CreatingHCPClient": {
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.AssertNotCalled(t, "Start", mock.Anything)
				mgr.AssertNotCalled(t, "UpdateConfig", mock.Anything, mock.Anything)
				mgr.EXPECT().Stop().Return(nil).Once()
			},
			hcpClientFn: func(_ config.CloudConfig) (hcpclient.Client, error) {
				return nil, errors.New("could not create HCP client")
			},
		},
		// This should result in the HCP manager not being started
		"Error_LoadMgmtToken": {
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.AssertNotCalled(t, "Start", mock.Anything)
				mgr.AssertNotCalled(t, "UpdateConfig", mock.Anything, mock.Anything)
				mgr.EXPECT().Stop().Return(nil).Once()
			},
			loadMgmtTokenFn: func(ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string) (string, error) {
				return "", errors.New("could not load management token")
			},
		},
		"Error_HCPManagerStart": {
			applyMocksAndAssertions: func(t *testing.T, mgr *MockManager, link *pbhcp.Link) {
				mgr.EXPECT().Start(mock.Anything).Return(errors.New("could not start HCP manager")).Once()
				mgr.EXPECT().UpdateConfig(mock.Anything, mock.Anything).Return().Once()
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

			testHcpClientFn := mockHcpClientFn
			if test.hcpClientFn != nil {
				testHcpClientFn = test.hcpClientFn
			}

			testLoadMgmtToken := mockLoadMgmtTokenFn
			if test.loadMgmtTokenFn != nil {
				testLoadMgmtToken = test.loadMgmtTokenFn
			}

			updateManagerLifecycle := HCPManagerLifecycleFn(
				mgr, testHcpClientFn,
				testLoadMgmtToken, existingCfg, dataDir,
			)

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

			// Handle upsert event
			updateManagerLifecycle(ctx, logger, &pbresource.WatchEvent{
				Event: &pbresource.WatchEvent_Upsert_{
					Upsert: upsertEvent,
				},
			})

			// Handle delete event. This should stop HCP manager
			updateManagerLifecycle(ctx, logger, &pbresource.WatchEvent{
				Event: &pbresource.WatchEvent_Delete_{
					Delete: &pbresource.WatchEvent_Delete{},
				},
			})

			// Ensure hcp-config directory is removed
			file := filepath.Join(dataDir, constants.SubDir)
			if _, err := os.Stat(file); err == nil || !os.IsNotExist(err) {
				require.Fail(t2, "should have removed hcp-config directory")
			}
		})
	}
}

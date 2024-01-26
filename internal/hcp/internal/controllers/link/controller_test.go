// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package link

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-uuid"
	gnmmod "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/agent/hcp/bootstrap"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	tenancies []*pbresource.Tenancy
	dataDir   string
}

func mockHcpClientFn(t *testing.T) (*hcpclient.MockClient, HCPClientFn) {
	mockClient := hcpclient.NewMockClient(t)

	mockClientFunc := func(link *pbhcp.Link) (hcpclient.Client, error) {
		return mockClient, nil
	}

	return mockClient, mockClientFunc
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	suite.tenancies = rtest.TestTenancies()
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register).
		WithTenancies(suite.tenancies...).
		Run(suite.T())

	suite.rt = controller.Runtime{
		Client: client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.client = rtest.NewClient(client)
}

func TestLinkController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func (suite *controllerSuite) deleteResourceFunc(id *pbresource.ID) func() {
	return func() {
		suite.client.MustDelete(suite.T(), id)

		// Ensure hcp-config directory is removed
		retry.Run(suite.T(), func(r *retry.R) {
			if suite.dataDir != "" {
				file := filepath.Join(suite.dataDir, bootstrap.SubDir)
				if _, err := os.Stat(file); !os.IsNotExist(err) {
					r.Fatalf("should have removed hcp-config directory")
				}
			}
		})

		suite.client.WaitForDeletion(suite.T(), id)
	}
}

func (suite *controllerSuite) TestController_Ok() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mockClient, mockClientFn := mockHcpClientFn(suite.T())
	readWrite := gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevelCONSULACCESSLEVELGLOBALREADWRITE
	mockClient.EXPECT().GetCluster(mock.Anything).Return(&hcpclient.Cluster{
		HCPPortalURL: "http://test.com",
		AccessLevel:  &readWrite,
	}, nil)

	token, err := uuid.GenerateUUID()
	require.NoError(suite.T(), err)
	mockClient.EXPECT().FetchBootstrap(mock.Anything).
		Return(&hcpclient.BootstrapConfig{
			ManagementToken: token,
			ConsulConfig:    "{}",
		}, nil).Once()

	dataDir := testutil.TempDir(suite.T(), "test-link-controller")
	suite.dataDir = dataDir
	mgr.Register(LinkController(
		false,
		false,
		mockClientFn,
		config.CloudConfig{},
		dataDir,
	))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   types.GenerateTestResourceID(suite.T()),
	}

	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

	// Ensure finalizer was added
	suite.client.WaitForResourceState(suite.T(), link.Id, func(t rtest.T, res *pbresource.Resource) {
		require.True(t, resource.HasFinalizer(res, StatusKey), "link resource does not have finalizer")
	})

	suite.client.WaitForStatusCondition(suite.T(), link.Id, StatusKey, ConditionLinked(linkData.ResourceId))
	var updatedLink pbhcp.Link
	updatedLinkResource := suite.client.WaitForNewVersion(suite.T(), link.Id, link.Version)
	require.NoError(suite.T(), updatedLinkResource.Data.UnmarshalTo(&updatedLink))
	require.Equal(suite.T(), "http://test.com", updatedLink.HcpClusterUrl)
	require.Equal(suite.T(), pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_WRITE, updatedLink.AccessLevel)
}

func (suite *controllerSuite) TestController_Initialize() {
	// Run the controller manager with a configured link
	mgr := controller.NewManager(suite.client, suite.rt.Logger)

	mockClient, mockClientFn := mockHcpClientFn(suite.T())
	readOnly := gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevelCONSULACCESSLEVELGLOBALREADONLY
	mockClient.EXPECT().GetCluster(mock.Anything).Return(&hcpclient.Cluster{
		HCPPortalURL: "http://test.com",
		AccessLevel:  &readOnly,
	}, nil)

	cloudCfg := config.CloudConfig{
		ClientID:     "client-id-abc",
		ClientSecret: "client-secret-abc",
		ResourceID:   types.GenerateTestResourceID(suite.T()),
	}

	dataDir := testutil.TempDir(suite.T(), "test-link-controller")
	suite.dataDir = dataDir

	mgr.Register(LinkController(
		false,
		false,
		mockClientFn,
		cloudCfg,
		dataDir,
	))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	// Wait for link to be created by initializer
	id := &pbresource.ID{
		Type: pbhcp.LinkType,
		Name: types.LinkName,
	}
	suite.T().Cleanup(suite.deleteResourceFunc(id))
	r := suite.client.WaitForResourceExists(suite.T(), id)

	// Check that created link has expected values
	var link pbhcp.Link
	err := r.Data.UnmarshalTo(&link)
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), cloudCfg.ResourceID, link.ResourceId)
	require.Equal(suite.T(), cloudCfg.ClientID, link.ClientId)
	require.Equal(suite.T(), cloudCfg.ClientSecret, link.ClientSecret)
	require.Equal(suite.T(), types.MetadataSourceConfig, r.Metadata[types.MetadataSourceKey])

	// Wait for link to be connected successfully
	suite.client.WaitForStatusCondition(suite.T(), id, StatusKey, ConditionLinked(link.ResourceId))
}

func (suite *controllerSuite) TestControllerResourceApisEnabled_LinkDisabled() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	_, mockClientFunc := mockHcpClientFn(suite.T())
	dataDir := testutil.TempDir(suite.T(), "test-link-controller")
	suite.dataDir = dataDir
	mgr.Register(LinkController(
		true,
		false,
		mockClientFunc,
		config.CloudConfig{},
		dataDir,
	))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   types.GenerateTestResourceID(suite.T()),
	}
	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

	suite.client.WaitForStatusCondition(suite.T(), link.Id, StatusKey, ConditionDisabled)
}

func (suite *controllerSuite) TestControllerResourceApisEnabledWithOverride_LinkNotDisabled() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mockClient, mockClientFunc := mockHcpClientFn(suite.T())
	mockClient.EXPECT().GetCluster(mock.Anything).Return(&hcpclient.Cluster{
		HCPPortalURL: "http://test.com",
	}, nil)

	token, err := uuid.GenerateUUID()
	require.NoError(suite.T(), err)
	mockClient.EXPECT().FetchBootstrap(mock.Anything).
		Return(&hcpclient.BootstrapConfig{
			ManagementToken: token,
			ConsulConfig:    "{}",
		}, nil).Once()

	dataDir := testutil.TempDir(suite.T(), "test-link-controller")
	suite.dataDir = dataDir

	mgr.Register(LinkController(
		true,
		true,
		mockClientFunc,
		config.CloudConfig{},
		dataDir,
	))

	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   types.GenerateTestResourceID(suite.T()),
	}
	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

	suite.client.WaitForStatusCondition(suite.T(), link.Id, StatusKey, ConditionLinked(linkData.ResourceId))
}

func (suite *controllerSuite) TestController_GetClusterError() {
	type testCase struct {
		expectErr       error
		expectCondition *pbresource.Condition
	}
	tt := map[string]testCase{
		"unexpected": {
			expectErr:       fmt.Errorf("error"),
			expectCondition: ConditionFailed,
		},
		"unauthorized": {
			expectErr:       hcpclient.ErrUnauthorized,
			expectCondition: ConditionUnauthorized,
		},
		"forbidden": {
			expectErr:       hcpclient.ErrForbidden,
			expectCondition: ConditionForbidden,
		},
	}

	for name, tc := range tt {
		suite.T().Run(name, func(t *testing.T) {
			// Run the controller manager
			mgr := controller.NewManager(suite.client, suite.rt.Logger)
			mockClient, mockClientFunc := mockHcpClientFn(suite.T())
			mockClient.EXPECT().GetCluster(mock.Anything).Return(nil, tc.expectErr)

			dataDir := testutil.TempDir(suite.T(), "test-link-controller")
			suite.dataDir = dataDir
			mgr.Register(LinkController(
				true,
				true,
				mockClientFunc,
				config.CloudConfig{},
				dataDir,
			))

			mgr.SetRaftLeader(true)
			go mgr.Run(suite.ctx)

			linkData := &pbhcp.Link{
				ClientId:     "abc",
				ClientSecret: "abc",
				ResourceId:   types.GenerateTestResourceID(suite.T()),
			}
			link := rtest.Resource(pbhcp.LinkType, "global").
				WithData(suite.T(), linkData).
				Write(suite.T(), suite.client)

			suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

			suite.client.WaitForStatusCondition(suite.T(), link.Id, StatusKey, tc.expectCondition)
		})
	}
}

func Test_hcpAccessModeToConsul(t *testing.T) {
	type testCase struct {
		hcpAccessLevel    *gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevel
		consulAccessLevel pbhcp.AccessLevel
	}
	tt := map[string]testCase{
		"unspecified": {
			hcpAccessLevel: func() *gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevel {
				t := gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevelCONSULACCESSLEVELUNSPECIFIED
				return &t
			}(),
			consulAccessLevel: pbhcp.AccessLevel_ACCESS_LEVEL_UNSPECIFIED,
		},
		"invalid": {
			hcpAccessLevel:    nil,
			consulAccessLevel: pbhcp.AccessLevel_ACCESS_LEVEL_UNSPECIFIED,
		},
		"read_only": {
			hcpAccessLevel: func() *gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevel {
				t := gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevelCONSULACCESSLEVELGLOBALREADONLY
				return &t
			}(),
			consulAccessLevel: pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_ONLY,
		},
		"read_write": {
			hcpAccessLevel: func() *gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevel {
				t := gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevelCONSULACCESSLEVELGLOBALREADWRITE
				return &t
			}(),
			consulAccessLevel: pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_WRITE,
		},
	}
	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			accessLevel := hcpAccessLevelToConsul(tc.hcpAccessLevel)
			require.Equal(t, tc.consulAccessLevel, accessLevel)
		})
	}
}

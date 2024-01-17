// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package link

import (
	"context"
	"fmt"

	gnmmod "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
)

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	ctl       linkReconciler
	tenancies []*pbresource.Tenancy
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
	suite.tenancies = resourcetest.TestTenancies()
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
	}
}

func (suite *controllerSuite) TestController_Ok() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mockClient, mockClientFn := mockHcpClientFn(suite.T())
	readOnly := gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevelCONSULACCESSLEVELGLOBALREADONLY
	mockClient.EXPECT().GetCluster(mock.Anything).Return(&hcpclient.Cluster{
		HCPPortalURL: "http://test.com",
		AccessLevel:  &readOnly,
	}, nil)
	mgr.Register(LinkController(false, false, mockClientFn))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}

	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

	suite.client.WaitForStatusCondition(suite.T(), link.Id, StatusKey, ConditionLinked(linkData.ResourceId))
	var updatedLink pbhcp.Link
	updatedLinkResource := suite.client.WaitForNewVersion(suite.T(), link.Id, link.Version)
	require.NoError(suite.T(), updatedLinkResource.Data.UnmarshalTo(&updatedLink))
	require.Equal(suite.T(), "http://test.com", updatedLink.HcpClusterUrl)
	require.Equal(suite.T(), pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_ONLY, updatedLink.AccessLevel)
}

func (suite *controllerSuite) TestControllerResourceApisEnabled_LinkDisabled() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mockClient, mockClientFunc := mockHcpClientFn(suite.T())
	mockClient.EXPECT().GetCluster(mock.Anything).Return(&hcpclient.Cluster{}, nil)
	mgr.Register(LinkController(true, false, mockClientFunc))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
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

	mgr.Register(LinkController(true, true, mockClientFunc))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}
	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

	suite.client.WaitForStatusCondition(suite.T(), link.Id, StatusKey, ConditionLinked(linkData.ResourceId))
}

func (suite *controllerSuite) TestController_GetClusterError() {
	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mockClient, mockClientFunc := mockHcpClientFn(suite.T())
	mockClient.EXPECT().GetCluster(mock.Anything).Return(nil, fmt.Errorf("error"))

	mgr.Register(LinkController(true, true, mockClientFunc))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   "abc",
	}
	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		Write(suite.T(), suite.client)

	suite.T().Cleanup(suite.deleteResourceFunc(link.Id))

	suite.client.WaitForStatusCondition(suite.T(), link.Id, StatusKey, ConditionFailed)
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
			accessLevel := hcpAccessModeToConsul(tc.hcpAccessLevel)
			require.Equal(t, tc.consulAccessLevel, accessLevel)
		})
	}
}

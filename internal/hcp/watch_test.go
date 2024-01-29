// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"github.com/hashicorp/go-hclog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
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

	tenancies []*pbresource.Tenancy
	dataDir   string
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

func TestLinkWatch(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func (suite *controllerSuite) deleteResourceFunc(id *pbresource.ID) func() {
	return func() {
		suite.client.MustDelete(suite.T(), id)
	}
}

func (suite *controllerSuite) TestLinkWatch_Ok() {
	// Run the controller manager
	linkData := &pbhcp.Link{
		ClientId:     "abc",
		ClientSecret: "abc",
		ResourceId:   types.GenerateTestResourceID(suite.T()),
	}

	linkWatchCh, err := NewLinkWatch(suite.ctx, hclog.Default(), suite.client)
	require.NoError(suite.T(), err)

	link := rtest.Resource(pbhcp.LinkType, "global").
		WithData(suite.T(), linkData).
		Write(suite.T(), suite.client)

	select {
	case watchEvent := <-linkWatchCh:
		require.Equal(suite.T(), pbresource.WatchEvent_OPERATION_UPSERT, watchEvent.Operation)
		res := watchEvent.Resource
		var upsertedLink pbhcp.Link
		err := res.Data.UnmarshalTo(&upsertedLink)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), linkData.ClientId, upsertedLink.ClientId)
		require.Equal(suite.T(), linkData.ClientSecret, upsertedLink.ClientSecret)
		require.Equal(suite.T(), linkData.ResourceId, upsertedLink.ResourceId)
	case <-time.After(time.Second):
		require.Fail(suite.T(), "nothing emitted on link watch channel for upsert")
	}

	_, err = suite.client.Delete(suite.ctx, &pbresource.DeleteRequest{Id: link.Id})

	select {
	case watchEvent := <-linkWatchCh:
		require.Equal(suite.T(), pbresource.WatchEvent_OPERATION_DELETE, watchEvent.Operation)
	case <-time.After(time.Second):
		require.Fail(suite.T(), "nothing emitted on link watch channel for delete")
	}
}

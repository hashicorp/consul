// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
)

type cacheClientSuite struct {
	suite.Suite

	cache   Cache
	mclient *mockpbresource.ResourceServiceClient_Expecter
	client  pbresource.ResourceServiceClient

	album1 *pbresource.Resource
	album2 *pbresource.Resource
}

func (suite *cacheClientSuite) SetupTest() {
	suite.cache = New()

	// It would be difficult to use the inmem resource service here due to cyclical dependencies.
	// Any type registrations from other packages cannot be imported because those packages
	// will require the controller package which will require this cache package. The easiest
	// way of getting around this was to not use the real resource service and require type registrations.
	client := mockpbresource.NewResourceServiceClient(suite.T())
	suite.mclient = client.EXPECT()

	require.NoError(suite.T(), suite.cache.AddIndex(pbdemo.AlbumType, namePrefixIndexer()))
	require.NoError(suite.T(), suite.cache.AddIndex(pbdemo.AlbumType, releaseYearIndexer()))
	require.NoError(suite.T(), suite.cache.AddIndex(pbdemo.AlbumType, tracksIndexer()))

	suite.album1 = resourcetest.Resource(pbdemo.AlbumType, "one").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "one",
			YearOfRelease: 2023,
			Tracks:        []string{"foo", "bar", "baz"},
		}).
		Build()

	suite.album2 = resourcetest.Resource(pbdemo.AlbumType, "two").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "two",
			YearOfRelease: 2023,
			Tracks:        []string{"fangorn", "zoo"},
		}).
		Build()

	suite.cache.Insert(suite.album1)
	suite.cache.Insert(suite.album2)

	suite.client = NewCachedClient(suite.cache, client)
}

func (suite *cacheClientSuite) performWrite(res *pbresource.Resource, shouldError bool) {
	req := &pbresource.WriteRequest{
		Resource: res,
	}

	// Setup the expectation for the inner mocked client to receive the real request
	if shouldError {
		suite.mclient.Write(mock.Anything, req).
			Return(nil, fakeWrappedErr).
			Once()
	} else {
		suite.mclient.Write(mock.Anything, req).
			Return(&pbresource.WriteResponse{
				Resource: res,
			}, nil).
			Once()
	}

	// Now use the wrapper client to perform the request
	out, err := suite.client.Write(context.Background(), req)
	if shouldError {
		require.ErrorIs(suite.T(), err, fakeWrappedErr)
		require.Nil(suite.T(), out)
	} else {
		require.NoError(suite.T(), err)
		prototest.AssertDeepEqual(suite.T(), res, out.Resource)
	}
}

func (suite *cacheClientSuite) performDelete(id *pbresource.ID, shouldError bool) {
	req := &pbresource.DeleteRequest{
		Id: id,
	}

	// Setup the expectation for the inner mocked client to receive the real request
	if shouldError {
		suite.mclient.Delete(mock.Anything, req).
			Return(nil, fakeWrappedErr).
			Once()
	} else {
		suite.mclient.Delete(mock.Anything, req).
			Return(&pbresource.DeleteResponse{}, nil).
			Once()
	}

	// Now use the wrapper client to perform the request
	out, err := suite.client.Delete(context.Background(), req)
	if shouldError {
		require.ErrorIs(suite.T(), err, fakeWrappedErr)
		require.Nil(suite.T(), out)
	} else {
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), out)
	}
}

func (suite *cacheClientSuite) performWriteStatus(res *pbresource.Resource, key string, status *pbresource.Status, shouldError bool) {
	req := &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    key,
		Status: status,
	}

	// Setup the expectation for the inner mocked client to receive the real request
	if shouldError {
		suite.mclient.WriteStatus(mock.Anything, req).
			Return(nil, fakeWrappedErr).
			Once()
	} else {
		suite.mclient.WriteStatus(mock.Anything, req).
			Return(&pbresource.WriteStatusResponse{
				Resource: res,
			}, nil).
			Once()
	}

	// Now use the wrapper client to perform the request
	out, err := suite.client.WriteStatus(context.Background(), req)
	if shouldError {
		require.ErrorIs(suite.T(), err, fakeWrappedErr)
		require.Nil(suite.T(), out)
	} else {
		require.NoError(suite.T(), err)
		prototest.AssertDeepEqual(suite.T(), res, out.Resource)
	}
}

func (suite *cacheClientSuite) TestWrite_Ok() {
	newRes := resourcetest.ResourceID(suite.album1.Id).
		WithTenancy(&pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		}).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "changed",
			YearOfRelease: 2023,
			Tracks:        []string{"fangorn", "zoo"},
		}).
		Build()

	suite.performWrite(newRes, false)

	// now ensure the entry was updated in the cache
	res, err := suite.cache.Get(suite.album1.Id.Type, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	prototest.AssertDeepEqual(suite.T(), newRes, res)
}

func (suite *cacheClientSuite) TestWrite_Error() {
	newRes := resourcetest.ResourceID(suite.album1.Id).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "changed",
			YearOfRelease: 2023,
			Tracks:        []string{"fangorn", "zoo"},
		}).
		WithVersion("notaversion").
		Build()

	suite.performWrite(newRes, true)

	// now ensure the entry was not updated in the cache
	res, err := suite.cache.Get(suite.album1.Id.Type, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	prototest.AssertDeepEqual(suite.T(), suite.album1, res)
}

func (suite *cacheClientSuite) TestWriteStatus_Ok() {
	status := &pbresource.Status{ObservedGeneration: suite.album1.Generation}

	updatedRes := resourcetest.ResourceID(suite.album1.Id).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "changed",
			YearOfRelease: 2023,
			Tracks:        []string{"fangorn", "zoo"},
		}).
		WithStatus("testing", status).
		WithVersion("notaversion").
		Build()

	suite.performWriteStatus(updatedRes, "testing", status, false)

	// now ensure the entry was updated in the cache
	res, err := suite.cache.Get(suite.album1.Id.Type, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	_, updated := res.Status["testing"]
	require.True(suite.T(), updated)
}

func (suite *cacheClientSuite) TestWriteStatus_Error() {
	status := &pbresource.Status{ObservedGeneration: suite.album1.Generation}

	updatedRes := resourcetest.ResourceID(suite.album1.Id).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "changed",
			YearOfRelease: 2023,
			Tracks:        []string{"fangorn", "zoo"},
		}).
		WithStatus("testing", status).
		WithVersion("notaversion").
		Build()

	suite.performWriteStatus(updatedRes, "testing", status, true)

	// now ensure the entry was not updated in the cache
	res, err := suite.cache.Get(suite.album1.Id.Type, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	_, updated := res.Status["testing"]
	require.False(suite.T(), updated)
}

func (suite *cacheClientSuite) TestDelete_Ok() {
	suite.performDelete(suite.album1.Id, false)

	// now ensure the entry was removed from the cache
	res, err := suite.cache.Get(suite.album1.Id.Type, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), res)
}

func (suite *cacheClientSuite) TestDelete_Error() {
	suite.performDelete(suite.album1.Id, true)

	// now ensure the entry was NOT removed from the cache
	res, err := suite.cache.Get(suite.album1.Id.Type, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
}

func TestCacheClient(t *testing.T) {
	suite.Run(t, new(cacheClientSuite))
}

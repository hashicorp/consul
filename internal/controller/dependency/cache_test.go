// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache/cachemock"
	"github.com/hashicorp/consul/internal/controller/dependency/dependencymock"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type cacheSuite struct {
	suite.Suite

	cache *cachemock.ReadOnlyCache
	idMod *dependencymock.CacheIDModifier
	res   *pbresource.Resource
	rt    controller.Runtime
}

func (suite *cacheSuite) SetupTest() {
	suite.res = resourcetest.Resource(fakeMapType, "foo").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	suite.idMod = dependencymock.NewCacheIDModifier(suite.T())
	suite.cache = cachemock.NewReadOnlyCache(suite.T())
	suite.rt = controller.Runtime{
		Cache:  suite.cache,
		Logger: testutil.Logger(suite.T()),
	}
}

func (suite *cacheSuite) TestGetMapper_ModErr() {
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(nil, injectedErr).
		Once()

	reqs, err := CacheGetMapper(suite.res.Id.Type, "doesnt-matter", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), reqs)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestGetMapper_CacheErr() {
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	suite.cache.EXPECT().
		Get(suite.res.Id.Type, "fake-index", id).
		Return(nil, injectedErr).
		Once()

	reqs, err := CacheGetMapper(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), reqs)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestGetMapper_Ok() {
	out := resourcetest.Resource(altFakeResourceType, "blah").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	suite.cache.EXPECT().
		Get(suite.res.Id.Type, "fake-index", id).
		Return(out, nil).
		Once()

	reqs, err := CacheGetMapper(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), reqs, 1)
	prototest.AssertDeepEqual(suite.T(), out.Id, reqs[0].ID)
}

func (suite *cacheSuite) TestListMapper_ModErr() {
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(nil, injectedErr).
		Once()

	reqs, err := CacheListMapper(suite.res.Id.Type, "doesnt-matter", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), reqs)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestListMapper_CacheErr() {
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	suite.cache.EXPECT().
		ListIterator(suite.res.Id.Type, "fake-index", id).
		Return(nil, injectedErr).
		Once()

	reqs, err := CacheListMapper(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), reqs)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestListMapper_Ok() {
	out := resourcetest.Resource(altFakeResourceType, "blah").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	out2 := resourcetest.Resource(altFakeResourceType, "blah2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	mockIter := cachemock.NewResourceIterator(suite.T())
	mockIter.EXPECT().Next().Return(out).Once()
	mockIter.EXPECT().Next().Return(out2).Once()
	mockIter.EXPECT().Next().Return(nil).Once()

	suite.cache.EXPECT().
		ListIterator(suite.res.Id.Type, "fake-index", id).
		Return(mockIter, nil).
		Once()

	reqs, err := CacheListMapper(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), reqs, 2)
	expected := []controller.Request{
		{ID: out.Id},
		{ID: out2.Id},
	}
	prototest.AssertElementsMatch(suite.T(), expected, reqs)
}

func (suite *cacheSuite) TestParentsMapper_ModErr() {
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(nil, injectedErr).
		Once()

	reqs, err := CacheParentsMapper(suite.res.Id.Type, "doesnt-matter", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), reqs)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestParentsMapper_CacheErr() {
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	suite.cache.EXPECT().
		ParentsIterator(suite.res.Id.Type, "fake-index", id).
		Return(nil, injectedErr).
		Once()

	reqs, err := CacheParentsMapper(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), reqs)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestParentsMapper_Ok() {
	out := resourcetest.Resource(altFakeResourceType, "blah").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	out2 := resourcetest.Resource(altFakeResourceType, "blah2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	mockIter := cachemock.NewResourceIterator(suite.T())
	mockIter.EXPECT().Next().Return(out).Once()
	mockIter.EXPECT().Next().Return(out2).Once()
	mockIter.EXPECT().Next().Return(nil).Once()

	suite.cache.EXPECT().
		ParentsIterator(suite.res.Id.Type, "fake-index", id).
		Return(mockIter, nil).
		Once()

	reqs, err := CacheParentsMapper(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), reqs, 2)
	expected := []controller.Request{
		{ID: out.Id},
		{ID: out2.Id},
	}
	prototest.AssertElementsMatch(suite.T(), expected, reqs)
}

func (suite *cacheSuite) TestListTransform_ModErr() {
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(nil, injectedErr).
		Once()

	reqs, err := CacheListTransform(suite.res.Id.Type, "doesnt-matter", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), reqs)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestListTransform_CacheErr() {
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	suite.cache.EXPECT().
		List(suite.res.Id.Type, "fake-index", id).
		Return(nil, injectedErr).
		Once()

	resources, err := CacheListTransform(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.Nil(suite.T(), resources)
	require.ErrorIs(suite.T(), err, injectedErr)
}

func (suite *cacheSuite) TestListTransform_Ok() {
	out := resourcetest.Resource(altFakeResourceType, "blah").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	out2 := resourcetest.Resource(altFakeResourceType, "blah2").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
	id := resourceID("testing", "v1", "fake", "foo")
	suite.idMod.EXPECT().
		Execute(mock.Anything, suite.rt, suite.res.Id).
		Return(id, nil).
		Once()

	expected := []*pbresource.Resource{out, out2}
	suite.cache.EXPECT().
		List(suite.res.Id.Type, "fake-index", id).
		Return(expected, nil).
		Once()

	resources, err := CacheListTransform(suite.res.Id.Type, "fake-index", suite.idMod.Execute)(
		context.Background(),
		suite.rt,
		suite.res,
	)

	require.NoError(suite.T(), err)
	prototest.AssertElementsMatch(suite.T(), expected, resources)
}

func TestCacheDependencies(t *testing.T) {
	suite.Run(t, new(cacheSuite))
}

func TestReplaceCacheIDType(t *testing.T) {
	rt := controller.Runtime{
		// populate something to differentiate from zero value
		Logger: hclog.Default(),
	}

	in := resourceID("testing", "v1", "pre-mod", "foo")

	mod := ReplaceCacheIDType(fakeResourceType)

	out, err := mod(context.Background(), rt, in)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, resource.ReplaceType(fakeResourceType, in), out)

}

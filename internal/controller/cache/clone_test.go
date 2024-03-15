// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache_test

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/cachemock"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/suite"
)

var (
	injectedError = errors.New("injected")
	indexName     = "some-index"
)

type cloningReadOnlyCacheSuite struct {
	suite.Suite

	rtype *pbresource.Type
	res1  *pbresource.Resource
	res2  *pbresource.Resource

	mcache *cachemock.ReadOnlyCache
	ccache cache.ReadOnlyCache
}

func TestReadOnlyCache(t *testing.T) {
	suite.Run(t, new(cloningReadOnlyCacheSuite))
}

func (suite *cloningReadOnlyCacheSuite) SetupTest() {
	suite.rtype = &pbresource.Type{
		Group:        "testing",
		GroupVersion: "v1",
		Kind:         "Fake",
	}

	suite.res1 = resourcetest.Resource(suite.rtype, "foo").Build()
	suite.res2 = resourcetest.Resource(suite.rtype, "bar").Build()

	suite.mcache = cachemock.NewReadOnlyCache(suite.T())
	suite.ccache = cache.NewCloningReadOnlyCache(suite.mcache)
}

func (suite *cloningReadOnlyCacheSuite) makeMockIterator(resources ...*pbresource.Resource) *cachemock.ResourceIterator {
	iter := cachemock.NewResourceIterator(suite.T())
	for _, res := range resources {
		iter.EXPECT().
			Next().
			Return(res).
			Once()
	}

	iter.EXPECT().
		Next().
		Return(nil).
		Times(0)

	return iter
}

func (suite *cloningReadOnlyCacheSuite) requireEqualNotSame(expected, actual *pbresource.Resource) {
	suite.T().Helper()

	suite.Require().Equal(expected, actual)
	suite.Require().NotSame(expected, actual)
}

func (suite *cloningReadOnlyCacheSuite) TestGet_Ok() {
	suite.mcache.EXPECT().
		Get(suite.rtype, indexName, "ok").
		Return(suite.res1, nil)

	actual, err := suite.ccache.Get(suite.rtype, indexName, "ok")
	suite.Require().NoError(err)
	suite.requireEqualNotSame(suite.res1, actual)
}

func (suite *cloningReadOnlyCacheSuite) TestGet_Error() {
	suite.mcache.EXPECT().
		Get(suite.rtype, indexName, "error").
		Return(nil, injectedError)

	actual, err := suite.ccache.Get(suite.rtype, indexName, "error")
	suite.Require().ErrorIs(err, injectedError)
	suite.Require().Nil(actual)
}

func (suite *cloningReadOnlyCacheSuite) TestList_Ok() {
	preClone := []*pbresource.Resource{suite.res1, suite.res2}

	suite.mcache.EXPECT().
		List(suite.rtype, indexName, "ok").
		Return(preClone, nil)

	postClone, err := suite.ccache.List(suite.rtype, indexName, "ok")
	suite.Require().NoError(err)
	suite.Require().Len(postClone, len(preClone))
	for i, actual := range postClone {
		suite.requireEqualNotSame(preClone[i], actual)
	}
}

func (suite *cloningReadOnlyCacheSuite) TestList_Error() {
	suite.mcache.EXPECT().
		List(suite.rtype, indexName, "error").
		Return(nil, injectedError)

	actual, err := suite.ccache.List(suite.rtype, indexName, "error")
	suite.Require().ErrorIs(err, injectedError)
	suite.Require().Nil(actual)
}

func (suite *cloningReadOnlyCacheSuite) TestParents_Ok() {
	preClone := []*pbresource.Resource{suite.res1, suite.res2}

	suite.mcache.EXPECT().
		Parents(suite.rtype, indexName, "ok").
		Return(preClone, nil)

	postClone, err := suite.ccache.Parents(suite.rtype, indexName, "ok")
	suite.Require().NoError(err)
	suite.Require().Len(postClone, len(preClone))
	for i, actual := range postClone {
		suite.requireEqualNotSame(preClone[i], actual)
	}
}

func (suite *cloningReadOnlyCacheSuite) TestParents_Error() {
	suite.mcache.EXPECT().
		Parents(suite.rtype, indexName, "error").
		Return(nil, injectedError)

	actual, err := suite.ccache.Parents(suite.rtype, indexName, "error")
	suite.Require().ErrorIs(err, injectedError)
	suite.Require().Nil(actual)
}

func (suite *cloningReadOnlyCacheSuite) TestListIterator_Ok() {
	suite.mcache.EXPECT().
		ListIterator(suite.rtype, indexName, "ok").
		Return(suite.makeMockIterator(suite.res1, suite.res2), nil)

	iter, err := suite.ccache.ListIterator(suite.rtype, indexName, "ok")
	suite.Require().NoError(err)
	suite.Require().NotNil(iter)

	suite.requireEqualNotSame(suite.res1, iter.Next())
	suite.requireEqualNotSame(suite.res2, iter.Next())
	suite.Require().Nil(iter.Next())
}

func (suite *cloningReadOnlyCacheSuite) TestListIterator_Error() {
	suite.mcache.EXPECT().
		ListIterator(suite.rtype, indexName, "error").
		Return(nil, injectedError)

	actual, err := suite.ccache.ListIterator(suite.rtype, indexName, "error")
	suite.Require().ErrorIs(err, injectedError)
	suite.Require().Nil(actual)
}

func (suite *cloningReadOnlyCacheSuite) TestParentsIterator_Ok() {
	suite.mcache.EXPECT().
		ParentsIterator(suite.rtype, indexName, "ok").
		Return(suite.makeMockIterator(suite.res1, suite.res2), nil)

	iter, err := suite.ccache.ParentsIterator(suite.rtype, indexName, "ok")
	suite.Require().NoError(err)
	suite.Require().NotNil(iter)

	suite.requireEqualNotSame(suite.res1, iter.Next())
	suite.requireEqualNotSame(suite.res2, iter.Next())
	suite.Require().Nil(iter.Next())
}

func (suite *cloningReadOnlyCacheSuite) TestParentsIterator_Error() {
	suite.mcache.EXPECT().
		ParentsIterator(suite.rtype, indexName, "error").
		Return(nil, injectedError)

	actual, err := suite.ccache.ParentsIterator(suite.rtype, indexName, "error")
	suite.Require().ErrorIs(err, injectedError)
	suite.Require().Nil(actual)
}

func (suite *cloningReadOnlyCacheSuite) TestQuery_Ok() {
	suite.mcache.EXPECT().
		Query(indexName, "ok").
		Return(suite.makeMockIterator(suite.res1, suite.res2), nil)

	iter, err := suite.ccache.Query(indexName, "ok")
	suite.Require().NoError(err)
	suite.Require().NotNil(iter)

	suite.requireEqualNotSame(suite.res1, iter.Next())
	suite.requireEqualNotSame(suite.res2, iter.Next())
	suite.Require().Nil(iter.Next())
}

func (suite *cloningReadOnlyCacheSuite) TestQuery_Error() {
	suite.mcache.EXPECT().
		Query(indexName, "error").
		Return(nil, injectedError)

	actual, err := suite.ccache.Query(indexName, "error")
	suite.Require().ErrorIs(err, injectedError)
	suite.Require().Nil(actual)
}

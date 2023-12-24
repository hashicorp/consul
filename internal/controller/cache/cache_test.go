// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
)

const (
	errQueryName = "error-query"
	okQueryName  = "ok-query"
)

func namePrefixIndexer() *index.Index {
	return indexers.DecodedSingleIndexer(
		"name_prefix",
		index.SingleValueFromArgs(func(value string) ([]byte, error) {
			return []byte(value), nil
		}),
		func(r *resource.DecodedResource[*pbdemo.Album]) (bool, []byte, error) {
			return true, []byte(r.Data.Name), nil
		})
}

func releaseYearIndexer() *index.Index {
	return indexers.DecodedSingleIndexer(
		"year",
		index.SingleValueFromArgs(func(value int32) ([]byte, error) {
			var b index.Builder
			binary.Write(&b, binary.BigEndian, value)
			return b.Bytes(), nil
		}),
		func(r *resource.DecodedResource[*pbdemo.Album]) (bool, []byte, error) {
			var b index.Builder
			binary.Write(&b, binary.BigEndian, r.Data.YearOfRelease)
			return true, b.Bytes(), nil
		})
}

func tracksIndexer() *index.Index {
	return indexers.DecodedMultiIndexer(
		"tracks",
		index.SingleValueFromOneOrTwoArgs(func(value string, prefix bool) ([]byte, error) {
			var b index.Builder
			if prefix {
				b.Raw([]byte(value))
			} else {
				b.String(value)
			}
			return b.Bytes(), nil
		}),
		func(r *resource.DecodedResource[*pbdemo.Album]) (bool, [][]byte, error) {
			indexes := make([][]byte, len(r.Data.Tracks))
			for idx, track := range r.Data.Tracks {
				var b index.Builder
				b.String(track)
				indexes[idx] = b.Bytes()
			}

			return true, indexes, nil
		})
}

func requireCacheIndex(t *testing.T, c *cache, rtype *pbresource.Type, indexes ...string) {
	t.Helper()
	indices, err := c.getTypeIndices(rtype)
	require.NoError(t, err)
	require.NotNil(t, indices)

	for _, name := range indexes {
		index, err := indices.getIndex(name)
		require.NoError(t, err)
		require.NotNil(t, index)
	}
}

func TestCacheAddType(t *testing.T) {
	c := newCache()
	c.AddType(pbdemo.AlbumType)

	// Adding a type will ensure that the `id` index exists
	requireCacheIndex(t, c, pbdemo.AlbumType, "id")
}

func TestCacheAddIndex(t *testing.T) {
	c := newCache()
	require.NoError(t, c.AddIndex(pbdemo.AlbumType, releaseYearIndexer()))
	require.NoError(t, c.AddIndex(pbdemo.AlbumType, tracksIndexer()))

	// Adding indexes should also have the side effect of ensuring that the `id` index exists
	requireCacheIndex(t, c, pbdemo.AlbumType, "id", "year", "tracks")
}

func TestCacheAddIndex_Duplicate(t *testing.T) {
	c := newCache()
	require.NoError(t, c.AddIndex(pbdemo.AlbumType, releaseYearIndexer()))
	// should get an error due to a duplicate index name
	require.Error(t, c.AddIndex(pbdemo.AlbumType, releaseYearIndexer()))
}

func noopQuery(_ ReadOnlyCache, _ ...any) (ResourceIterator, error) {
	return nil, nil
}

func errQuery(_ ReadOnlyCache, _ ...any) (ResourceIterator, error) {
	return nil, injectedError
}

func TestCacheAddQuery(t *testing.T) {
	c := newCache()
	require.NoError(t, c.AddQuery("foo", noopQuery))
	require.NoError(t, c.AddQuery("bar", errQuery))

	fn, found := c.queries["foo"]
	require.True(t, found)
	iter, err := fn(c)
	require.NoError(t, err)
	require.Nil(t, iter)

	fn, found = c.queries["bar"]
	require.True(t, found)
	iter, err = fn(c)
	require.ErrorIs(t, err, injectedError)
	require.Nil(t, iter)
}

func TestCacheAddQuery_Duplicate(t *testing.T) {
	c := newCache()

	require.NoError(t, c.AddQuery("foo", noopQuery))
	// should get an error due to a duplicate query name
	require.Error(t, c.AddQuery("foo", noopQuery))
}

func TestCacheAddQuery_Nil(t *testing.T) {
	c := newCache()
	require.ErrorIs(t, c.AddQuery("foo", nil), QueryRequired)
}

func TestQuery_NotFound(t *testing.T) {
	c := newCache()
	iter, err := c.Query("foo", "something")
	require.ErrorIs(t, err, QueryNotFoundError{"foo"})
	require.Nil(t, iter)
}

func TestCache(t *testing.T) {
	suite.Run(t, &cacheSuite{})
}

type cacheSuite struct {
	suite.Suite
	c Cache

	album1 *pbresource.Resource
	album2 *pbresource.Resource
	album3 *pbresource.Resource
	album4 *pbresource.Resource
}

func (suite *cacheSuite) SetupTest() {
	suite.c = New()

	require.NoError(suite.T(), suite.c.AddIndex(pbdemo.AlbumType, namePrefixIndexer()))
	require.NoError(suite.T(), suite.c.AddQuery(okQueryName, func(c ReadOnlyCache, args ...any) (ResourceIterator, error) {
		return c.ParentsIterator(pbdemo.AlbumType, "name_prefix", args...)
	}))
	require.NoError(suite.T(), suite.c.AddIndex(pbdemo.AlbumType, releaseYearIndexer()))
	require.NoError(suite.T(), suite.c.AddQuery(errQueryName, errQuery))
	require.NoError(suite.T(), suite.c.AddIndex(pbdemo.AlbumType, tracksIndexer()))

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

	suite.album3 = resourcetest.Resource(pbdemo.AlbumType, "third").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "foo",
			YearOfRelease: 2022,
			Tracks:        []string{"blah", "something", "else"},
		}).
		Build()

	suite.album4 = resourcetest.Resource(pbdemo.AlbumType, "four").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(suite.T(), &pbdemo.Album{
			Name:          "food",
			YearOfRelease: 2020,
			Tracks:        []string{"nothing", "food"},
		}).
		Build()

	require.NoError(suite.T(), suite.c.Insert(suite.album1))
	require.NoError(suite.T(), suite.c.Insert(suite.album2))
	require.NoError(suite.T(), suite.c.Insert(suite.album3))
	require.NoError(suite.T(), suite.c.Insert(suite.album4))
}

func (suite *cacheSuite) TestGet() {
	res, err := suite.c.Get(pbdemo.AlbumType, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.album1, res)

	res, err = suite.c.Get(pbdemo.AlbumType, "year", int32(2022))
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.album3, res)

	res, err = suite.c.Get(pbdemo.AlbumType, "tracks", "fangorn")
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.album2, res)
}

func (suite *cacheSuite) TestGet_NilType() {
	res, err := suite.c.Get(nil, "id", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeUnspecifiedError)
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestGet_UncachedType() {
	res, err := suite.c.Get(pbdemo.ArtistType, "id", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeNotIndexedError)
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestGet_IndexNotFound() {
	res, err := suite.c.Get(pbdemo.AlbumType, "blah", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, IndexNotFoundError{name: "blah"})
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestList() {
	resources, err := suite.c.List(pbdemo.AlbumType, "year", int32(2023))
	require.NoError(suite.T(), err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album1, suite.album2}, resources)

	resources, err = suite.c.List(pbdemo.AlbumType, "tracks", "f", true)
	require.NoError(suite.T(), err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album1, suite.album2, suite.album4}, resources)
}

func (suite *cacheSuite) TestList_NilType() {
	res, err := suite.c.List(nil, "id", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeUnspecifiedError)
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestList_UncachedType() {
	res, err := suite.c.List(pbdemo.ArtistType, "id", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeNotIndexedError)
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestList_IndexNotFound() {
	res, err := suite.c.List(pbdemo.AlbumType, "blah", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, IndexNotFoundError{name: "blah"})
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestParents() {
	resources, err := suite.c.Parents(pbdemo.AlbumType, "name_prefix", "food")
	require.NoError(suite.T(), err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album3, suite.album4}, resources)
}

func (suite *cacheSuite) TestQuery() {
	resources, err := expandIterator(suite.c.Query(okQueryName, "food"))
	require.NoError(suite.T(), err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album3, suite.album4}, resources)
}

func (suite *cacheSuite) TestParents_NilType() {
	res, err := suite.c.Parents(nil, "id", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeUnspecifiedError)
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestParents_UncachedType() {
	res, err := suite.c.Parents(pbdemo.ArtistType, "id", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeNotIndexedError)
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestParents_IndexNotFound() {
	res, err := suite.c.Parents(pbdemo.AlbumType, "blah", suite.album1.Id)
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, IndexNotFoundError{name: "blah"})
	require.Nil(suite.T(), res)
}

func (suite *cacheSuite) TestInsert_UncachedType() {
	err := suite.c.Insert(resourcetest.Resource(pbdemo.ArtistType, "blah").Build())
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeNotIndexedError)
}

func (suite *cacheSuite) TestDelete() {
	err := suite.c.Delete(suite.album1)
	require.NoError(suite.T(), err)

	res, err := suite.c.Get(pbdemo.AlbumType, "id", suite.album1.Id)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), res)

	resources, err := suite.c.List(pbdemo.AlbumType, "year", int32(2023))
	require.NoError(suite.T(), err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album2}, resources)

	resources, err = suite.c.Parents(pbdemo.AlbumType, "name_prefix", "onesie")
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), resources)
}

func (suite *cacheSuite) TestDelete_UncachedType() {
	err := suite.c.Delete(resourcetest.Resource(pbdemo.ArtistType, "blah").Build())
	require.Error(suite.T(), err)
	require.ErrorIs(suite.T(), err, TypeNotIndexedError)
}

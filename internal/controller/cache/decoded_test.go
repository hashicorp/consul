// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/cachemock"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/proto/private/prototest"
)

type decodedSuite struct {
	suite.Suite
	rc   *cachemock.ReadOnlyCache
	iter *cachemock.ResourceIterator

	artistGood  *resource.DecodedResource[*pbdemo.Artist]
	artistGood2 *resource.DecodedResource[*pbdemo.Artist]
	artistBad   *pbresource.Resource
}

func (suite *decodedSuite) SetupTest() {
	suite.rc = cachemock.NewReadOnlyCache(suite.T())
	suite.iter = cachemock.NewResourceIterator(suite.T())
	artist, err := demo.GenerateV2Artist()
	require.NoError(suite.T(), err)
	suite.artistGood, err = resource.Decode[*pbdemo.Artist](artist)
	require.NoError(suite.T(), err)

	artist2, err := demo.GenerateV2Artist()
	require.NoError(suite.T(), err)
	suite.artistGood2, err = resource.Decode[*pbdemo.Artist](artist2)
	require.NoError(suite.T(), err)

	suite.artistBad, err = demo.GenerateV2Album(artist.Id)
	require.NoError(suite.T(), err)
}

func (suite *decodedSuite) TestGetDecoded_Ok() {
	suite.rc.EXPECT().Get(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(suite.artistGood.Resource, nil)

	dec, err := cache.GetDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec.Data)
}

func (suite *decodedSuite) TestGetDecoded_DecodeError() {
	suite.rc.EXPECT().Get(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(suite.artistBad, nil)

	dec, err := cache.GetDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestGetDecoded_CacheError() {
	suite.rc.EXPECT().Get(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, injectedError)

	dec, err := cache.GetDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.ErrorIs(suite.T(), err, injectedError)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestGetDecoded_Nil() {
	suite.rc.EXPECT().Get(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, nil)

	dec, err := cache.GetDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestListDecoded_Ok() {
	suite.rc.EXPECT().List(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return([]*pbresource.Resource{suite.artistGood.Resource, suite.artistGood2.Resource}, nil)

	dec, err := cache.ListDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), dec, 2)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec[0].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec[0].Data)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Resource, dec[1].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Data, dec[1].Data)
}

func (suite *decodedSuite) TestListDecoded_DecodeError() {
	suite.rc.EXPECT().List(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return([]*pbresource.Resource{suite.artistGood.Resource, suite.artistBad}, nil)

	dec, err := cache.ListDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestListDecoded_CacheError() {
	suite.rc.EXPECT().List(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, injectedError)

	dec, err := cache.ListDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.ErrorIs(suite.T(), err, injectedError)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestListDecoded_Nil() {
	suite.rc.EXPECT().List(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, nil)

	dec, err := cache.ListDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestListIteratorDecoded_Ok() {
	suite.iter.EXPECT().Next().Return(suite.artistGood.Resource).Once()
	suite.iter.EXPECT().Next().Return(suite.artistGood2.Resource).Once()
	suite.iter.EXPECT().Next().Return(nil).Times(0)
	suite.rc.EXPECT().ListIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return(suite.iter, nil)

	iter, err := cache.ListIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), iter)

	dec, err := iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec.Data)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Data, dec.Data)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestListIteratorDecoded_DecodeError() {
	suite.iter.EXPECT().Next().Return(suite.artistGood.Resource).Once()
	suite.iter.EXPECT().Next().Return(suite.artistBad).Once()
	suite.iter.EXPECT().Next().Return(nil).Times(0)
	suite.rc.EXPECT().ListIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return(suite.iter, nil)

	iter, err := cache.ListIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), iter)

	dec, err := iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec.Data)

	dec, err = iter.Next()
	require.Error(suite.T(), err)
	require.Nil(suite.T(), dec)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestListIteratorDecoded_CacheError() {
	suite.rc.EXPECT().ListIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, injectedError)

	iter, err := cache.ListIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.ErrorIs(suite.T(), err, injectedError)
	require.Nil(suite.T(), iter)
}

func (suite *decodedSuite) TestListIteratorDecoded_Nil() {
	suite.rc.EXPECT().ListIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, nil)

	dec, err := cache.ListIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestParentsDecoded_Ok() {
	suite.rc.EXPECT().Parents(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return([]*pbresource.Resource{suite.artistGood.Resource, suite.artistGood2.Resource}, nil)

	dec, err := cache.ParentsDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), dec, 2)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec[0].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec[0].Data)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Resource, dec[1].Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Data, dec[1].Data)
}

func (suite *decodedSuite) TestParentsDecoded_DecodeError() {
	suite.rc.EXPECT().Parents(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return([]*pbresource.Resource{suite.artistGood.Resource, suite.artistBad}, nil)

	dec, err := cache.ParentsDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestParentsDecoded_CacheError() {
	suite.rc.EXPECT().Parents(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, injectedError)

	dec, err := cache.ParentsDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.ErrorIs(suite.T(), err, injectedError)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestParentsDecoded_Nil() {
	suite.rc.EXPECT().Parents(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, nil)

	dec, err := cache.ParentsDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestParentsIteratorDecoded_Ok() {
	suite.iter.EXPECT().Next().Return(suite.artistGood.Resource).Once()
	suite.iter.EXPECT().Next().Return(suite.artistGood2.Resource).Once()
	suite.iter.EXPECT().Next().Return(nil).Times(0)
	suite.rc.EXPECT().ParentsIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return(suite.iter, nil)

	iter, err := cache.ParentsIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), iter)

	dec, err := iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec.Data)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Data, dec.Data)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestParentsIteratorDecoded_DecodeError() {
	suite.iter.EXPECT().Next().Return(suite.artistGood.Resource).Once()
	suite.iter.EXPECT().Next().Return(suite.artistBad).Once()
	suite.iter.EXPECT().Next().Return(nil).Times(0)
	suite.rc.EXPECT().ParentsIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).
		Return(suite.iter, nil)

	iter, err := cache.ParentsIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), iter)

	dec, err := iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec.Data)

	dec, err = iter.Next()
	require.Error(suite.T(), err)
	require.Nil(suite.T(), dec)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestParentsIteratorDecoded_CacheError() {
	suite.rc.EXPECT().ParentsIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, injectedError)

	iter, err := cache.ParentsIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.ErrorIs(suite.T(), err, injectedError)
	require.Nil(suite.T(), iter)
}

func (suite *decodedSuite) TestParentsIteratorDecoded_Nil() {
	suite.rc.EXPECT().ParentsIterator(pbdemo.ArtistType, "id", suite.artistGood.Id).Return(nil, nil)

	dec, err := cache.ParentsIteratorDecoded[*pbdemo.Artist](suite.rc, pbdemo.ArtistType, "id", suite.artistGood.Id)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestQueryDecoded_Ok() {
	suite.iter.EXPECT().Next().Return(suite.artistGood.Resource).Once()
	suite.iter.EXPECT().Next().Return(suite.artistGood2.Resource).Once()
	suite.iter.EXPECT().Next().Return(nil).Times(0)
	suite.rc.EXPECT().Query("query", "blah").
		Return(suite.iter, nil)

	iter, err := cache.QueryDecoded[*pbdemo.Artist](suite.rc, "query", "blah")
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), iter)

	dec, err := iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec.Data)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood2.Data, dec.Data)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestQueryDecoded_DecodeError() {
	suite.iter.EXPECT().Next().Return(suite.artistGood.Resource).Once()
	suite.iter.EXPECT().Next().Return(suite.artistBad).Once()
	suite.iter.EXPECT().Next().Return(nil).Times(0)
	suite.rc.EXPECT().Query("query", "blah").
		Return(suite.iter, nil)

	iter, err := cache.QueryDecoded[*pbdemo.Artist](suite.rc, "query", "blah")
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), iter)

	dec, err := iter.Next()
	require.NoError(suite.T(), err)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Resource, dec.Resource)
	prototest.AssertDeepEqual(suite.T(), suite.artistGood.Data, dec.Data)

	dec, err = iter.Next()
	require.Error(suite.T(), err)
	require.Nil(suite.T(), dec)

	dec, err = iter.Next()
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestQueryDecoded_CacheError() {
	suite.rc.EXPECT().Query("query", "blah").Return(nil, injectedError)

	dec, err := cache.QueryDecoded[*pbdemo.Artist](suite.rc, "query", "blah")
	require.ErrorIs(suite.T(), err, injectedError)
	require.Nil(suite.T(), dec)
}

func (suite *decodedSuite) TestQueryDecoded_Nil() {
	suite.rc.EXPECT().Query("query", "blah").Return(nil, nil)

	dec, err := cache.QueryDecoded[*pbdemo.Artist](suite.rc, "query", "blah")
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), dec)
}

func TestDecodedCache(t *testing.T) {
	suite.Run(t, new(decodedSuite))
}

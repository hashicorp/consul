// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package indexers

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache/indexers/indexersmock"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/suite"
)

var (
	fakeTestError = errors.New("fake test error")
)

func TestSingleIndexer(t *testing.T) {
	suite.Run(t, &decodedSingleIndexerSuite{})
}

type decodedSingleIndexerSuite struct {
	suite.Suite

	indexer *indexersmock.SingleIndexer[*pbdemo.Album]
	args    *indexersmock.FromArgs

	index *singleIndexer[*pbdemo.Album]
}

func (suite *decodedSingleIndexerSuite) SetupTest() {
	suite.indexer = indexersmock.NewSingleIndexer[*pbdemo.Album](suite.T())
	suite.args = indexersmock.NewFromArgs(suite.T())
	suite.index = &singleIndexer[*pbdemo.Album]{
		indexArgs:      suite.args.Execute,
		decodedIndexer: suite.indexer.Execute,
	}
}

func (suite *decodedSingleIndexerSuite) TestFromArgs() {
	suite.args.EXPECT().
		Execute("blah", 1, true).
		Return([]byte("foo"), nil).
		Once()

	val, err := suite.index.FromArgs("blah", 1, true)
	suite.Require().NoError(err)
	suite.Require().Equal([]byte("foo"), val)
}

func (suite *decodedSingleIndexerSuite) TestFromArgs_Error() {
	suite.args.EXPECT().
		Execute("blah", 1, true).
		Return(nil, fakeTestError).
		Once()

	val, err := suite.index.FromArgs("blah", 1, true)
	suite.Require().ErrorIs(err, fakeTestError)
	suite.Require().Nil(val)
}

func (suite *decodedSingleIndexerSuite) TestFromResource() {
	res := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithData(suite.T(), &pbdemo.Album{
			Name: "blah",
		}).
		Build()

	dec := resourcetest.MustDecode[*pbdemo.Album](suite.T(), res)

	suite.indexer.EXPECT().
		Execute(dec).
		Return(true, []byte{1, 2, 3}, nil).
		Once()

	indexed, val, err := suite.index.FromResource(res)
	suite.Require().True(indexed)
	suite.Require().NoError(err)
	suite.Require().Equal([]byte{1, 2, 3}, val)
}

func (suite *decodedSingleIndexerSuite) TestFromResource_Error() {
	res := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithData(suite.T(), &pbdemo.Album{
			Name: "blah",
		}).
		Build()

	dec := resourcetest.MustDecode[*pbdemo.Album](suite.T(), res)

	suite.indexer.EXPECT().
		Execute(dec).
		Return(false, nil, fakeTestError).
		Once()

	indexed, val, err := suite.index.FromResource(res)
	suite.Require().False(indexed)
	suite.Require().ErrorIs(err, fakeTestError)
	suite.Require().Nil(val)
}

func (suite *decodedSingleIndexerSuite) TestFromResource_DecodeError() {
	res := resourcetest.Resource(pbdemo.ArtistType, "foo").
		WithData(suite.T(), &pbdemo.Artist{
			Name: "blah",
		}).
		Build()

	var expectedErr resource.ErrDataParse
	indexed, val, err := suite.index.FromResource(res)
	suite.Require().False(indexed)
	suite.Require().ErrorAs(err, &expectedErr)
	suite.Require().Nil(val)
}

func (suite *decodedSingleIndexerSuite) TestIntegration() {
	// This test attempts to do enough to ensure that the
	// cache Index creator configures all the interfaces/funcs
	// the correct way. It is not meant to fully test the
	// Index type itself.
	res := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithData(suite.T(), &pbdemo.Album{
			Name: "blah",
		}).
		Build()

	dec := resourcetest.MustDecode[*pbdemo.Album](suite.T(), res)

	idx := DecodedSingleIndexer("test", suite.args.Execute, suite.indexer.Execute).IndexedData()

	suite.indexer.EXPECT().
		Execute(dec).
		Return(true, []byte{1, 2}, nil).
		Once()

	txn := idx.Txn()
	suite.Require().NoError(txn.Insert(res))
	txn.Commit()

	suite.args.EXPECT().
		Execute("fake").
		Return([]byte{1, 2}, nil).
		Once()

	r, err := idx.Txn().Get("fake")
	suite.Require().NoError(err)
	prototest.AssertDeepEqual(suite.T(), res, r)
}

func TestMultiIndexer(t *testing.T) {
	suite.Run(t, &decodedMultiIndexerSuite{})
}

type decodedMultiIndexerSuite struct {
	suite.Suite

	indexer *indexersmock.MultiIndexer[*pbdemo.Album]
	args    *indexersmock.FromArgs

	index *multiIndexer[*pbdemo.Album]
}

func (suite *decodedMultiIndexerSuite) SetupTest() {
	suite.indexer = indexersmock.NewMultiIndexer[*pbdemo.Album](suite.T())
	suite.args = indexersmock.NewFromArgs(suite.T())
	suite.index = &multiIndexer[*pbdemo.Album]{
		indexArgs:      suite.args.Execute,
		decodedIndexer: suite.indexer.Execute,
	}
}

func (suite *decodedMultiIndexerSuite) TestFromArgs() {
	suite.args.EXPECT().
		Execute("blah", 1, true).
		Return([]byte("foo"), nil).
		Once()

	val, err := suite.index.FromArgs("blah", 1, true)
	suite.Require().NoError(err)
	suite.Require().Equal([]byte("foo"), val)
}

func (suite *decodedMultiIndexerSuite) TestFromArgs_Error() {
	suite.args.EXPECT().
		Execute("blah", 1, true).
		Return(nil, fakeTestError).
		Once()

	val, err := suite.index.FromArgs("blah", 1, true)
	suite.Require().ErrorIs(err, fakeTestError)
	suite.Require().Nil(val)
}

func (suite *decodedMultiIndexerSuite) TestFromResource() {
	res := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithData(suite.T(), &pbdemo.Album{
			Name: "blah",
		}).
		Build()

	dec := resourcetest.MustDecode[*pbdemo.Album](suite.T(), res)

	suite.indexer.EXPECT().
		Execute(dec).
		Return(true, [][]byte{{1, 2}, {3}}, nil).
		Once()

	indexed, val, err := suite.index.FromResource(res)
	suite.Require().True(indexed)
	suite.Require().NoError(err)
	suite.Require().Equal([][]byte{{1, 2}, {3}}, val)
}

func (suite *decodedMultiIndexerSuite) TestFromResource_Error() {
	res := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithData(suite.T(), &pbdemo.Album{
			Name: "blah",
		}).
		Build()

	dec := resourcetest.MustDecode[*pbdemo.Album](suite.T(), res)

	suite.indexer.EXPECT().
		Execute(dec).
		Return(false, nil, fakeTestError).
		Once()

	indexed, val, err := suite.index.FromResource(res)
	suite.Require().False(indexed)
	suite.Require().ErrorIs(err, fakeTestError)
	suite.Require().Nil(val)
}

func (suite *decodedMultiIndexerSuite) TestFromResource_DecodeError() {
	res := resourcetest.Resource(pbdemo.ArtistType, "foo").
		WithData(suite.T(), &pbdemo.Artist{
			Name: "blah",
		}).
		Build()

	var expectedErr resource.ErrDataParse
	indexed, val, err := suite.index.FromResource(res)
	suite.Require().False(indexed)
	suite.Require().ErrorAs(err, &expectedErr)
	suite.Require().Nil(val)
}

func (suite *decodedMultiIndexerSuite) TestIntegration() {
	// This test attempts to do enough to ensure that the
	// cache Index creator configures all the interfaces/funcs
	// the correct way. It is not meant to fully test the
	// Index type itself.
	res := resourcetest.Resource(pbdemo.AlbumType, "foo").
		WithData(suite.T(), &pbdemo.Album{
			Name: "blah",
		}).
		Build()

	dec := resourcetest.MustDecode[*pbdemo.Album](suite.T(), res)

	idx := DecodedMultiIndexer("test", suite.args.Execute, suite.indexer.Execute).IndexedData()

	suite.indexer.EXPECT().
		Execute(dec).
		Return(true, [][]byte{{1, 2}, {3}}, nil).
		Once()

	txn := idx.Txn()
	suite.Require().NoError(txn.Insert(res))
	txn.Commit()

	suite.args.EXPECT().
		Execute("fake").
		Return([]byte{1, 2}, nil).
		Once()

	suite.args.EXPECT().
		Execute("fake2").
		Return([]byte{3}, nil).
		Once()

	txn = idx.Txn()
	r, err := txn.Get("fake")
	suite.Require().NoError(err)
	prototest.AssertDeepEqual(suite.T(), res, r)

	r, err = txn.Get("fake2")
	suite.Require().NoError(err)
	prototest.AssertDeepEqual(suite.T(), res, r)
}

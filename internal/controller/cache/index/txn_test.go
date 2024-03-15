// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache/index/indexmock"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/suite"
)

var fakeTestError = errors.New("fake test error")

func TestTxn(t *testing.T) {
	suite.Run(t, new(txnSuite))
}

type txnSuite struct {
	suite.Suite

	indexer *indexmock.SingleIndexer
	index   *IndexedData

	r1   *pbresource.Resource
	r2   *pbresource.Resource
	r11  *pbresource.Resource
	r123 *pbresource.Resource
}

func (suite *txnSuite) SetupTest() {
	suite.indexer = indexmock.NewSingleIndexer(suite.T())
	suite.index = New("test", suite.indexer, IndexRequired).IndexedData()

	suite.r1 = testResource("r1")
	suite.r2 = testResource("r2")
	suite.r11 = testResource("r11")
	suite.r123 = testResource("r123")

	exp := suite.indexer.EXPECT()
	exp.FromResource(suite.r1).Return(true, PrefixIndexFromRefOrID(suite.r1.Id), nil).Once()
	exp.FromResource(suite.r2).Return(true, PrefixIndexFromRefOrID(suite.r2.Id), nil).Once()
	exp.FromResource(suite.r11).Return(true, PrefixIndexFromRefOrID(suite.r11.Id), nil).Once()
	exp.FromResource(suite.r123).Return(true, PrefixIndexFromRefOrID(suite.r123.Id), nil).Once()

	txn := suite.index.Txn()
	txn.Insert(suite.r1)
	txn.Insert(suite.r2)
	txn.Insert(suite.r11)
	txn.Insert(suite.r123)
	txn.Commit()
}

func (suite *txnSuite) TestGet() {
	suite.indexer.EXPECT().
		FromArgs(suite.r1.Id).
		RunAndReturn(PrefixReferenceOrIDFromArgs).
		Once()

	actual, err := suite.index.Txn().Get(suite.r1.Id)
	suite.Require().NoError(err)
	suite.Require().NotNil(actual)
	prototest.AssertDeepEqual(suite.T(), suite.r1, actual)
}

func (suite *txnSuite) TestGet_NotFound() {
	suite.indexer.EXPECT().
		FromArgs(suite.r1.Id).
		Return(nil, nil).
		Once()

	actual, err := suite.index.Txn().Get(suite.r1.Id)
	suite.Require().NoError(err)
	suite.Require().Nil(actual)
}

func (suite *txnSuite) TestGet_Error() {
	suite.indexer.EXPECT().
		FromArgs(suite.r1.Id).
		Return(nil, fakeTestError).
		Once()

	actual, err := suite.index.Txn().Get(suite.r1.Id)
	suite.Require().ErrorIs(err, fakeTestError)
	suite.Require().Nil(actual)
}

func (suite *txnSuite) TestListIterator() {
	refQuery := &pbresource.Reference{
		Type: testResourceType,
	}

	suite.indexer.EXPECT().
		FromArgs(refQuery).
		// Calculate a prefix based query for use with the ListIterator
		RunAndReturn(PrefixReferenceOrIDFromArgs).
		Once()

	iter, err := suite.index.Txn().ListIterator(refQuery)
	suite.Require().NoError(err)

	r := iter.Next()
	suite.Require().NotNil(r)
	prototest.AssertDeepEqual(suite.T(), suite.r1, r)

	r = iter.Next()
	suite.Require().NotNil(r)
	prototest.AssertDeepEqual(suite.T(), suite.r11, r)

	r = iter.Next()
	suite.Require().NotNil(r)
	prototest.AssertDeepEqual(suite.T(), suite.r123, r)

	r = iter.Next()
	suite.Require().NotNil(r)
	prototest.AssertDeepEqual(suite.T(), suite.r2, r)

	r = iter.Next()
	suite.Require().Nil(r)
}

func (suite *txnSuite) TestListIterator_Error() {
	suite.indexer.EXPECT().
		// abusing the mock to create a shortened index for us.
		FromArgs("sentinel").
		Return(nil, fakeTestError).
		Once()

	iter, err := suite.index.Txn().ListIterator("sentinel")
	suite.Require().ErrorIs(err, fakeTestError)
	suite.Require().Nil(iter)
}

func (suite *txnSuite) TestParentsIterator() {
	suite.indexer.EXPECT().
		// abusing the mock to create a shortened index for us.
		FromArgs(suite.r123.Id).
		RunAndReturn(ReferenceOrIDFromArgs).
		Once()

	iter, err := suite.index.Txn().ParentsIterator(suite.r123.Id)
	suite.Require().NoError(err)

	r := iter.Next()
	suite.Require().NotNil(r)
	prototest.AssertDeepEqual(suite.T(), suite.r1, r)

	r = iter.Next()
	suite.Require().NotNil(r)
	prototest.AssertDeepEqual(suite.T(), suite.r123, r)

	r = iter.Next()
	suite.Require().Nil(r)
}

func (suite *txnSuite) TestParentsIterator_Error() {
	suite.indexer.EXPECT().
		// abusing the mock to create a shortened index for us.
		FromArgs("sentinel").
		Return(nil, fakeTestError).
		Once()

	iter, err := suite.index.Txn().ParentsIterator("sentinel")
	suite.Require().ErrorIs(err, fakeTestError)
	suite.Require().Nil(iter)
}

func (suite *txnSuite) TestInsert_MissingRequiredIndex() {
	suite.indexer.EXPECT().
		FromResource(suite.r1).
		Return(false, nil, nil).
		Once()

	err := suite.index.Txn().Insert(suite.r1)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, MissingRequiredIndexError{Name: "test"})
}

func (suite *txnSuite) TestInsert_IndexError() {
	suite.indexer.EXPECT().
		FromResource(suite.r1).
		Return(false, nil, fakeTestError).
		Once()

	err := suite.index.Txn().Insert(suite.r1)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, fakeTestError)
}

func (suite *txnSuite) TestInsert_UpdateInternalSlice() {
	// So if you look closely this is going to insert the newR
	// resource but is calculating the index value of the r1
	// resource. This is done to exercise the insertion functionality
	// where indexes are non-unique and at each leaf in the radix
	// tree we must keep a list of items.
	newR := testResource("newR")
	suite.indexer.EXPECT().
		FromResource(newR).
		Return(true, PrefixIndexFromRefOrID(suite.r1.Id), nil).
		Once()

	// here we are setting up the expecation for re-inserting r1
	newR1 := testResource("r1")
	suite.indexer.EXPECT().
		FromResource(newR1).
		Return(true, PrefixIndexFromRefOrID(newR1.Id), nil).
		Once()

	// Actually index the resource
	txn := suite.index.Txn()
	suite.Require().NoError(txn.Insert(newR))
	suite.Require().NoError(txn.Insert(newR1))
	txn.Commit()

	// No validate that the insertions worked correctly.
	suite.indexer.EXPECT().
		FromArgs(newR1.Id).
		RunAndReturn(PrefixReferenceOrIDFromArgs).
		Once()
	iter, err := suite.index.Txn().ListIterator(newR1.Id)
	suite.Require().NoError(err)

	var resources []*pbresource.Resource
	for r := iter.Next(); r != nil; r = iter.Next() {
		resources = append(resources, r)
	}

	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{newR, newR1, suite.r11, suite.r123}, resources)
}

func (suite *txnSuite) TestDelete() {
	// expected to index the resource during deletion
	suite.indexer.EXPECT().
		FromResource(suite.r1).
		Return(true, PrefixIndexFromRefOrID(suite.r1.Id), nil).
		Once()

	// perform the deletion
	txn := suite.index.Txn()
	suite.Require().NoError(txn.Delete(suite.r1))
	txn.Commit()

	// expect to index the ID during the query
	suite.indexer.EXPECT().
		FromArgs(suite.r1.Id).
		RunAndReturn(PrefixReferenceOrIDFromArgs).
		Once()

	// ensure that the deletion worked
	res, err := suite.index.Txn().Get(suite.r1.Id)
	suite.Require().NoError(err)
	suite.Require().Nil(res)
}

func (suite *txnSuite) TestDelete_NotFound() {
	res := testResource("foo")
	suite.indexer.EXPECT().
		FromResource(res).
		Return(true, PrefixIndexFromRefOrID(res.Id), nil).
		Once()

	// attempt the deletion
	txn := suite.index.Txn()
	suite.Require().NoError(txn.Delete(res))
	txn.Commit()
}

func (suite *txnSuite) TestDelete_IdxPresentValNotFound() {
	// The index holds a radix tree that points to a slice of resources.
	// A slice is used to account for non-unique indexes. This test case
	// is meant to specifically exercise the case where the radix leaf
	// node exists but a resource with an equivalent ID is not present
	// in the slice.

	// Calculating the index from the r1 resource will ensure that a
	// radix leaf exists but since newR was never inserted this should
	// exercise the case where the resource is not found within the slice
	newR := testResource("newR")
	suite.indexer.EXPECT().
		FromResource(newR).
		Return(true, PrefixIndexFromRefOrID(suite.r1.Id), nil).
		Once()

	txn := suite.index.Txn()
	suite.Require().NoError(txn.Delete(newR))
	txn.Commit()
}

func (suite *txnSuite) TestDelete_SliceModifications() {
	commonIndex := []byte("fake\x00")

	injectResource := func(name string) *pbresource.Resource {
		r := testResource(name)
		suite.indexer.EXPECT().
			FromResource(r).
			Return(true, commonIndex, nil).
			Once()

		txn := suite.index.Txn()
		suite.Require().NoError(txn.Insert(r))
		txn.Commit()

		return r
	}

	fr1 := injectResource("fr1")
	fr2 := injectResource("fr2")
	fr3 := injectResource("fr3")
	fr4 := injectResource("fr4")
	fr5 := injectResource("fr5")

	txn := suite.index.Txn()

	// excercise deletion of the first slice element
	suite.indexer.EXPECT().
		FromResource(fr1).
		Return(true, commonIndex, nil).
		Once()

	suite.Require().NoError(txn.Delete(fr1))

	// excercise deletion of the last slice element
	suite.indexer.EXPECT().
		FromResource(fr5).
		Return(true, commonIndex, nil).
		Once()

	suite.Require().NoError(txn.Delete(fr5))

	// excercise deletion from the middle of the list
	suite.indexer.EXPECT().
		FromResource(fr3).
		Return(true, commonIndex, nil).
		Once()

	suite.Require().NoError(txn.Delete(fr3))

	txn.Commit()

	// no verify that only fr2 and fr4 exist
	suite.indexer.EXPECT().
		FromArgs(fr2.Id).
		Return(commonIndex, nil).
		Once()

	iter, err := suite.index.Txn().ListIterator(fr2.Id)
	suite.Require().NoError(err)
	suite.Require().NotNil(iter)

	var resources []*pbresource.Resource
	for r := iter.Next(); r != nil; r = iter.Next() {
		resources = append(resources, r)
	}

	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{fr2, fr4}, resources)
}

func (suite *txnSuite) TestDelete_MissingRequiredIndex() {
	suite.indexer.EXPECT().
		FromResource(suite.r1).
		Return(false, nil, nil).
		Once()

	err := suite.index.Txn().Delete(suite.r1)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, MissingRequiredIndexError{Name: "test"})
}

func (suite *txnSuite) TestDelete_IndexError() {
	suite.indexer.EXPECT().
		FromResource(suite.r1).
		Return(false, nil, fakeTestError).
		Once()

	err := suite.index.Txn().Delete(suite.r1)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, fakeTestError)
}

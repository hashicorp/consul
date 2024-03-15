// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache/index"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/suite"
)

var injectedError = errors.New("test-error")

type argsErrorIndexer struct{}

func (i argsErrorIndexer) FromArgs(args ...any) ([]byte, error) {
	return nil, injectedError
}

func (i argsErrorIndexer) FromResource(r *pbresource.Resource) (bool, []byte, error) {
	return true, index.IndexFromRefOrID(r.GetId()), nil
}

type resourceErrorIndexer struct{}

func (i resourceErrorIndexer) FromArgs(args ...any) ([]byte, error) {
	return index.ReferenceOrIDFromArgs(args...)
}

func (i resourceErrorIndexer) FromResource(r *pbresource.Resource) (bool, []byte, error) {
	return false, nil, injectedError
}

func TestKindIndices(t *testing.T) {
	suite.Run(t, &kindSuite{})
}

type kindSuite struct {
	suite.Suite
	k *kindIndices

	album1 *pbresource.Resource
	album2 *pbresource.Resource
	album3 *pbresource.Resource
	album4 *pbresource.Resource
}

func (suite *kindSuite) SetupTest() {
	suite.k = newKindIndices()

	suite.Require().NoError(suite.k.addIndex(namePrefixIndexer()))
	suite.Require().NoError(suite.k.addIndex(releaseYearIndexer()))
	suite.Require().NoError(suite.k.addIndex(tracksIndexer()))

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

	suite.Require().NoError(suite.k.insert(suite.album1))
	suite.Require().NoError(suite.k.insert(suite.album2))
	suite.Require().NoError(suite.k.insert(suite.album3))
	suite.Require().NoError(suite.k.insert(suite.album4))
}

func (suite *kindSuite) TestGet() {
	res, err := suite.k.get("id", suite.album1.Id)
	suite.Require().NoError(err)
	prototest.AssertDeepEqual(suite.T(), suite.album1, res)

	res, err = suite.k.get("year", int32(2022))
	suite.Require().NoError(err)
	prototest.AssertDeepEqual(suite.T(), suite.album3, res)

	res, err = suite.k.get("tracks", "fangorn")
	suite.Require().NoError(err)
	prototest.AssertDeepEqual(suite.T(), suite.album2, res)
}

func (suite *kindSuite) TestGet_IndexNotFound() {
	res, err := suite.k.get("blah", suite.album1.Id)
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, IndexNotFoundError{name: "blah"})
	suite.Require().Nil(res)
}

func (suite *kindSuite) TestList() {
	resources, err := expandIterator(suite.k.listIterator("year", int32(2023)))
	suite.Require().NoError(err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album1, suite.album2}, resources)

	resources, err = expandIterator(suite.k.listIterator("tracks", "f", true))
	suite.Require().NoError(err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album1, suite.album2, suite.album4}, resources)
}

func (suite *kindSuite) TestList_IndexNotFound() {
	res, err := expandIterator(suite.k.listIterator("blah", suite.album1.Id))
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, IndexNotFoundError{name: "blah"})
	suite.Require().Nil(res)
}

func (suite *kindSuite) TestParents() {
	resources, err := expandIterator(suite.k.parentsIterator("name_prefix", "food"))
	suite.Require().NoError(err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album3, suite.album4}, resources)
}

func (suite *kindSuite) TestParents_IndexNotFound() {
	res, err := expandIterator(suite.k.parentsIterator("blah", suite.album1.Id))
	suite.Require().Error(err)
	suite.Require().ErrorIs(err, IndexNotFoundError{name: "blah"})
	suite.Require().Nil(res)
}
func (suite *kindSuite) TestDelete() {
	err := suite.k.delete(suite.album1)
	suite.Require().NoError(err)

	res, err := suite.k.get("id", suite.album1.Id)
	suite.Require().NoError(err)
	suite.Require().Nil(res)

	resources, err := expandIterator(suite.k.listIterator("year", int32(2023)))
	suite.Require().NoError(err)
	prototest.AssertElementsMatch(suite.T(), []*pbresource.Resource{suite.album2}, resources)

	resources, err = expandIterator(suite.k.parentsIterator("name_prefix", "onesie"))
	suite.Require().NoError(err)
	suite.Require().Nil(resources)
}

func (suite *kindSuite) TestInsertIndexError() {
	err := suite.k.insert(
		resourcetest.Resource(pbdemo.ConceptType, "foo").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(suite.T(), &pbdemo.Concept{}).
			Build())

	suite.Require().Error(err)
	suite.Require().ErrorAs(err, &IndexError{})
}

func (suite *kindSuite) TestGetIndexError() {
	val, err := suite.k.get("year", "blah")
	suite.Require().Error(err)
	suite.Require().ErrorAs(err, &IndexError{})
	suite.Require().Nil(val)
}

func (suite *kindSuite) TestListIteratorIndexError() {
	vals, err := suite.k.listIterator("year", "blah")
	suite.Require().Error(err)
	suite.Require().ErrorAs(err, &IndexError{})
	suite.Require().Nil(vals)
}

func (suite *kindSuite) TestParentsIteratorIndexError() {
	vals, err := suite.k.parentsIterator("year", "blah")
	suite.Require().Error(err)
	suite.Require().ErrorAs(err, &IndexError{})
	suite.Require().Nil(vals)
}

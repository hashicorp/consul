// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/controllermock"
	"github.com/hashicorp/consul/internal/controller/dependency/dependencymock"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMapperWithTransform(t *testing.T) {
	res := resourcetest.Resource(fakeMapType, "something").Build()
	rt := controller.Runtime{
		// populating some field to differentiate from zero value
		Logger: hclog.Default(),
	}
	transformed1 := resourcetest.Resource(fakeResourceType, "foo").Build()
	transformed2 := resourcetest.Resource(fakeResourceType, "bar").Build()

	t.Run("transform-err", func(t *testing.T) {
		mockTransform := dependencymock.NewDependencyTransform(t)
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, res).
			Return(nil, injectedErr).
			Once()

		mockMapper := controllermock.NewDependencyMapper(t)

		mt := MapperWithTransform(mockMapper.Execute, mockTransform.Execute)
		reqs, err := mt(context.Background(), rt, res)
		require.Nil(t, reqs)
		require.ErrorIs(t, err, injectedErr)
	})

	t.Run("mapper-err", func(t *testing.T) {
		mockTransform := dependencymock.NewDependencyTransform(t)
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, res).
			Return([]*pbresource.Resource{transformed1, transformed2}, nil).
			Once()

		mockMapper := controllermock.NewDependencyMapper(t)
		mockMapper.EXPECT().
			Execute(mock.Anything, rt, transformed1).
			Return(nil, injectedErr).
			Once()

		mt := MapperWithTransform(mockMapper.Execute, mockTransform.Execute)
		reqs, err := mt(context.Background(), rt, res)
		require.Nil(t, reqs)
		require.ErrorIs(t, err, injectedErr)
	})

	t.Run("ok", func(t *testing.T) {
		mockTransform := dependencymock.NewDependencyTransform(t)
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, res).
			Return([]*pbresource.Resource{transformed1, transformed2}, nil).
			Once()

		mockMapper := controllermock.NewDependencyMapper(t)
		mockMapper.EXPECT().
			Execute(mock.Anything, rt, transformed1).
			Return([]controller.Request{
				{ID: resourceID("testing", "v1", "alt-fake", "foo")},
				{ID: resourceID("testing", "v1", "alt-fake", "bar")},
			}, nil).
			Once()

		mockMapper.EXPECT().
			Execute(mock.Anything, rt, transformed2).
			Return([]controller.Request{
				{ID: resourceID("testing", "v1", "alt-fake", "foo2")},
				{ID: resourceID("testing", "v1", "alt-fake", "bar2")},
			}, nil).
			Once()

		mt := MapperWithTransform(mockMapper.Execute, mockTransform.Execute)
		reqs, err := mt(context.Background(), rt, res)
		require.NoError(t, err)
		require.Len(t, reqs, 4)
		expected := []controller.Request{
			{ID: resourceID("testing", "v1", "alt-fake", "foo")},
			{ID: resourceID("testing", "v1", "alt-fake", "bar")},
			{ID: resourceID("testing", "v1", "alt-fake", "foo2")},
			{ID: resourceID("testing", "v1", "alt-fake", "bar2")},
		}
		prototest.AssertElementsMatch(t, expected, reqs)
	})
}

func TestTransformChain(t *testing.T) {
	res := resourcetest.Resource(fakeMapType, "something").Build()
	rt := controller.Runtime{
		// populating some field to differentiate from zero value
		Logger: hclog.Default(),
	}
	transformed1 := resourcetest.Resource(fakeResourceType, "foo").Build()
	transformed2 := resourcetest.Resource(fakeResourceType, "bar").Build()

	out1 := resourcetest.Resource(altFakeResourceType, "foo").Build()
	out2 := resourcetest.Resource(altFakeResourceType, "bar").Build()
	out3 := resourcetest.Resource(altFakeResourceType, "baz").Build()

	t.Run("err", func(t *testing.T) {
		mockTransform := dependencymock.NewDependencyTransform(t)
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, res).
			Return([]*pbresource.Resource{transformed1}, nil).
			Once()
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, transformed1).
			Return(nil, injectedErr).
			Once()

		resources, err := TransformChain(mockTransform.Execute, mockTransform.Execute)(
			context.Background(),
			rt,
			res,
		)

		require.Nil(t, resources)
		require.ErrorIs(t, err, injectedErr)
	})

	t.Run("ok", func(t *testing.T) {
		mockTransform := dependencymock.NewDependencyTransform(t)
		// Transform Chain
		//
		// 1. Transform original res to the two outputs
		// 2. Transform the first output from 1
		// 3. Transform the second output from 1
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, res).
			Return([]*pbresource.Resource{transformed1, transformed2}, nil).
			Once()
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, transformed1).
			Return([]*pbresource.Resource{out1, out2}, nil).
			Once()
		mockTransform.EXPECT().
			Execute(mock.Anything, rt, transformed2).
			Return([]*pbresource.Resource{out3}, nil).
			Once()

		resources, err := TransformChain(mockTransform.Execute, mockTransform.Execute)(
			context.Background(),
			rt,
			res,
		)

		require.NoError(t, err)
		expected := []*pbresource.Resource{out1, out2, out3}
		prototest.AssertElementsMatch(t, expected, resources)
	})
}

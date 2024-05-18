// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/controllermock"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

var (
	fakeMapType = &pbresource.Type{
		Group:        "testing",
		GroupVersion: "v1",
		Kind:         "pre-map",
	}

	fakeResourceType = &pbresource.Type{
		Group:        "testing",
		GroupVersion: "v1",
		Kind:         "fake",
	}

	altFakeResourceType = &pbresource.Type{
		Group:        "testing",
		GroupVersion: "v1",
		Kind:         "alt-fake",
	}

	injectedErr = errors.New("injected")
)

func TestWrapAndReplaceType(t *testing.T) {
	res := resourcetest.Resource(fakeMapType, "something").Build()
	// populating the runtime with something so we can tell that
	// the runtime is passed through
	rt := controller.Runtime{
		Logger: hclog.Default(),
	}

	t.Run("ok", func(t *testing.T) {
		mm := controllermock.NewDependencyMapper(t)
		mm.EXPECT().
			Execute(mock.Anything, rt, res).
			Return([]controller.Request{
				{ID: resourceID("testing", "v1", "fake", "foo")},
				{ID: resourceID("testing", "v1", "fake", "bar")},
			}, nil).
			Once()

		mapper := WrapAndReplaceType(altFakeResourceType, mm.Execute)
		reqs, err := mapper(context.Background(), rt, res)
		require.NoError(t, err)
		require.Len(t, reqs, 2)
		expected := []controller.Request{
			{ID: resourceID("testing", "v1", "alt-fake", "foo")},
			{ID: resourceID("testing", "v1", "alt-fake", "bar")},
		}
		prototest.AssertElementsMatch(t, expected, reqs)
	})

	t.Run("err", func(t *testing.T) {
		mm := controllermock.NewDependencyMapper(t)
		mm.EXPECT().
			Execute(mock.Anything, rt, res).
			Return(nil, injectedErr).
			Once()
		mapper := WrapAndReplaceType(altFakeResourceType, mm.Execute)
		reqs, err := mapper(context.Background(), rt, res)
		require.Nil(t, reqs)
		require.ErrorIs(t, err, injectedErr)
	})
}

func TestMultiMapper(t *testing.T) {
	res := resourcetest.Resource(fakeMapType, "something").Build()
	// populating the runtime with something so we can tell that
	// the runtime is passed through
	rt := controller.Runtime{
		Logger: hclog.Default(),
	}

	t.Run("ok", func(t *testing.T) {
		mockMapper := controllermock.NewDependencyMapper(t)
		mockMapper.EXPECT().
			Execute(mock.Anything, rt, res).
			Return([]controller.Request{
				{ID: resourceID("testing", "v1", "fake", "foo")},
				{ID: resourceID("testing", "v1", "fake", "bar")},
			}, nil).
			Times(2)

		mm := MultiMapper(
			mockMapper.Execute,
			WrapAndReplaceType(altFakeResourceType, mockMapper.Execute),
		)

		reqs, err := mm(context.Background(), rt, res)
		require.NoError(t, err)
		require.Len(t, reqs, 4)
		expected := []controller.Request{
			{ID: resourceID("testing", "v1", "alt-fake", "foo")},
			{ID: resourceID("testing", "v1", "alt-fake", "bar")},
			{ID: resourceID("testing", "v1", "fake", "foo")},
			{ID: resourceID("testing", "v1", "fake", "bar")},
		}
		prototest.AssertElementsMatch(t, expected, reqs)
	})

	t.Run("err", func(t *testing.T) {
		mockMapper := controllermock.NewDependencyMapper(t)
		mockMapper.EXPECT().
			Execute(mock.Anything, rt, res).
			Return([]controller.Request{
				{ID: resourceID("testing", "v1", "fake", "foo")},
				{ID: resourceID("testing", "v1", "fake", "bar")},
			}, nil).
			Once()

		mockMapper.EXPECT().
			Execute(mock.Anything, rt, res).
			Return(nil, injectedErr).
			Once()

		mm := MultiMapper(
			mockMapper.Execute,
			WrapAndReplaceType(altFakeResourceType, mockMapper.Execute),
		)
		reqs, err := mm(context.Background(), rt, res)
		require.Nil(t, reqs)
		require.ErrorIs(t, err, injectedErr)
	})
}

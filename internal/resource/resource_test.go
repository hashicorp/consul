// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v2"
)

func TestFinalizer(t *testing.T) {
	t.Run("no finalizers", func(t *testing.T) {
		res := rtest.Resource(pbdemo.ArtistType, "art1").Build()
		require.False(t, resource.HasFinalizers(res))
		require.False(t, resource.HasFinalizer(res, "finalizer1"))
		require.Equal(t, mapset.NewSet[string](), resource.GetFinalizers(res))
		resource.RemoveFinalizer(res, "finalizer")
	})

	t.Run("add finalizer", func(t *testing.T) {
		res := rtest.Resource(pbdemo.ArtistType, "art1").Build()
		resource.AddFinalizer(res, "finalizer1")
		require.True(t, resource.HasFinalizers(res))
		require.True(t, resource.HasFinalizer(res, "finalizer1"))
		require.False(t, resource.HasFinalizer(res, "finalizer2"))
		require.Equal(t, mapset.NewSet[string]("finalizer1"), resource.GetFinalizers(res))

		// add duplicate -> noop
		resource.AddFinalizer(res, "finalizer1")
		require.Equal(t, mapset.NewSet[string]("finalizer1"), resource.GetFinalizers(res))
	})

	t.Run("remove finalizer", func(t *testing.T) {
		res := rtest.Resource(pbdemo.ArtistType, "art1").Build()
		resource.AddFinalizer(res, "finalizer1")
		resource.AddFinalizer(res, "finalizer2")
		resource.RemoveFinalizer(res, "finalizer1")
		require.False(t, resource.HasFinalizer(res, "finalizer1"))
		require.True(t, resource.HasFinalizer(res, "finalizer2"))
		require.Equal(t, mapset.NewSet[string]("finalizer2"), resource.GetFinalizers(res))

		// remove non-existent -> noop
		resource.RemoveFinalizer(res, "finalizer3")
		require.Equal(t, mapset.NewSet[string]("finalizer2"), resource.GetFinalizers(res))
	})

}

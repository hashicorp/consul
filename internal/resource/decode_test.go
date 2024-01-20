// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestGetDecodedResource(t *testing.T) {
	var (
		baseClient = svctest.NewResourceServiceBuilder().WithRegisterFns(demo.RegisterTypes).Run(t)
		client     = rtest.NewClient(baseClient)
		ctx        = testutil.TestContext(t)
	)

	babypantsID := &pbresource.ID{
		Type:    demo.TypeV2Artist,
		Tenancy: resource.DefaultNamespacedTenancy(),
		Name:    "babypants",
	}

	testutil.RunStep(t, "not found", func(t *testing.T) {
		got, err := resource.GetDecodedResource[*pbdemo.Artist](ctx, client, babypantsID)
		require.NoError(t, err)
		require.Nil(t, got)
	})

	testutil.RunStep(t, "found", func(t *testing.T) {
		data := &pbdemo.Artist{
			Name: "caspar babypants",
		}
		res := rtest.Resource(demo.TypeV2Artist, "babypants").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, data).
			Write(t, client)

		got, err := resource.GetDecodedResource[*pbdemo.Artist](ctx, client, babypantsID)
		require.NoError(t, err)
		require.NotNil(t, got)

		// Clone generated fields over.
		res.Id.Uid = got.Resource.Id.Uid
		res.Version = got.Resource.Version
		res.Generation = got.Resource.Generation

		// Clone defaulted fields over
		data.Genre = pbdemo.Genre_GENRE_DISCO

		prototest.AssertDeepEqual(t, res, got.Resource)
		prototest.AssertDeepEqual(t, data, got.Data)
	})
}

func TestDecode(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		fooData := &pbdemo.Artist{
			Name: "caspar babypants",
		}
		any, err := anypb.New(fooData)
		require.NoError(t, err)

		foo := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    "babypants",
			},
			Data: any,
			Metadata: map[string]string{
				"generated_at": time.Now().Format(time.RFC3339),
			},
		}

		dec, err := resource.Decode[*pbdemo.Artist](foo)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, foo, dec.Resource)
		prototest.AssertDeepEqual(t, fooData, dec.Data)
	})

	t.Run("bad", func(t *testing.T) {
		foo := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    "babypants",
			},
			Data: &anypb.Any{
				TypeUrl: "garbage",
				Value:   []byte("more garbage"),
			},
			Metadata: map[string]string{
				"generated_at": time.Now().Format(time.RFC3339),
			},
		}

		_, err := resource.Decode[*pbdemo.Artist](foo)
		require.Error(t, err)
	})
}

func TestDecodeList(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		artist1, err := demo.GenerateV2Artist()
		require.NoError(t, err)
		artist2, err := demo.GenerateV2Artist()
		require.NoError(t, err)
		dec1, err := resource.Decode[*pbdemo.Artist](artist1)
		require.NoError(t, err)
		dec2, err := resource.Decode[*pbdemo.Artist](artist2)
		require.NoError(t, err)

		resources := []*pbresource.Resource{artist1, artist2}

		decList, err := resource.DecodeList[*pbdemo.Artist](resources)
		require.NoError(t, err)
		require.Len(t, decList, 2)

		prototest.AssertDeepEqual(t, dec1.Resource, decList[0].Resource)
		prototest.AssertDeepEqual(t, dec1.Data, decList[0].Data)
		prototest.AssertDeepEqual(t, dec2.Resource, decList[1].Resource)
		prototest.AssertDeepEqual(t, dec2.Data, decList[1].Data)
	})

	t.Run("bad", func(t *testing.T) {
		artist1, err := demo.GenerateV2Artist()
		require.NoError(t, err)

		foo := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: resource.DefaultNamespacedTenancy(),
				Name:    "babypants",
			},
			Data: &anypb.Any{
				TypeUrl: "garbage",
				Value:   []byte("more garbage"),
			},
			Metadata: map[string]string{
				"generated_at": time.Now().Format(time.RFC3339),
			},
		}

		_, err = resource.DecodeList[*pbdemo.Artist]([]*pbresource.Resource{artist1, foo})
		require.Error(t, err)
	})
}

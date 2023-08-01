// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v2"
	"github.com/hashicorp/consul/proto/private/prototest"
)

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
				Tenancy: demo.TenancyDefault,
				Name:    "babypants",
			},
			Data: any,
			Metadata: map[string]string{
				"generated_at": time.Now().Format(time.RFC3339),
			},
		}

		dec, err := resource.Decode[pbdemo.Artist, *pbdemo.Artist](foo)
		require.NoError(t, err)

		prototest.AssertDeepEqual(t, foo, dec.Resource)
		prototest.AssertDeepEqual(t, fooData, dec.Data)
	})

	t.Run("bad", func(t *testing.T) {
		foo := &pbresource.Resource{
			Id: &pbresource.ID{
				Type:    demo.TypeV2Artist,
				Tenancy: demo.TenancyDefault,
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

		_, err := resource.Decode[pbdemo.Artist, *pbdemo.Artist](foo)
		require.Error(t, err)
	})
}

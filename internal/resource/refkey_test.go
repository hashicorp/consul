// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestReferenceKey(t *testing.T) {
	// TODO(peering/v2) update the test to account for peer tenancy
	tenancy1 := &pbresource.Tenancy{}
	tenancy1_actual := defaultTenancy()
	tenancy2 := &pbresource.Tenancy{
		Partition: "ap1",
		Namespace: "ns-billing",
	}
	tenancy3 := &pbresource.Tenancy{
		Partition: "ap2",
		Namespace: "ns-intern",
	}

	res1, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	res1.Id.Tenancy = tenancy1

	res2, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	res2.Id.Tenancy = tenancy2

	res3, err := demo.GenerateV2Artist()
	require.NoError(t, err)
	res3.Id.Tenancy = tenancy3

	id1 := res1.Id
	id2 := res2.Id
	id3 := res3.Id

	ref1 := resource.Reference(id1, "")
	ref2 := resource.Reference(id2, "")
	ref3 := resource.Reference(id3, "")

	idRK1 := resource.NewReferenceKey(id1)
	idRK2 := resource.NewReferenceKey(id2)
	idRK3 := resource.NewReferenceKey(id3)

	refRK1 := resource.NewReferenceKey(ref1)
	refRK2 := resource.NewReferenceKey(ref2)
	refRK3 := resource.NewReferenceKey(ref3)

	require.Equal(t, idRK1, refRK1)
	require.Equal(t, idRK2, refRK2)
	require.Equal(t, idRK3, refRK3)

	prototest.AssertDeepEqual(t, tenancy1_actual, idRK1.GetTenancy())
	prototest.AssertDeepEqual(t, tenancy2, idRK2.GetTenancy())
	prototest.AssertDeepEqual(t, tenancy3, idRK3.GetTenancy())

	// Now that we tested the defaulting, swap out the tenancy in the id so
	// that the comparisons work.
	id1.Tenancy = tenancy1_actual
	ref1.Tenancy = tenancy1_actual

	prototest.AssertDeepEqual(t, id1, idRK1.ToID())
	prototest.AssertDeepEqual(t, id2, idRK2.ToID())
	prototest.AssertDeepEqual(t, id3, idRK3.ToID())

	prototest.AssertDeepEqual(t, ref1, refRK1.ToReference())
	prototest.AssertDeepEqual(t, ref2, refRK2.ToReference())
	prototest.AssertDeepEqual(t, ref3, refRK3.ToReference())
}

func defaultTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import (
	"testing"

	"github.com/hashicorp/consul/internal/controller/cache/index/indexmock"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/stretchr/testify/require"
)

var (
	testResourceType = &pbresource.Type{
		Group:        "test",
		GroupVersion: "v1",
		Kind:         "fake",
	}
)

func testResource(name string) *pbresource.Resource {
	return resourcetest.Resource(testResourceType, name).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Build()
}

func TestResourceIteratorNext(t *testing.T) {
	m := indexmock.NewResourceIterable(t)

	r1 := testResource("one")

	r2 := testResource("two")
	r3 := testResource("three")

	m.EXPECT().Next().Once().Return(nil, []*pbresource.Resource{r1}, true)
	m.EXPECT().Next().Once().Return(nil, []*pbresource.Resource{r2, r3}, false)
	m.EXPECT().Next().Return(nil, nil, false)

	i := resourceIterator{
		iter: m,
	}

	// iterator is processing the first list of items
	actual := i.Next()
	require.NotNil(t, actual)
	prototest.AssertDeepEqual(t, r1, actual)
	// iterator should now be processing the second list of items.
	actual = i.Next()
	require.NotNil(t, actual)
	prototest.AssertDeepEqual(t, r2, actual)
	// second element of second list returned
	actual = i.Next()
	require.NotNil(t, actual)
	prototest.AssertDeepEqual(t, r3, actual)
	// no more items so a call to Next should return nil
	actual = i.Next()
	require.Nil(t, actual)
	// verify that it continues to return nil indefinitely without causing a panic.
	actual = i.Next()
	require.Nil(t, actual)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dependency

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func resourceID(group string, version string, kind string, name string) *pbresource.ID {
	return &pbresource.ID{
		Type: &pbresource.Type{
			Group:        group,
			GroupVersion: version,
			Kind:         kind,
		},
		Tenancy: &pbresource.Tenancy{
			Partition: "default",
			Namespace: "default",
		},
		Name: name,
	}
}

func TestMapOwner(t *testing.T) {
	owner := resourceID("foo", "v99", "bar", "object")

	res := &pbresource.Resource{
		Id:    resourceID("something", "v1", "else", "x"),
		Owner: owner,
	}

	reqs, err := MapOwner(context.Background(), controller.Runtime{}, res)
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	prototest.AssertDeepEqual(t, owner, reqs[0].ID)
}

func TestMapOwnerFiltered(t *testing.T) {
	mapper := MapOwnerFiltered(&pbresource.Type{
		Group:        "foo",
		GroupVersion: "v1",
		Kind:         "bar",
	})

	type testCase struct {
		owner   *pbresource.ID
		matches bool
	}

	cases := map[string]testCase{
		"nil-owner": {
			owner:   nil,
			matches: false,
		},
		"group-mismatch": {
			owner:   resourceID("other", "v1", "bar", "irrelevant"),
			matches: false,
		},
		"group-version-mismatch": {
			owner:   resourceID("foo", "v2", "bar", "irrelevant"),
			matches: false,
		},
		"kind-mismatch": {
			owner:   resourceID("foo", "v1", "baz", "irrelevant"),
			matches: false,
		},
		"match": {
			owner:   resourceID("foo", "v1", "bar", "irrelevant"),
			matches: true,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			// the runtime is not used by the mapper so its fine to pass an empty struct
			req, err := mapper(context.Background(), controller.Runtime{}, &pbresource.Resource{
				Id:    resourceID("foo", "v1", "other", "x"),
				Owner: tcase.owner,
			})

			// The mapper has no error paths at present
			require.NoError(t, err)

			if tcase.matches {
				require.NotNil(t, req)
				require.Len(t, req, 1)
				prototest.AssertDeepEqual(t, req[0].ID, tcase.owner, cmpopts.EquateEmpty())
			} else {
				require.Nil(t, req)
			}
		})
	}
}

func TestReplaceType(t *testing.T) {
	rtype := &pbresource.Type{
		Group:        "foo",
		GroupVersion: "v1",
		Kind:         "bar",
	}

	tenant := &pbresource.Tenancy{
		Partition: "not",
		Namespace: "using",
	}

	in := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: &pbresource.Type{
				Group:        "other",
				GroupVersion: "v2",
				Kind:         "baz",
			},
			Tenancy: tenant,
			Name:    "arr-matey",
		},
	}

	mapper := ReplaceType(rtype)

	reqs, err := mapper(nil, controller.Runtime{}, in)
	require.NoError(t, err)
	require.Len(t, reqs, 1)

	expected := &pbresource.ID{
		Type:    rtype,
		Tenancy: tenant,
		Name:    "arr-matey",
	}
	prototest.AssertDeepEqual(t, expected, reqs[0].ID)
}

func TestMapDecoded(t *testing.T) {
	mapper := MapDecoded[*pbdemo.Artist](func(_ context.Context, _ controller.Runtime, res *resource.DecodedResource[*pbdemo.Artist]) ([]controller.Request, error) {
		return []controller.Request{
			{
				ID: &pbresource.ID{
					Type:    res.Id.Type,
					Tenancy: res.Id.Tenancy,
					// not realistic for how the Artist's Name is intended but we just want to pull
					// some data out of the decoded portion and return it.
					Name: res.Data.Name,
				},
			},
		}, nil
	})

	for _, tenancy := range resourcetest.TestTenancies() {
		t.Run(resourcetest.AppendTenancyInfo(t.Name(), tenancy), func(t *testing.T) {
			ctx := testutil.TestContext(t)

			res1 := resourcetest.Resource(pbdemo.ArtistType, "foo").
				WithTenancy(tenancy).
				WithData(t, &pbdemo.Artist{Name: "something"}).
				Build()

			res2 := resourcetest.Resource(pbdemo.ArtistType, "foo").
				WithTenancy(tenancy).
				// Wrong data type here to force an error in the outer decoder
				WithData(t, &pbdemo.Album{Name: "else"}).
				Build()

			reqs, err := mapper(ctx, controller.Runtime{}, res1)
			require.NoError(t, err)
			require.Len(t, reqs, 1)

			expected := &pbresource.ID{
				Type:    res1.Id.Type,
				Tenancy: res1.Id.Tenancy,
				Name:    "something",
			}
			prototest.AssertDeepEqual(t, expected, reqs[0].ID)

			reqs, err = mapper(ctx, controller.Runtime{}, res2)
			require.Nil(t, reqs)
			require.Error(t, err)
		})
	}
}

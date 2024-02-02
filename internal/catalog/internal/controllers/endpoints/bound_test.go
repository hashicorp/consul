// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/endpoints"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/demo"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemo "github.com/hashicorp/consul/proto/private/pbdemo/v2"
)

func TestGetBoundIdentities(t *testing.T) {
	tenancy := resource.DefaultNamespacedTenancy()

	build := func(conds ...*pbresource.Condition) *pbresource.Resource {
		b := rtest.Resource(demo.TypeV2Artist, "artist").
			WithTenancy(tenancy).
			WithData(t, &pbdemo.Artist{Name: "very arty"})
		if len(conds) > 0 {
			b.WithStatus(endpoints.ControllerID, &pbresource.Status{
				Conditions: conds,
			})
		}
		return b.Build()
	}

	run := endpoints.GetBoundIdentities

	require.Empty(t, run(build(nil)))
	require.Empty(t, run(build(&pbresource.Condition{
		Type:    endpoints.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "",
	})))
	require.Equal(t, []string{"foo"}, run(build(&pbresource.Condition{
		Type:    endpoints.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "foo",
	})))
	require.Empty(t, run(build(&pbresource.Condition{
		Type:    endpoints.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_FALSE,
		Message: "foo",
	})))
	require.Equal(t, []string{"bar", "foo"}, run(build(&pbresource.Condition{
		Type:    endpoints.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "bar,foo", // proper order
	})))
	require.Equal(t, []string{"bar", "foo"}, run(build(&pbresource.Condition{
		Type:    endpoints.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "foo,bar", // incorrect order gets fixed
	})))

}

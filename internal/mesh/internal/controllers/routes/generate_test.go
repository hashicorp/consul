// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestGenerateComputedRoutes(t *testing.T) {
	type testcase struct {
		related       *loader.RelatedResources
		expect        []*ComputedRoutesResult
		expectPending int
	}

	run := func(t *testing.T, tc testcase) {
		pending := make(PendingStatuses)

		got := GenerateComputedRoutes(tc.related, pending)
		require.Len(t, pending, tc.expectPending)

		prototest.AssertElementsMatch[*ComputedRoutesResult](
			t, tc.expect, got,
		)
	}

	var (
		apiServiceID        = rtest.Resource(catalog.ServiceType, "api").ID()
		apiComputedRoutesID = rtest.Resource(types.ComputedRoutesType, "api").ID()
	)

	cases := map[string]testcase{
		"none": {
			related: loader.NewRelatedResources(),
		},
		"no aligned service": {
			related: loader.NewRelatedResources().
				AddComputedRoutesIDs(apiComputedRoutesID).
			AddResources(
	apiServiceData := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"api-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			// {TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
	}
			),
			expect: []*ComputedRoutesResult{
				{
					ID:      apiComputedRoutesID,
					OwnerID: apiServiceID,
					Data:    nil,
				},
			},
		},
		"aligned service not in mesh": {
			related: loader.NewRelatedResources(),
		},
	}
	// loader.NewRelatedResources().AddResources(nil).AddComputedRoutesIDs()

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

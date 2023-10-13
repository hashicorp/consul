// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	catalogtesthelpers "github.com/hashicorp/consul/internal/catalog/catalogtest/helpers"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestDestinationsConfigurationACLs(t *testing.T) {
	catalogtesthelpers.RunWorkloadSelectingTypeACLsTests[*pbmesh.DestinationsConfiguration](t, pbmesh.DestinationsConfigurationType,
		func(selector *pbcatalog.WorkloadSelector) *pbmesh.DestinationsConfiguration {
			return &pbmesh.DestinationsConfiguration{Workloads: selector}
		},
		RegisterDestinationsConfiguration,
	)
}

func TestValidateDestinationsConfiguration(t *testing.T) {
	type testcase struct {
		data      *pbmesh.DestinationsConfiguration
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.DestinationsConfigurationType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.data).
			Build()

		err := ValidateDestinationsConfiguration(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.DestinationsConfiguration](t, res)
		prototest.AssertDeepEqual(t, tc.data, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		// emptiness
		"empty": {
			data:      &pbmesh.DestinationsConfiguration{},
			expectErr: `invalid "workloads" field: cannot be empty`,
		},
		"empty selector": {
			data: &pbmesh.DestinationsConfiguration{
				Workloads: &pbcatalog.WorkloadSelector{},
			},
			expectErr: `invalid "workloads" field: cannot be empty`,
		},
		"bad selector": {
			data: &pbmesh.DestinationsConfiguration{
				Workloads: &pbcatalog.WorkloadSelector{
					Names:  []string{"blah"},
					Filter: "garbage.foo == bar",
				},
			},
			expectErr: `invalid "filter" field: filter "garbage.foo == bar" is invalid: Selector "garbage" is not valid`,
		},
		"good selector": {
			data: &pbmesh.DestinationsConfiguration{
				Workloads: &pbcatalog.WorkloadSelector{
					Names:  []string{"blah"},
					Filter: "metadata.foo == bar",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

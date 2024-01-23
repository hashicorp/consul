// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package authv2beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasReferencedSamenessGroups(t *testing.T) {
	type testCase struct {
		tp       *TrafficPermissions
		expected bool
	}
	testCases := []*testCase{
		{
			tp: &TrafficPermissions{
				Permissions: []*Permission{
					{
						Sources: []*Source{
							{
								SamenessGroup: "sg1",
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			tp: &TrafficPermissions{
				Permissions: []*Permission{
					{
						Sources: []*Source{
							{
								Peer: "peer",
							},
						},
					},
				},
			},
			expected: false,
		},
	}
	for _, tc := range testCases {
		require.Equal(t, tc.tp.HasReferencedSamenessGroups(), tc.expected)
	}
}

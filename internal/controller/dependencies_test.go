// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"testing"

	"github.com/hashicorp/consul/internal/testing/golden"
	"github.com/stretchr/testify/require"
)

func TestDependenciesGolden(t *testing.T) {
	deps := Dependencies{
		"t1": []string{"t2", "t3"},
		"t2": []string{"t4"},
		"t4": []string{"t1"},
	}
	mermaid := deps.ToMermaid()
	expected := golden.Get(t, mermaid, "dependencies.golden")
	require.Equal(t, expected, mermaid)
}

func TestValidateDependencies(t *testing.T) {
	type testCase struct {
		dependencies Dependencies
		expectErr    string
	}

	run := func(t *testing.T, tc testCase) {
		err := tc.dependencies.validate()
		if len(tc.expectErr) > 0 {
			require.Contains(t, err.Error(), tc.expectErr)
		} else {
			require.NoError(t, err)
		}

	}

	cases := map[string]testCase{
		"empty": {
			dependencies: nil,
		},
		"no circular dependencies": {
			dependencies: Dependencies{
				"t1": []string{"t2", "t3"},
				"t2": []string{"t3"},
				"t3": []string{"t4"},
				"t4": nil,
			},
		},
		"with circular dependency": {
			dependencies: Dependencies{
				"t1": []string{"t2", "t3"},
				"t2": []string{"t1"},
			},
			expectErr: `circular dependency between "t1" and "t2"`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

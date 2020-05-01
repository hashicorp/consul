package authmethod

import (
	"sort"

	"github.com/hashicorp/go-bexpr"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

// RequireIdentityMatch tests to see if the given Identity matches the provided
// projected vars and filters for testing purpose.
func RequireIdentityMatch(t testing.T, id *Identity, projectedVars map[string]string, filters ...string) {
	t.Helper()

	require.Equal(t, projectedVars, id.ProjectedVars)

	names := make([]string, 0, len(projectedVars))
	for k, _ := range projectedVars {
		names = append(names, k)
	}
	sort.Strings(names)
	require.Equal(t, names, id.ProjectedVarNames())
	require.Nil(t, id.EnterpriseMeta)

	for _, filter := range filters {
		eval, err := bexpr.CreateEvaluatorForType(filter, nil, id.SelectableFields)
		if err != nil {
			t.Fatalf("filter %q got err: %v", filter, err)
		}

		result, err := eval.Evaluate(id.SelectableFields)
		if err != nil {
			t.Fatalf("filter %q got err: %v", filter, err)
		}

		if !result {
			t.Fatalf("filter %q did not match", filter)
		}
	}
}

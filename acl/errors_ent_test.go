//go:build consulent
// +build consulent

package acl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPermissionDeniedErrorEnt(t *testing.T) {
	type testCase struct {
		err      PermissionDeniedError
		expected string
	}

	testName := func(t testCase) string {
		return t.expected
	}

	var ctx = &AuthorizerContext{Namespace: "bar", Partition: "foo"}
	auth1 := mockAuthorizer{}

	cases := []testCase{
		{
			err:      PermissionDeniedByACL(&auth1, ctx, ResourceService, AccessRead, "foobar"),
			expected: "Permission denied: provided token lacks permission 'service:read' on \"foobar\" in partition \"foo\" in namespace \"bar\"",
		},
		{
			err:      PermissionDeniedByACLUnnamed(&auth1, ctx, ResourceService, AccessRead),
			expected: "Permission denied: provided token lacks permission 'service:read'",
		},
	}

	for _, tcase := range cases {
		t.Run(testName(tcase), func(t *testing.T) {
			require.Error(t, tcase.err)
			require.Equal(t, tcase.expected, tcase.err.Error())
		})
	}
}

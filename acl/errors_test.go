package acl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPermissionDeniedError(t *testing.T) {
	type testCase struct {
		err      PermissionDeniedError
		expected string
	}

	testName := func(t testCase) string {
		return t.expected
	}

	auth1 := mockAuthorizer{}

	cases := []testCase{
		{
			err:      PermissionDeniedError{},
			expected: "Permission denied",
		},
		{
			err:      PermissionDeniedError{Cause: "simon says"},
			expected: "Permission denied: simon says",
		},
		{
			err:      PermissionDeniedByACL(&auth1, nil, ResourceService, AccessRead, "foobar"),
			expected: "Permission denied: provided token lacks permission 'service:read' on \"foobar\"",
		},
		{
			err:      PermissionDeniedByACLUnnamed(&auth1, nil, ResourceService, AccessRead),
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

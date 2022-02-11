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
			err:      PermissionDeniedByACL(&auth1, nil, "Service", "Read", "foobar"),
			expected: "Permission denied: provided accessor lacks permission 'Service:Read' foobar",
		},
		{
			err:      PermissionDeniedByACLUnnamed(&auth1, nil, "Service", "Read"),
			expected: "Permission denied: provided accessor lacks permission 'Service:Read'",
		},
	}

	for _, tcase := range cases {
		t.Run(testName(tcase), func(t *testing.T) {
			require.Error(t, tcase.err)
			require.Equal(t, tcase.expected, tcase.err.Error())
		})
	}
}

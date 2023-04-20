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

	auth1 := MockAuthorizer{}
	auth2 := AllowAuthorizer{nil, AnonymousTokenID}

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
			expected: "Permission denied: token with AccessorID '' lacks permission 'service:read' on \"foobar\"",
		},
		{
			err:      PermissionDeniedByACLUnnamed(&auth1, nil, ResourceService, AccessRead),
			expected: "Permission denied: token with AccessorID '' lacks permission 'service:read'",
		},
		{
			err:      PermissionDeniedByACLUnnamed(auth2, nil, ResourceService, AccessRead),
			expected: "Permission denied: anonymous token lacks permission 'service:read'. The anonymous token is used implicitly when a request does not specify a token.",
		},
	}

	for _, tcase := range cases {
		t.Run(testName(tcase), func(t *testing.T) {
			require.Error(t, tcase.err)
			require.Equal(t, tcase.expected, tcase.err.Error())
		})
	}
}

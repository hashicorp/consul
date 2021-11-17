package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSVersion_PartialEq(t *testing.T) {
	require.Greater(t, TLSv1_3, TLSv1_2)
	require.Greater(t, TLSv1_2, TLSv1_1)
	require.Greater(t, TLSv1_1, TLSv1_0)

	require.Less(t, TLSv1_2, TLSv1_3)
	require.Less(t, TLSv1_1, TLSv1_2)
	require.Less(t, TLSv1_0, TLSv1_1)
}

func TestTLSVersion_Invalid(t *testing.T) {
	require.NotEqual(t, TLSVersionAuto, TLSVersionInvalid)

	// Error values should be negative
	require.Less(t, TLSVersionInvalid, 0)
}

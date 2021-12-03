package types

import (
	"encoding/json"
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
	var zeroValue TLSVersion
	require.NotEqual(t, TLSVersionInvalid, zeroValue)
	require.NotEqual(t, TLSVersionInvalid, TLSVersionUnspecified)
	require.NotEqual(t, TLSVersionInvalid, TLSVersionAuto)
}

func TestTLSVersion_Zero(t *testing.T) {
	var zeroValue TLSVersion
	require.Equal(t, TLSVersionUnspecified, zeroValue)
	require.NotEqual(t, TLSVersionUnspecified, TLSVersionInvalid)
	require.NotEqual(t, TLSVersionUnspecified, TLSVersionAuto)
}

func TestTLSVersion_ToJSON(t *testing.T) {
	var tlsVersion TLSVersion
	err := tlsVersion.UnmarshalJSON([]byte(`"foo"`))
	require.Error(t, err)
	require.Equal(t, tlsVersion, TLSVersionInvalid)

	for str, version := range TLSVersions {
		versionJSON, err := json.Marshal(version)
		require.NoError(t, err)
		require.Equal(t, versionJSON, []byte(`"`+str+`"`))

		err = tlsVersion.UnmarshalJSON([]byte(`"` + str + `"`))
		require.NoError(t, err)
		require.Equal(t, tlsVersion, version)
	}
}

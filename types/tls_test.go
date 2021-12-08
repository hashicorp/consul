package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSVersion_Equality(t *testing.T) {
	require.Equal(t, TLSVersionAuto, TLSVersions["TLS_AUTO"])
	require.Equal(t, TLSv1_0, TLSVersions["TLSv1_0"])
	require.Equal(t, TLSv1_1, TLSVersions["TLSv1_1"])
	require.Equal(t, TLSv1_2, TLSVersions["TLSv1_2"])
	require.Equal(t, TLSv1_3, TLSVersions["TLSv1_3"])
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
	err := json.Unmarshal([]byte(`"foo"`), &tlsVersion)
	require.Error(t, err)
	require.Equal(t, tlsVersion, TLSVersionInvalid)

	for str, version := range TLSVersions {
		versionJSON, err := json.Marshal(version)
		require.NoError(t, err)
		require.Equal(t, versionJSON, []byte(`"`+str+`"`))

		err = json.Unmarshal([]byte(`"`+str+`"`), &tlsVersion)
		require.NoError(t, err)
		require.Equal(t, tlsVersion, version)
	}
}

package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTLSVersion_Valid(t *testing.T) {
	require.NoError(t, ValidateTLSVersion("TLS_AUTO"))
	require.NoError(t, ValidateTLSVersion("TLSv1_0"))
	require.NoError(t, ValidateTLSVersion("TLSv1_1"))
	require.NoError(t, ValidateTLSVersion("TLSv1_2"))
	require.NoError(t, ValidateTLSVersion("TLSv1_3"))
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

	// Unmarshalling won't catch invalid version strings,
	// must be checked in config or config entry validation
	err := json.Unmarshal([]byte(`"foo"`), &tlsVersion)
	require.NoError(t, err)

	for version := range tlsVersions {
		str := version.String()
		versionJSON, err := json.Marshal(version)
		require.NoError(t, err)
		require.Equal(t, versionJSON, []byte(`"`+str+`"`))

		err = json.Unmarshal([]byte(`"`+str+`"`), &tlsVersion)
		require.NoError(t, err)
		require.Equal(t, tlsVersion, version)
	}
}

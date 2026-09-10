// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

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

func TestTLSECDHCurves_Validation(t *testing.T) {
	// Valid curves for Consul Agent and Envoy
	require.NoError(t, ValidateConsulAgentECDHCurves([]TLSECDHCurve{CurveX25519MLKEM768, CurveX25519}))
	require.NoError(t, ValidateConsulAgentECDHCurves([]TLSECDHCurve{CurveP256, CurveP384, CurveP521}))
	require.NoError(t, ValidateEnvoyECDHCurves([]string{"X25519MLKEM768", "X25519"}))
	require.NoError(t, ValidateEnvoyECDHCurves([]string{"P-256", "P-384", "P-521"}))

	// Invalid curve
	require.ErrorContains(t, ValidateConsulAgentECDHCurves([]TLSECDHCurve{"secp256k1"}), "no matching Consul Agent TLS curve found for secp256k1")
	require.ErrorContains(t, ValidateEnvoyECDHCurves([]string{"secp256k1"}), `unsupported ecdh_curve "secp256k1"`)

	// Compatibility validation
	require.NoError(t, ValidateTLSVersionECDHCurvesCompat(TLSVersionUnspecified))
	require.NoError(t, ValidateTLSVersionECDHCurvesCompat(TLSVersionAuto))
	require.NoError(t, ValidateTLSVersionECDHCurvesCompat(TLSv1_2))
	require.NoError(t, ValidateTLSVersionECDHCurvesCompat(TLSv1_3))
	require.ErrorContains(t, ValidateTLSVersionECDHCurvesCompat(TLSv1_0), "ecdh_curves can only be configured when tls_min_version is 'TLSv1_2' or higher")
	require.ErrorContains(t, ValidateTLSVersionECDHCurvesCompat(TLSv1_1), "ecdh_curves can only be configured when tls_min_version is 'TLSv1_2' or higher")
}

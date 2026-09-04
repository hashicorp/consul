// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/consul/types"
)

func TestMeshConfigEntry_PeerThroughMeshGateways(t *testing.T) {
	tests := map[string]struct {
		input *MeshConfigEntry
		want  bool
	}{
		"nil entry": {
			input: nil,
			want:  false,
		},
		"nil peering config": {
			input: &MeshConfigEntry{
				Peering: nil,
			},
			want: false,
		},
		"not peering through gateways": {
			input: &MeshConfigEntry{
				Peering: &PeeringMeshConfig{
					PeerThroughMeshGateways: false,
				},
			},
			want: false,
		},
		"peering through gateways": {
			input: &MeshConfigEntry{
				Peering: &PeeringMeshConfig{
					PeerThroughMeshGateways: true,
				},
			},
			want: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equalf(t, tc.want, tc.input.PeerThroughMeshGateways(), "PeerThroughMeshGateways()")
		})
	}
}

func TestMeshConfigEntry_GetHTTPIncomingRequestNormalization(t *testing.T) {
	tests := map[string]struct {
		input *MeshConfigEntry
		want  *RequestNormalizationMeshConfig
	}{
		// Ensure nil is gracefully handled at each level of config path.
		"nil entry": {
			input: nil,
			want:  nil,
		},
		"nil http config": {
			input: &MeshConfigEntry{
				HTTP: nil,
			},
			want: nil,
		},
		"nil http incoming config": {
			input: &MeshConfigEntry{
				HTTP: &MeshHTTPConfig{
					Incoming: nil,
				},
			},
			want: nil,
		},
		"nil http incoming request normalization config": {
			input: &MeshConfigEntry{
				HTTP: &MeshHTTPConfig{
					Incoming: &MeshDirectionalHTTPConfig{
						RequestNormalization: nil,
					},
				},
			},
			want: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.input.GetHTTPIncomingRequestNormalization())
		})
	}
}

func TestMeshConfigEntry_RequestNormalizationMeshConfig(t *testing.T) {
	tests := map[string]struct {
		input *RequestNormalizationMeshConfig
		getFn func(*RequestNormalizationMeshConfig) any
		want  any
	}{
		// Ensure defaults are returned when config is not set.
		"nil entry gets false GetInsecureDisablePathNormalization": {
			input: nil,
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetInsecureDisablePathNormalization()
			},
			want: false,
		},
		"nil entry gets false GetMergeSlashes": {
			input: nil,
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetMergeSlashes()
			},
			want: false,
		},
		"nil entry gets default GetPathWithEscapedSlashesAction": {
			input: nil,
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetPathWithEscapedSlashesAction()
			},
			want: PathWithEscapedSlashesAction("IMPLEMENTATION_SPECIFIC_DEFAULT"),
		},
		"nil entry gets default GetHeadersWithUnderscoresAction": {
			input: nil,
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetHeadersWithUnderscoresAction()
			},
			want: HeadersWithUnderscoresAction("ALLOW"),
		},
		"empty entry gets default GetPathWithEscapedSlashesAction": {
			input: &RequestNormalizationMeshConfig{},
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetPathWithEscapedSlashesAction()
			},
			want: PathWithEscapedSlashesAction("IMPLEMENTATION_SPECIFIC_DEFAULT"),
		},
		"empty entry gets default GetHeadersWithUnderscoresAction": {
			input: &RequestNormalizationMeshConfig{},
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetHeadersWithUnderscoresAction()
			},
			want: HeadersWithUnderscoresAction("ALLOW"),
		},
		// Ensure values are returned when set.
		"non-default entry gets expected InsecureDisablePathNormalization": {
			input: &RequestNormalizationMeshConfig{InsecureDisablePathNormalization: true},
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetInsecureDisablePathNormalization()
			},
			want: true,
		},
		"non-default entry gets expected MergeSlashes": {
			input: &RequestNormalizationMeshConfig{MergeSlashes: true},
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetMergeSlashes()
			},
			want: true,
		},
		"non-default entry gets expected PathWithEscapedSlashesAction": {
			input: &RequestNormalizationMeshConfig{PathWithEscapedSlashesAction: "UNESCAPE_AND_FORWARD"},
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetPathWithEscapedSlashesAction()
			},
			want: PathWithEscapedSlashesAction("UNESCAPE_AND_FORWARD"),
		},
		"non-default entry gets expected HeadersWithUnderscoresAction": {
			input: &RequestNormalizationMeshConfig{HeadersWithUnderscoresAction: "REJECT_REQUEST"},
			getFn: func(c *RequestNormalizationMeshConfig) any {
				return c.GetHeadersWithUnderscoresAction()
			},
			want: HeadersWithUnderscoresAction("REJECT_REQUEST"),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.getFn(tc.input))
		})
	}
}

func TestMeshConfigEntry_validateRequestNormalizationMeshConfig(t *testing.T) {
	tests := map[string]struct {
		input   *RequestNormalizationMeshConfig
		wantErr string
	}{
		"nil entry is valid": {
			input:   nil,
			wantErr: "",
		},
		"invalid PathWithEscapedSlashesAction is rejected": {
			input: &RequestNormalizationMeshConfig{
				PathWithEscapedSlashesAction: PathWithEscapedSlashesAction("INVALID"),
			},
			wantErr: "no matching PathWithEscapedSlashesAction value found for INVALID, please specify one of [IMPLEMENTATION_SPECIFIC_DEFAULT, KEEP_UNCHANGED, REJECT_REQUEST, UNESCAPE_AND_REDIRECT, UNESCAPE_AND_FORWARD]",
		},
		"invalid HeadersWithUnderscoresAction is rejected": {
			input: &RequestNormalizationMeshConfig{
				HeadersWithUnderscoresAction: HeadersWithUnderscoresAction("INVALID"),
			},
			wantErr: "no matching HeadersWithUnderscoresAction value found for INVALID, please specify one of [ALLOW, REJECT_REQUEST, DROP_HEADER]",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.wantErr == "" {
				assert.NoError(t, validateRequestNormalizationMeshConfig(tc.input))
			} else {
				assert.EqualError(t, validateRequestNormalizationMeshConfig(tc.input), tc.wantErr)
			}
		})
	}
}

func TestMeshConfigEntry_validateMeshDirectionalTLSConfig(t *testing.T) {
	tests := map[string]struct {
		input   *MeshDirectionalTLSConfig
		wantErr string
	}{
		"nil config is valid": {
			input:   nil,
			wantErr: "",
		},
		"empty config is valid": {
			input:   &MeshDirectionalTLSConfig{},
			wantErr: "",
		},
		"TLSv1_3 with valid PQC hybrid curves": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_3,
				ECDHCurves:    []string{"X25519MLKEM768", "X25519"},
			},
			wantErr: "",
		},
		"TLSv1_3 with valid NIST curves": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_3,
				ECDHCurves:    []string{"P-256", "P-384", "P-521"},
			},
			wantErr: "",
		},
		"TLSv1_3 with single PQC curve": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_3,
				ECDHCurves:    []string{"X25519MLKEM768"},
			},
			wantErr: "",
		},
		"TLSv1_3 with single X25519 curve": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_3,
				ECDHCurves:    []string{"X25519"},
			},
			wantErr: "",
		},
		"invalid curve name is rejected": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_3,
				ECDHCurves:    []string{"secp256k1"},
			},
			wantErr: `unsupported ecdh_curve "secp256k1"; must be one of [P-256, P-384, P-521, X25519, X25519MLKEM768]`,
		},
		"unsupported curve in multi-curve list is rejected": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_3,
				ECDHCurves:    []string{"X25519", "INVALID_CURVE"},
			},
			wantErr: `unsupported ecdh_curve "INVALID_CURVE"; must be one of [P-256, P-384, P-521, X25519, X25519MLKEM768]`,
		},
		"ECDHCurves with TLSv1_2 is rejected": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_2,
				ECDHCurves:    []string{"X25519MLKEM768"},
			},
			wantErr: "ecdh_curves can only be configured when tls_min_version is 'TLSv1_3'",
		},
		"ECDHCurves with TLSv1_1 is rejected": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_1,
				ECDHCurves:    []string{"X25519"},
			},
			wantErr: "ecdh_curves can only be configured when tls_min_version is 'TLSv1_3'",
		},
		"ECDHCurves with TLSv1_0 is rejected": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSv1_0,
				ECDHCurves:    []string{"P-256"},
			},
			wantErr: "ecdh_curves can only be configured when tls_min_version is 'TLSv1_3'",
		},
		"ECDHCurves with unspecified TLS version is rejected": {
			input: &MeshDirectionalTLSConfig{
				TLSMinVersion: types.TLSVersionUnspecified,
				ECDHCurves:    []string{"X25519MLKEM768"},
			},
			wantErr: "ecdh_curves can only be configured when tls_min_version is 'TLSv1_3'",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateMeshDirectionalTLSConfig(tc.input)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

func TestMeshConfigEntry_Validate_DirectionalTLS(t *testing.T) {
	tests := map[string]struct {
		entry   *MeshConfigEntry
		wantErr string
	}{
		"valid incoming and outgoing PQC curves": {
			entry: &MeshConfigEntry{
				TLS: &MeshTLSConfig{
					Incoming: &MeshDirectionalTLSConfig{
						TLSMinVersion: types.TLSv1_3,
						ECDHCurves:    []string{"X25519MLKEM768", "X25519"},
					},
					Outgoing: &MeshDirectionalTLSConfig{
						TLSMinVersion: types.TLSv1_3,
						ECDHCurves:    []string{"P-384"},
					},
				},
			},
			wantErr: "",
		},
		"invalid incoming curve propagates error": {
			entry: &MeshConfigEntry{
				TLS: &MeshTLSConfig{
					Incoming: &MeshDirectionalTLSConfig{
						TLSMinVersion: types.TLSv1_3,
						ECDHCurves:    []string{"bad-curve"},
					},
				},
			},
			wantErr: `error in incoming TLS configuration: unsupported ecdh_curve "bad-curve"; must be one of [P-256, P-384, P-521, X25519, X25519MLKEM768]`,
		},
		"invalid outgoing version mismatch propagates error": {
			entry: &MeshConfigEntry{
				TLS: &MeshTLSConfig{
					Outgoing: &MeshDirectionalTLSConfig{
						TLSMinVersion: types.TLSv1_2,
						ECDHCurves:    []string{"X25519MLKEM768"},
					},
				},
			},
			wantErr: "error in outgoing TLS configuration: ecdh_curves can only be configured when tls_min_version is 'TLSv1_3'",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.wantErr)
			}
		})
	}
}

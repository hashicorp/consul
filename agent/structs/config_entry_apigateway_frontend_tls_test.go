// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIGatewayFrontendTLSNormalizeValidateAndSelect(t *testing.T) {
	entry := &APIGatewayConfigEntry{
		Name: "gateway",
		Listeners: []APIGatewayListener{{
			Name:     "https",
			Port:     443,
			Protocol: ListenerProtocolHTTP,
		}},
		FrontendTLS: &APIGatewayFrontendTLSConfig{
			Default: &APIGatewayFrontendTLSValidation{
				CACertificateRefs: []ResourceReference{{Name: "default-ca"}},
			},
			PerPort: []APIGatewayFrontendTLSPortConfig{{
				Port: 8443,
				Validation: &APIGatewayFrontendTLSValidation{
					Mode:              APIGatewayFrontendValidationAllowValidOnly,
					CACertificateRefs: []ResourceReference{{Name: "partner-ca"}},
				},
			}},
		},
	}

	require.NoError(t, entry.Normalize())
	require.NoError(t, entry.Validate())
	require.Equal(t, APIGatewayFrontendValidationAllowValidOnly, entry.FrontendTLS.Default.Mode)
	require.Equal(t, FileSystemCertificate, entry.FrontendTLS.Default.CACertificateRefs[0].Kind)
	require.Equal(t, "default-ca", entry.FrontendTLS.ValidationForPort(443).CACertificateRefs[0].Name)
	require.Equal(t, "partner-ca", entry.FrontendTLS.ValidationForPort(8443).CACertificateRefs[0].Name)
}

func TestAPIGatewayFrontendTLSValidation(t *testing.T) {
	base := func() *APIGatewayConfigEntry {
		return &APIGatewayConfigEntry{
			Name: "gateway",
			Listeners: []APIGatewayListener{{
				Name:     "https",
				Port:     443,
				Protocol: ListenerProtocolHTTP,
			}},
		}
	}

	t.Run("invalid mode", func(t *testing.T) {
		entry := base()
		entry.FrontendTLS = &APIGatewayFrontendTLSConfig{
			Default: &APIGatewayFrontendTLSValidation{
				Mode:              "invalid",
				CACertificateRefs: []ResourceReference{{Kind: FileSystemCertificate, Name: "ca"}},
			},
		}
		require.ErrorContains(t, entry.Validate(), "unsupported frontend TLS validation mode")
	})

	t.Run("duplicate port", func(t *testing.T) {
		entry := base()
		validation := &APIGatewayFrontendTLSValidation{
			Mode:              APIGatewayFrontendValidationAllowValidOnly,
			CACertificateRefs: []ResourceReference{{Kind: FileSystemCertificate, Name: "ca"}},
		}
		entry.FrontendTLS = &APIGatewayFrontendTLSConfig{
			PerPort: []APIGatewayFrontendTLSPortConfig{
				{Port: 443, Validation: validation},
				{Port: 443, Validation: validation},
			},
		}
		require.ErrorContains(t, entry.Validate(), "configured more than once")
	})

	t.Run("multiple references are rejected", func(t *testing.T) {
		entry := base()
		entry.FrontendTLS = &APIGatewayFrontendTLSConfig{
			Default: &APIGatewayFrontendTLSValidation{
				Mode: APIGatewayFrontendValidationAllowValidOnly,
				CACertificateRefs: []ResourceReference{
					{Kind: FileSystemCertificate, Name: "ca-a"},
					{Kind: FileSystemCertificate, Name: "ca-b"},
				},
			},
		}
		require.ErrorContains(t, entry.Validate(), "exactly one CA certificate")
	})
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIGatewayFrontendTLSJSONRoundTrip(t *testing.T) {
	input := &APIGatewayConfigEntry{
		Kind: APIGateway,
		Name: "gateway",
		FrontendTLS: &APIGatewayFrontendTLSConfig{
			Default: &APIGatewayFrontendTLSValidation{
				Mode: APIGatewayFrontendValidationAllowValidOnly,
				CACertificateRefs: []ResourceReference{{
					Kind:      FileSystemCertificate,
					Name:      "client-ca",
					Namespace: "payments",
					Partition: "prod",
				}},
			},
		},
	}

	data, err := json.Marshal(input)
	require.NoError(t, err)

	var output APIGatewayConfigEntry
	require.NoError(t, json.Unmarshal(data, &output))
	require.Equal(t, input.FrontendTLS, output.FrontendTLS)
	require.Equal(t, "client-ca", output.FrontendTLS.ValidationForPort(443).CACertificateRefs[0].Name)
}

// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package extauthz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	ext_cmn "github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

// TestAPIGatewayExtAuthzSupported_CE verifies that API Gateway support for the
// builtin/ext-authz extension is disabled in CE. This is an enterprise-only
// capability; the enterprise build overrides it to true.
func TestAPIGatewayExtAuthzSupported_CE(t *testing.T) {
	require.False(t, apiGatewayExtAuthzSupported())
}

// TestNewExtAuthz_APIGatewayProxyTypeRejectedInCE verifies that configuring the
// extension with an api-gateway ProxyType is rejected in CE, and that the error
// only advertises connect-proxy as a supported value.
func TestNewExtAuthz_APIGatewayProxyTypeRejectedInCE(t *testing.T) {
	_, err := newExtAuthz(api.EnvoyExtension{
		Name: api.BuiltinExtAuthzExtension,
		Arguments: map[string]any{
			"ProxyType": string(api.ServiceKindAPIGateway),
			"Config": map[string]any{
				"GrpcService": map[string]any{
					"Target": map[string]any{"URI": "localhost:9191"},
				},
			},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), `unsupported ProxyType "api-gateway"`)
	require.Contains(t, err.Error(), `supported values are "connect-proxy"`)
}

// TestExtAuthz_CanApply_APIGatewayNotSupportedInCE verifies that the extension
// cannot be applied to an API Gateway proxy in CE, while connect-proxy is still
// supported.
func TestExtAuthz_CanApply_APIGatewayNotSupportedInCE(t *testing.T) {
	a := &extAuthz{}
	require.True(t, a.CanApply(&ext_cmn.RuntimeConfig{Kind: api.ServiceKindConnectProxy}))
	require.False(t, a.CanApply(&ext_cmn.RuntimeConfig{Kind: api.ServiceKindAPIGateway}))
}

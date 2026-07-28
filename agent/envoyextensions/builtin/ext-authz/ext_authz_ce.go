// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package extauthz

// apiGatewayExtAuthzSupported reports whether the builtin/ext-authz extension may
// be applied to API Gateway proxies. This is an enterprise-only capability, so it
// is always false in CE; the enterprise build overrides it to true.
func apiGatewayExtAuthzSupported() bool {
	return false
}

// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package extensionruntime

import (
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

// appendAPIGatewayExtAuthzUpstreams is a no-op in CE. Surfacing an API Gateway's
// compiled discovery chains as builtin/ext-authz upstream targets (so an
// ext-authz extension can resolve a mesh Service's cluster/SNI) is an
// enterprise-only feature; the enterprise build provides the real implementation.
func appendAPIGatewayExtAuthzUpstreams(
	_ *proxycfg.ConfigSnapshot,
	_ map[api.CompoundServiceName]*extensioncommon.UpstreamData,
	_ map[api.CompoundServiceName][]api.EnvoyExtension,
	_ string,
) {
}

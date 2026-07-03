// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfg

import "context"

// watchExtAuthzMeshTargets is a no-op in CE. Watching the discovery chains of
// builtin/ext-authz mesh (Service) targets declared on an API Gateway's
// EnvoyExtensions is an enterprise-only feature; the enterprise build provides
// the real implementation.
func (h *handlerAPIGateway) watchExtAuthzMeshTargets(_ context.Context, _ *ConfigSnapshot) error {
	return nil
}

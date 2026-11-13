// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import "github.com/hashicorp/consul/lifecycle"

// ACLResolverComponent is the lifecycle component name of the server ACL resolver.
const ACLResolverComponent lifecycle.ComponentName = "server/acl-resolver"

// registerACLResolverLifecycle registers the ACL resolver with the lifecycle
// Exchange. It is for visibility only: nothing is gated on it yet. The resolver
// is constructed and usable by the time this is called, so it is Ready.
func (s *Server) registerACLResolverLifecycle(ex *lifecycle.Exchange) {
	h, err := ex.Register(lifecycle.Simple(ACLResolverComponent))
	if err != nil {
		s.logger.Warn("failed to register ACL resolver lifecycle component", "error", err)
		return
	}
	if err := h.Transition(lifecycle.StateReady, lifecycle.WithReason("ACL resolver created")); err != nil {
		s.logger.Warn("failed to mark ACL resolver ready", "error", err)
	}
}

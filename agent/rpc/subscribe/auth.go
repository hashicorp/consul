package subscribe

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
)

// EnforceACL takes an acl.Authorizer and returns the decision for whether the
// event is allowed to be sent to this client or not.
func enforceACL(authz acl.Authorizer, e stream.Event) acl.EnforcementDecision {
	switch {
	case e.IsEndOfSnapshot(), e.IsEndOfEmptySnapshot():
		return acl.Allow
	}

	switch p := e.Payload.(type) {
	case state.EventPayloadCheckServiceNode:
		csn := p.Value
		if csn.Node == nil || csn.Service == nil || csn.Node.Node == "" || csn.Service.Service == "" {
			return acl.Deny
		}

		// TODO: what about acl.Default?
		// TODO(streaming): we need the AuthorizerContext for ent
		if dec := authz.NodeRead(csn.Node.Node, nil); dec != acl.Allow {
			return acl.Deny
		}

		// TODO(streaming): we need the AuthorizerContext for ent
		// Enterprise support for streaming events - they don't have enough data to
		// populate it yet.
		if dec := authz.ServiceRead(csn.Service.Service, nil); dec != acl.Allow {
			return acl.Deny
		}
		return acl.Allow
	}

	return acl.Deny
}

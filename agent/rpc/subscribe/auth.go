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
	case e.IsEndOfSnapshot(), e.IsNewSnapshotToFollow():
		return acl.Allow
	}

	switch p := e.Payload.(type) {
	case state.EventPayloadCheckServiceNode:
		return p.Value.CanRead(authz)
	}
	return acl.Deny
}

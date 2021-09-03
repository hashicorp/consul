package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

var ACLEndpointLegacySummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"acl", "apply"},
		Help: "Measures the time it takes to complete an update to the ACL store.",
	},
}

func (a *ACL) Bootstrap(*structs.DCSpecificRequest, *structs.ACL) error {
	return fmt.Errorf("ACL.Bootstrap: the legacy ACL system has been removed")
}

// aclApplyInternal is used to apply an ACL request after it has been vetted that
// this is a valid operation. It is used when users are updating ACLs, in which
// case we check their token to make sure they have management privileges. It is
// also used for ACL replication. We want to run the replicated ACLs through the
// same checks on the change itself.
func aclApplyInternal(srv *Server, args *structs.ACLRequest, reply *string) error {
	// All ACLs must have an ID by this point.
	if args.ACL.ID == "" {
		return fmt.Errorf("Missing ACL ID")
	}

	switch args.Op {
	case structs.ACLSet:
		// Verify the ACL type
		switch args.ACL.Type {
		case structs.ACLTokenTypeClient:
		case structs.ACLTokenTypeManagement:
		default:
			return fmt.Errorf("Invalid ACL Type")
		}

		// No need to check expiration times as those did not exist in legacy tokens.
		_, existing, _ := srv.fsm.State().ACLTokenGetBySecret(nil, args.ACL.ID, nil)
		if existing != nil && existing.UsesNonLegacyFields() {
			return fmt.Errorf("Cannot use legacy endpoint to modify a non-legacy token")
		}

		// Verify this is not a root ACL
		if acl.RootAuthorizer(args.ACL.ID) != nil {
			return acl.PermissionDeniedError{Cause: "Cannot modify root ACL"}
		}

		// Ensure that we allow more permissive rule formats for legacy tokens,
		// but that we correct them on the way into the system.
		//
		// DEPRECATED (ACL-Legacy-Compat)
		correctedRules := structs.SanitizeLegacyACLTokenRules(args.ACL.Rules)
		if correctedRules != "" {
			args.ACL.Rules = correctedRules
		}

		// Validate the rules compile
		_, err := acl.NewPolicyFromSource("", 0, args.ACL.Rules, acl.SyntaxLegacy, srv.aclConfig, nil)
		if err != nil {
			return fmt.Errorf("ACL rule compilation failed: %v", err)
		}

	case structs.ACLDelete:
		if args.ACL.ID == anonymousToken {
			return acl.PermissionDeniedError{Cause: "Cannot delete anonymous token"}
		}

	default:
		return fmt.Errorf("Invalid ACL Operation")
	}

	// Apply the update
	resp, err := srv.raftApply(structs.ACLRequestType, args)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}

	// Check if the return type is a string
	if respString, ok := resp.(string); ok {
		*reply = respString
	}

	return nil
}

// Apply is used to apply a modifying request to the data store. This should
// only be used for operations that modify the data
func (a *ACL) Apply(args *structs.ACLRequest, reply *string) error {
	if done, err := a.srv.ForwardRPC("ACL.Apply", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"acl", "apply"}, time.Now())

	// Verify we are allowed to serve this request
	if !a.srv.config.ACLsEnabled {
		return acl.ErrDisabled
	}

	// Verify token is permitted to modify ACLs
	// NOTE: We will not support enterprise authorizer contexts with legacy ACLs
	if authz, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if authz.ACLWrite(nil) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	// If no ID is provided, generate a new ID. This must be done prior to
	// appending to the Raft log, because the ID is not deterministic. Once
	// the entry is in the log, the state update MUST be deterministic or
	// the followers will not converge.
	if args.Op == structs.ACLSet && args.ACL.ID == "" {
		var err error
		args.ACL.ID, err = lib.GenerateUUID(a.srv.checkTokenUUID)
		if err != nil {
			return err
		}
	}

	// Do the apply now that this update is vetted.
	if err := aclApplyInternal(a.srv, args, reply); err != nil {
		return err
	}

	// Clear the cache if applicable
	if args.ACL.ID != "" {
		a.srv.acls.cache.RemoveIdentity(tokenSecretCacheID(args.ACL.ID))
	}

	return nil
}

func (a *ACL) Get(*structs.ACLSpecificRequest, *structs.IndexedACLs) error {
	return fmt.Errorf("ACL.Get: the legacy ACL system has been removed")
}

func (a *ACL) List(*structs.DCSpecificRequest, *structs.IndexedACLs) error {
	return fmt.Errorf("ACL.List: the legacy ACL system has been removed")
}

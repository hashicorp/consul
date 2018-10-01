package consul

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
)

const (
	// aclBootstrapReset is the file name to create in the data dir. It's only contents
	// should be the reset index
	aclBootstrapReset = "acl-bootstrap-reset"
)

// Regex for matching
var validPolicyName = regexp.MustCompile(`[A-Za-z0-9\\-_]+`)

// ACL endpoint is used to manipulate ACLs
type ACL struct {
	srv *Server
}

func (a *ACL) fileBootstrapResetIndex() uint64 {
	// Determine the file path to check
	path := filepath.Join(a.srv.config.DataDir, aclBootstrapReset)

	// Read the file
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			a.srv.logger.Printf("[ERR] acl.bootstrap: failed to read %q: %v", path, err)
		}
		return 0
	}

	// Attempt to parse the file
	var resetIdx uint64
	if _, err := fmt.Sscanf(string(raw), "%d", &resetIdx); err != nil {
		a.srv.logger.Printf("[ERR] acl.bootstrap: failed to parse %q: %v", path, err)
		return 0
	}

	// Return the reset index
	a.srv.logger.Printf("[WARN] acl.bootstrap: parsed %q: reset index %d", path, resetIdx)
	return resetIdx
}

// Bootstrap is used to perform a one-time ACL bootstrap operation on
// a cluster to get the first management token.
func (a *ACL) Bootstrap(args *structs.DCSpecificRequest, reply *structs.ACLToken) error {
	if done, err := a.srv.forward("ACL.Bootstrap", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if a.srv.config.ACLDatacenter != a.srv.config.Datacenter {
		return acl.ErrDisabled
	}

	// By doing some pre-checks we can head off later bootstrap attempts
	// without having to run them through Raft, which should curb abuse.
	state := a.srv.fsm.State()
	allowed, resetIdx, err := state.CanBootstrapACLToken()
	if err != nil {
		return err
	}
	var specifiedIndex uint64 = 0
	if !allowed {
		// Check if there is a reset index specified
		specifiedIndex = a.fileBootstrapResetIndex()
		if specifiedIndex == 0 {
			return fmt.Errorf("ACL bootstrap already done (reset index: %d)", resetIdx)
		} else if specifiedIndex != resetIdx {
			return fmt.Errorf("Invalid bootstrap reset index (specified %d, reset index: %d)", specifiedIndex, resetIdx)
		}

	}

	accessor, err := lib.GenerateUUID(a.srv.checkTokenUUID)
	if err != nil {
		return err
	}
	secret, err := lib.GenerateUUID(a.srv.checkTokenUUID)
	if err != nil {
		return err
	}

	req := structs.ACLTokenBootstrapRequest{
		Datacenter: a.srv.config.Datacenter,
		ACLToken: structs.ACLToken{
			AccessorID:  accessor,
			SecretID:    secret,
			Description: "Bootstrap Token (Global Management)",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			CreateTime: time.Now(),
			// DEPRECATED (ACL-Legacy-Compat) - This is used so that the bootstrap token is still visible via the v1 acl APIs
			Type:  structs.ACLTokenTypeManagement,
			Local: false,
		},
		ResetIndex: specifiedIndex,
	}

	req.ACLToken.SetHash(true)

	resp, err := a.srv.raftApply(structs.ACLBootstrapRequestType, &req)
	if err != nil {
		return err
	}
	switch v := resp.(type) {
	case error:
		return v

	case *structs.ACLToken:
		*reply = *v

	default:
		// Just log this, since it looks like the bootstrap may have
		// completed.
		a.srv.logger.Printf("[ERR] consul.acl: Unexpected response during bootstrap: %T", v)
	}

	a.srv.logger.Printf("[INFO] consul.acl: ACL bootstrap completed")
	return nil
}

func (a *ACL) TokenRead(args *structs.ACLTokenReadRequest, reply *structs.ACLTokenResponse) error {
	// TODO (ACL-V2) - Implement checking for local tokens prior to hitting the ACL DC
	if done, err := a.srv.forward("ACL.TokenRead", args, args, reply); done {
		return err
	}

	if !a.srv.ACLsEnabled() {
		return acl.ErrDisabled
	}

	var rule acl.Authorizer
	if args.IDType == structs.ACLTokenAccessor {
		var err error
		// Only ACLRead privileges are required to list tokens
		// However if you do not have ACLWrite as well the token
		// secrets will be redacted
		if rule, err = a.srv.ResolveToken(args.Token); err != nil {
			return err
		} else if rule == nil || !rule.ACLRead() {
			return acl.ErrPermissionDenied
		}
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var token *structs.ACLToken
			var err error

			if args.IDType == structs.ACLTokenAccessor {
				index, token, err = state.ACLTokenGetByAccessor(ws, args.ID)
				if token != nil {
					a.srv.filterACLWithAuthorizer(rule, &token)
				}
			} else {
				index, token, err = state.ACLTokenGetBySecret(ws, args.ID)
			}

			if err != nil {
				return err
			}

			reply.Index, reply.Token = index, token
			return nil
		})
}

func (a *ACL) TokenClone(args *structs.ACLTokenWriteRequest, reply *structs.ACLToken) error {
	// TODO (ACL-V2) - handle local tokens
	if done, err := a.srv.forward("ACL.TokenClone", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "token", "clone"}, time.Now())

	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLWrite() {
		return acl.ErrPermissionDenied
	}

	_, token, err := a.srv.fsm.State().ACLTokenGetByAccessor(nil, args.ACLToken.AccessorID)
	if err != nil {
		return err
	} else if token == nil {
		return acl.ErrNotFound
	}

	if token.Rules != "" {
		return fmt.Errorf("Cannot clone a legacy ACL with this endpoint")
	}

	cloneReq := structs.ACLTokenWriteRequest{
		Datacenter: args.Datacenter,
		Op:         structs.ACLSet,
		ACLToken: structs.ACLToken{
			Policies:    token.Policies,
			Local:       token.Local,
			Description: token.Description,
		},
	}

	cloneReq.Token = args.Token

	if args.ACLToken.Description != "" {
		cloneReq.ACLToken.Description = args.ACLToken.Description
	}

	return a.TokenWrite(&cloneReq, reply)

}

func (a *ACL) TokenWrite(args *structs.ACLTokenWriteRequest, reply *structs.ACLToken) error {
	token := &args.ACLToken
	// TODO (ACL-V2) - handle local tokens
	if done, err := a.srv.forward("ACL.TokenWrite", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "token", "write"}, time.Now())

	// Verify token is permitted to modify ACLs
	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLWrite() {
		return acl.ErrPermissionDenied
	}

	state := a.srv.fsm.State()

	switch args.Op {
	case structs.ACLSet:
		if token.AccessorID == "" {
			// Token Create
			var err error

			// Generate the AccessorID
			token.AccessorID, err = lib.GenerateUUID(a.srv.checkTokenUUID)
			if err != nil {
				return err
			}

			// Generate the SecretID - not supporting non-UUID secrets
			token.SecretID, err = lib.GenerateUUID(a.srv.checkTokenUUID)
			if err != nil {
				return err
			}

			token.CreateTime = time.Now()
		} else {
			// Token Update
			if _, err := uuid.ParseUUID(token.AccessorID); err != nil {
				return fmt.Errorf("AccessorID is not a valid UUID")
			}

			// DEPRECATED (ACL-Legacy-Compat) - maybe get rid of this in the future
			//   and instead do a ParseUUID check. New tokens will not have
			//   secrets generated by users but rather they will always be UUIDs.
			//   However if users just continue the upgrade cycle they may still
			//   have tokens using secrets that are not UUIDS
			// TODO (ACL-V2) - Do we really need a RootAuthorizer check here? If so should
			//   the message be updated?
			if acl.RootAuthorizer(token.SecretID) != nil {
				return acl.PermissionDeniedError{Cause: "Cannot modify root ACL"}
			}

			// Verify the token exists
			_, existing, err := state.ACLTokenGetByAccessor(nil, token.AccessorID)
			if err != nil {
				return fmt.Errorf("Failed to lookup the acl token %q: %v", token.AccessorID, err)
			}
			if existing == nil {
				return fmt.Errorf("Cannot find token %q", token.AccessorID)
			}
			if existing.SecretID != token.SecretID {
				return fmt.Errorf("Changing a tokens SecretID is not permitted")
			}

			// Cannot toggle the "Global" mode
			if token.Local != existing.Local {
				return fmt.Errorf("cannot toggle local mode of %s", token.AccessorID)
			}

			token.CreateTime = existing.CreateTime
		}

		// Validate all the policy names and convert them to policy IDs
		for linkIndex, link := range token.Policies {
			if link.ID == "" {
				_, policy, err := state.ACLPolicyGetByName(nil, link.Name)
				if err != nil {
					return fmt.Errorf("Error looking up policy for name %q: %v", link.Name, err)
				}
				if policy == nil {
					return fmt.Errorf("No such ACL policy with name %q", link.Name)
				}
				token.Policies[linkIndex].ID = policy.ID
			}

			// Do not store the policy name within raft/memdb as the policy could be renamed in the future.
			token.Policies[linkIndex].Name = ""
		}

		if token.Rules != "" {
			return fmt.Errorf("Rules cannot be specified for this token")
		}

		if token.Type != "" {
			return fmt.Errorf("Type cannot be specified for this token")
		}

		token.SetHash(true)
	case structs.ACLDelete:
		if _, err := uuid.ParseUUID(token.AccessorID); err != nil {
			return fmt.Errorf("Accessor ID is missing or an invalid UUID for operation: %v", args.Op)
		}

		if token.AccessorID == structs.ACLTokenAnonymousID {
			return fmt.Errorf("Delete operation not permitted on the anonymous token")
		}
	default:
		return fmt.Errorf("Invalid TokenWrite operation: %v", args.Op)
	}

	resp, err := a.srv.raftApply(structs.ACLTokenRequestType, args)
	if err != nil {
		return fmt.Errorf("Failed to apply token write request: %v", err)
	}

	// Purge the identity from the cache to prevent using the previous definition of the identity
	a.srv.acls.cache.RemoveIdentity(token.SecretID)

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	if respToken, ok := resp.(*structs.ACLToken); ok {
		*reply = *respToken
	}

	return nil
}

func (a *ACL) TokenList(args *structs.DCSpecificRequest, reply *structs.ACLTokensResponse) error {
	// TODO (ACL-V2) - Implement listing  local tokens prior to hitting the ACL DC
	if done, err := a.srv.forward("ACL.TokenList", args, args, reply); done {
		return err
	}

	rule, err := a.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	} else if rule == nil || !rule.ACLRead() {
		return acl.ErrPermissionDenied
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		// TODO (ACL-V2) - implement secret removal
		func(ws memdb.WatchSet, state *state.Store) error {
			index, tokens, err := state.ACLTokenList(ws)
			if err != nil {
				return err
			}

			a.srv.filterACLWithAuthorizer(rule, &tokens)
			reply.Index, reply.Tokens = index, tokens
			return nil
		})
}

func (a *ACL) PolicyRead(args *structs.ACLPolicyReadRequest, reply *structs.ACLPolicyResponse) error {
	if done, err := a.srv.forward("ACL.PolicyRead", args, args, reply); done {
		return err
	}

	if !a.srv.ACLsEnabled() {
		return acl.ErrDisabled
	}

	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLRead() {
		return acl.ErrPermissionDenied
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var policy *structs.ACLPolicy
			var err error
			if args.IDType == structs.ACLPolicyID {
				index, policy, err = state.ACLPolicyGetByID(ws, args.ID)
			} else {
				index, policy, err = state.ACLPolicyGetByName(ws, args.ID)
			}

			if err != nil {
				return err
			}

			reply.Index, reply.Policy = index, policy
			return nil
		})
}

func (a *ACL) PolicyWrite(args *structs.ACLPolicyWriteRequest, reply *structs.ACLPolicy) error {
	// make sure this RPC is destined for the ACLDatacenter
	args.Datacenter = a.srv.config.ACLDatacenter

	if done, err := a.srv.forward("ACL.PolicyWrite", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "policy", "write"}, time.Now())

	// Verify token is permitted to modify ACLs
	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLWrite() {
		return acl.ErrPermissionDenied
	}

	policy := &args.Policy
	state := a.srv.fsm.State()

	// Almost all of the checks here are also done in the state store. However,
	// we want to prevent the raft operations when we know they are going to fail
	// so we still do them here.
	switch args.Op {
	case structs.ACLSet:
		// ensure a name is set
		if policy.Name == "" {
			return fmt.Errorf("Invalid Policy: no Name is set")
		}

		if !validPolicyName.MatchString(policy.Name) {
			return fmt.Errorf("Invalid Policy: invalid Name. Only alphanumeric characters, '-' and '_' are allowed")
		}

		if policy.ID == "" {
			// with no policy ID one will be generated
			var err error

			policy.ID, err = lib.GenerateUUID(a.srv.checkPolicyUUID)
			if err != nil {
				return err
			}

			// validate the name is unique
			if _, existing, err := state.ACLPolicyGetByName(nil, policy.Name); err != nil {
				return fmt.Errorf("acl policy lookup by name failed: %v", err)
			} else if existing != nil {
				return fmt.Errorf("Invalid Policy: A Policy with Name %q already exists", policy.Name)
			}
		} else {
			if _, err := uuid.ParseUUID(policy.ID); err != nil {
				return fmt.Errorf("Policy ID invalid UUID for operation: %v", args.Op)
			}

			// Verify the policy exists
			_, existing, err := state.ACLPolicyGetByID(nil, policy.ID)
			if err != nil {
				return fmt.Errorf("acl policy lookup failed: %v", err)
			} else if existing == nil {
				return fmt.Errorf("cannot find policy %s", policy.ID)
			}

			if existing.Name != policy.Name {
				if _, nameMatch, err := state.ACLPolicyGetByName(nil, policy.Name); err != nil {
					return fmt.Errorf("acl policy lookup by name failed: %v", err)
				} else if nameMatch != nil {
					return fmt.Errorf("Invalid Policy: A policy with name %q already exists", policy.Name)
				}
			}

			if policy.ID == structs.ACLPolicyGlobalManagementID {
				if policy.Datacenters != nil || len(policy.Datacenters) > 0 {
					return fmt.Errorf("Changing the Datacenters of the builtin global-management policy is not permitted")
				}

				if policy.Rules != existing.Rules {
					return fmt.Errorf("Changing the Rules for the builtin global-management policy is not permitted")
				}
			}
		}

		// validate the rules
		_, err := acl.NewPolicyFromSource("", 0, policy.Rules, policy.Syntax, a.srv.sentinel)
		if err != nil {
			return err
		}

		// calcualte the hash for this policy
		policy.SetHash(true)
	case structs.ACLDelete:
		if _, err := uuid.ParseUUID(policy.ID); err != nil {
			return fmt.Errorf("Policy ID is missing or an invalid UUID for operation: %v", args.Op)
		}

		if policy.ID == structs.ACLPolicyGlobalManagementID {
			return fmt.Errorf("Delete operation not permitted on the builtin global-management policy")
		}
	default:
		return fmt.Errorf("Invalid operation for the PolicyWrite RPC: %v", args.Op)
	}

	resp, err := a.srv.raftApply(structs.ACLPolicyRequestType, args)
	if err != nil {
		return fmt.Errorf("Failed to apply policy write request: %v", err)
	}

	a.srv.acls.cache.RemovePolicy(policy.ID)

	if resp == nil {
		return nil
	}

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	if respPolicy, ok := resp.(*structs.ACLPolicy); ok {
		*reply = *respPolicy
	}

	return nil
}

func (a *ACL) PolicyList(args *structs.DCSpecificRequest, reply *structs.ACLPolicyMultiResponse) error {
	if done, err := a.srv.forward("ACL.PolicyList", args, args, reply); done {
		return err
	}

	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLRead() {
		return acl.ErrPermissionDenied
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, policies, err := state.ACLPolicyList(ws)
			if err != nil {
				return err
			}

			reply.Index, reply.Policies = index, policies
			return nil
		})
}

func (a *ACL) PolicyResolve(args *structs.ACLPolicyResolveRequest, reply *structs.ACLPolicyMultiResponse) error {
	if done, err := a.srv.forward("ACL.PolicyResolve", args, args, reply); done {
		return err
	}

	// get full list of policies for this token
	policies, err := a.srv.acls.resolveTokenToPolicies(args.Token)
	if err != nil {
		return err
	}

	// filter down the requested list to just what was requested
	for _, policy := range policies {
		for _, policyID := range args.IDs {
			if policy.ID == policyID {
				reply.Policies = append(reply.Policies, policy)
				break
			}
		}
	}
	a.srv.setQueryMeta(&reply.QueryMeta)

	return nil
}

// makeACLETag returns an ETag for the given parent and policy.
func makeACLETag(parent string, policy *acl.Policy) string {
	return fmt.Sprintf("%s:%s", parent, policy.ID)
}

// GetPolicy is used to retrieve a compiled policy object with a TTL. Does not
// support a blocking query.
func (a *ACL) GetPolicy(args *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
	// TODO (ACL-V2) - implement local token support
	if done, err := a.srv.forward("ACL.GetPolicy", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if a.srv.config.ACLDatacenter != a.srv.config.Datacenter {
		return acl.ErrDisabled
	}

	// Get the policy via the cache
	parent := a.srv.config.ACLDefaultPolicy

	policy, err := a.srv.acls.GetMergedPolicyForToken(args.TokenSecret())
	if err != nil {
		return err
	}

	// Generate an ETag
	etag := makeACLETag(parent, policy)

	// Setup the response
	reply.ETag = etag
	reply.TTL = a.srv.config.ACLTokenTTL
	a.srv.setQueryMeta(&reply.QueryMeta)

	// Only send the policy on an Etag mis-match
	if args.ETag != etag {
		reply.Parent = parent
		reply.Policy = policy
	}
	return nil
}

// ReplicationStatus is used to retrieve the current ACL replication status.
func (a *ACL) ReplicationStatus(args *structs.DCSpecificRequest,
	reply *structs.ACLReplicationStatus) error {
	// This must be sent to the leader, so we fix the args since we are
	// re-using a structure where we don't support all the options.
	args.RequireConsistent = true
	args.AllowStale = false
	if done, err := a.srv.forward("ACL.ReplicationStatus", args, args, reply); done {
		return err
	}

	// There's no ACL token required here since this doesn't leak any
	// sensitive information, and we don't want people to have to use
	// management tokens if they are querying this via a health check.

	// Poll the latest status.
	a.srv.aclReplicationStatusLock.RLock()
	*reply = a.srv.aclReplicationStatus
	a.srv.aclReplicationStatusLock.RUnlock()
	return nil
}

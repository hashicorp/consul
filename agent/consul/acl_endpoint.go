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
var validPolicyName = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,128}$`)

// ACL endpoint is used to manipulate ACLs
type ACL struct {
	srv *Server
}

// fileBootstrapResetIndex retrieves the reset index specified by the adminstrator from
// the file on disk.
//
// Q: What is the bootstrap reset index?
// A: If you happen to lose acess to all tokens capable of ACL management you need a way
//    to get back into your system. This allows an admin to write the current
//    bootstrap "index" into a special file on disk to override the mechanism preventing
//    a second token bootstrap. The index will be retrieved by a API call to /v1/acl/bootstrap
//    When already bootstrapped this API will return the reset index necessary within
//    the error response. Once set in the file, the bootstrap API can be used again to
//    get a new token.
//
// Q: Why is the reset index not in the config?
// A: We want to be able to remove the reset index once we have used it. This prevents
//    accidentally allowing bootstrapping yet again after a snapshot restore.
//
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
	a.srv.logger.Printf("[DEBUG] acl.bootstrap: parsed %q: reset index %d", path, resetIdx)
	return resetIdx
}

func (a *ACL) removeBootstrapResetFile() {
	if err := os.Remove(filepath.Join(a.srv.config.DataDir, aclBootstrapReset)); err != nil {
		a.srv.logger.Printf("[WARN] acl.bootstrap: failed to remove bootstrap file: %v", err)
	}
}

func (a *ACL) aclPreCheck() error {
	if !a.srv.ACLsEnabled() {
		return acl.ErrDisabled
	}

	if a.srv.UseLegacyACLs() {
		return fmt.Errorf("The ACL system is currently in legacy mode.")
	}

	return nil
}

// Bootstrap is used to perform a one-time ACL bootstrap operation on
// a cluster to get the first management token.
func (a *ACL) BootstrapTokens(args *structs.DCSpecificRequest, reply *structs.ACLToken) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}
	if done, err := a.srv.forward("ACL.BootstrapTokens", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if !a.srv.InACLDatacenter() {
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
			return fmt.Errorf("ACL bootstrap no longer allowed (reset index: %d)", resetIdx)
		} else if specifiedIndex != resetIdx {
			return fmt.Errorf("Invalid bootstrap reset index (specified %d, reset index: %d)", specifiedIndex, resetIdx)
		}
	}

	// remove the bootstrap override file now that we have the index from it and it was valid.
	// whether bootstrapping works or not is irrelevant as we really don't want this file hanging around
	// in case a snapshot restore is done. In that case we don't want to accidentally allow re-bootstrapping
	// just because the file was unchanged.
	a.removeBootstrapResetFile()

	accessor, err := lib.GenerateUUID(a.srv.checkTokenUUID)
	if err != nil {
		return err
	}
	secret, err := lib.GenerateUUID(a.srv.checkTokenUUID)
	if err != nil {
		return err
	}

	req := structs.ACLTokenBootstrapRequest{
		Token: structs.ACLToken{
			AccessorID:  accessor,
			SecretID:    secret,
			Description: "Bootstrap Token (Global Management)",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: structs.ACLPolicyGlobalManagementID,
				},
			},
			CreateTime: time.Now(),
			Local:      false,
			// DEPRECATED (ACL-Legacy-Compat) - This is used so that the bootstrap token is still visible via the v1 acl APIs
			Type: structs.ACLTokenTypeManagement,
		},
		ResetIndex: specifiedIndex,
	}

	req.Token.SetHash(true)

	resp, err := a.srv.raftApply(structs.ACLBootstrapRequestType, &req)
	if err != nil {
		return err
	}

	if err, ok := resp.(error); ok {
		return err
	}

	if _, token, err := state.ACLTokenGetByAccessor(nil, accessor); err == nil {
		*reply = *token
	}

	a.srv.logger.Printf("[INFO] consul.acl: ACL bootstrap completed")
	return nil
}

func (a *ACL) TokenRead(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	// clients will not know whether the server has local token store. In the case
	// where it doesn't we will transparently forward requests.
	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.ACLDatacenter
	}

	if done, err := a.srv.forward("ACL.TokenRead", args, args, reply); done {
		return err
	}

	var rule acl.Authorizer
	if args.TokenIDType == structs.ACLTokenAccessor {
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

			if args.TokenIDType == structs.ACLTokenAccessor {
				index, token, err = state.ACLTokenGetByAccessor(ws, args.TokenID)
				if token != nil {
					a.srv.filterACLWithAuthorizer(rule, &token)
				}
			} else {
				index, token, err = state.ACLTokenGetBySecret(ws, args.TokenID)
			}

			if err != nil {
				return err
			}

			reply.Index, reply.Token = index, token
			return nil
		})
}

func (a *ACL) TokenClone(args *structs.ACLTokenSetRequest, reply *structs.ACLToken) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	// clients will not know whether the server has local token store. In the case
	// where it doesn't we will transparently forward requests.
	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.ACLDatacenter
	}

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
	} else if !a.srv.InACLDatacenter() && !token.Local {
		// global token writes must be forwarded to the primary DC
		args.Datacenter = a.srv.config.ACLDatacenter
		return a.srv.forwardDC("ACL.TokenClone", a.srv.config.ACLDatacenter, args, reply)
	}

	if token.Rules != "" {
		return fmt.Errorf("Cannot clone a legacy ACL with this endpoint")
	}

	cloneReq := structs.ACLTokenSetRequest{
		Datacenter: args.Datacenter,
		ACLToken: structs.ACLToken{
			Policies:    token.Policies,
			Local:       token.Local,
			Description: token.Description,
		},
		WriteRequest: args.WriteRequest,
	}

	if args.ACLToken.Description != "" {
		cloneReq.ACLToken.Description = args.ACLToken.Description
	}

	return a.tokenSetInternal(&cloneReq, reply, false)
}

func (a *ACL) TokenSet(args *structs.ACLTokenSetRequest, reply *structs.ACLToken) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	// Global token creation/modification always goes to the ACL DC
	if !args.ACLToken.Local {
		args.Datacenter = a.srv.config.ACLDatacenter
	} else if !a.srv.LocalTokensEnabled() {
		return fmt.Errorf("Local tokens are disabled")
	}

	if done, err := a.srv.forward("ACL.TokenSet", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "token", "upsert"}, time.Now())

	// Verify token is permitted to modify ACLs
	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLWrite() {
		return acl.ErrPermissionDenied
	}

	return a.tokenSetInternal(args, reply, false)
}

func (a *ACL) tokenSetInternal(args *structs.ACLTokenSetRequest, reply *structs.ACLToken, upgrade bool) error {
	token := &args.ACLToken

	if !a.srv.LocalTokensEnabled() {
		// local token operations
		return fmt.Errorf("Cannot upsert tokens within this datacenter")
	} else if !a.srv.InACLDatacenter() && !token.Local {
		return fmt.Errorf("Cannot upsert global tokens within this datacenter")
	}

	state := a.srv.fsm.State()

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
		// The RootAuthorizer checks that the SecretID is not "allow", "deny"
		// or "manage" as a precaution against something accidentally using
		// one of these root policies by setting the secret to it.
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
		if token.SecretID == "" {
			token.SecretID = existing.SecretID
		} else if existing.SecretID != token.SecretID {
			return fmt.Errorf("Changing a tokens SecretID is not permitted")
		}

		// Cannot toggle the "Global" mode
		if token.Local != existing.Local {
			return fmt.Errorf("cannot toggle local mode of %s", token.AccessorID)
		}

		if upgrade {
			token.CreateTime = time.Now()
		} else {
			token.CreateTime = existing.CreateTime
		}
	}

	policyIDs := make(map[string]struct{})
	var policies []structs.ACLTokenPolicyLink

	// Validate all the policy names and convert them to policy IDs
	for _, link := range token.Policies {
		if link.ID == "" {
			_, policy, err := state.ACLPolicyGetByName(nil, link.Name)
			if err != nil {
				return fmt.Errorf("Error looking up policy for name %q: %v", link.Name, err)
			}
			if policy == nil {
				return fmt.Errorf("No such ACL policy with name %q", link.Name)
			}
			link.ID = policy.ID
		}

		// Do not store the policy name within raft/memdb as the policy could be renamed in the future.
		link.Name = ""

		// dedup policy links by id
		if _, ok := policyIDs[link.ID]; !ok {
			policies = append(policies, link)
			policyIDs[link.ID] = struct{}{}
		}
	}
	token.Policies = policies

	if token.Rules != "" {
		return fmt.Errorf("Rules cannot be specified for this token")
	}

	if token.Type != "" {
		return fmt.Errorf("Type cannot be specified for this token")
	}

	token.SetHash(true)

	req := &structs.ACLTokenBatchSetRequest{
		Tokens: structs.ACLTokens{token},
		CAS:    false,
	}

	resp, err := a.srv.raftApply(structs.ACLTokenSetRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply token write request: %v", err)
	}

	// Purge the identity from the cache to prevent using the previous definition of the identity
	a.srv.acls.cache.RemoveIdentity(token.SecretID)

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	if _, updatedToken, err := a.srv.fsm.State().ACLTokenGetByAccessor(nil, token.AccessorID); err == nil && token != nil {
		*reply = *updatedToken
	} else {
		return fmt.Errorf("Failed to retrieve the token after insertion")
	}

	return nil
}

func (a *ACL) TokenDelete(args *structs.ACLTokenDeleteRequest, reply *string) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.ACLDatacenter
	}

	if done, err := a.srv.forward("ACL.TokenDelete", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "token", "delete"}, time.Now())

	// Verify token is permitted to modify ACLs
	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLWrite() {
		return acl.ErrPermissionDenied
	}

	if _, err := uuid.ParseUUID(args.TokenID); err != nil {
		return fmt.Errorf("Accessor ID is missing or an invalid UUID")
	}

	if args.TokenID == structs.ACLTokenAnonymousID {
		return fmt.Errorf("Delete operation not permitted on the anonymous token")
	}

	// grab the token here so we can invalidate our cache later on
	_, token, err := a.srv.fsm.State().ACLTokenGetByAccessor(nil, args.TokenID)
	if err != nil {
		return err
	}

	if token != nil {
		if args.Token == token.SecretID {
			return fmt.Errorf("Deletion of the request's authorization token is not permitted")
		}

		if !a.srv.InACLDatacenter() && !token.Local {
			args.Datacenter = a.srv.config.ACLDatacenter
			return a.srv.forwardDC("ACL.TokenDelete", a.srv.config.ACLDatacenter, args, reply)
		}
	}

	req := &structs.ACLTokenBatchDeleteRequest{
		TokenIDs: []string{args.TokenID},
	}

	resp, err := a.srv.raftApply(structs.ACLTokenDeleteRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply token delete request: %v", err)
	}

	// Purge the identity from the cache to prevent using the previous definition of the identity
	if token != nil {
		a.srv.acls.cache.RemoveIdentity(token.SecretID)
	}

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	if reply != nil && token != nil {
		*reply = token.AccessorID
	}

	return nil
}

func (a *ACL) TokenList(args *structs.ACLTokenListRequest, reply *structs.ACLTokenListResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		if args.Datacenter != a.srv.config.ACLDatacenter {
			args.Datacenter = a.srv.config.ACLDatacenter
			args.IncludeLocal = false
			args.IncludeGlobal = true
		}
		args.Datacenter = a.srv.config.ACLDatacenter
	}

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
		func(ws memdb.WatchSet, state *state.Store) error {
			index, tokens, err := state.ACLTokenList(ws, args.IncludeLocal, args.IncludeGlobal, args.Policy)
			if err != nil {
				return err
			}

			stubs := make([]*structs.ACLTokenListStub, 0, len(tokens))
			for _, token := range tokens {
				stubs = append(stubs, token.Stub())
			}
			reply.Index, reply.Tokens = index, stubs
			return nil
		})
}

func (a *ACL) TokenBatchRead(args *structs.ACLTokenBatchGetRequest, reply *structs.ACLTokenBatchResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.ACLDatacenter
	}

	if done, err := a.srv.forward("ACL.TokenBatchRead", args, args, reply); done {
		return err
	}

	rule, err := a.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	} else if rule == nil || !rule.ACLRead() {
		return acl.ErrPermissionDenied
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, tokens, err := state.ACLTokenBatchGet(ws, args.AccessorIDs)
			if err != nil {
				return err
			}

			a.srv.filterACLWithAuthorizer(rule, &tokens)

			reply.Index, reply.Tokens = index, tokens
			return nil
		})
}

func (a *ACL) PolicyRead(args *structs.ACLPolicyGetRequest, reply *structs.ACLPolicyResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if done, err := a.srv.forward("ACL.PolicyRead", args, args, reply); done {
		return err
	}

	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLRead() {
		return acl.ErrPermissionDenied
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, policy, err := state.ACLPolicyGetByID(ws, args.PolicyID)

			if err != nil {
				return err
			}

			reply.Index, reply.Policy = index, policy
			return nil
		})
}

func (a *ACL) PolicyBatchRead(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if done, err := a.srv.forward("ACL.PolicyBatchRead", args, args, reply); done {
		return err
	}

	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLRead() {
		return acl.ErrPermissionDenied
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, policies, err := state.ACLPolicyBatchGet(ws, args.PolicyIDs)
			if err != nil {
				return err
			}

			reply.Index, reply.Policies = index, policies
			return nil
		})
}

func (a *ACL) PolicySet(args *structs.ACLPolicySetRequest, reply *structs.ACLPolicy) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.InACLDatacenter() {
		args.Datacenter = a.srv.config.ACLDatacenter
	}

	if done, err := a.srv.forward("ACL.PolicySet", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "policy", "upsert"}, time.Now())

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
			return fmt.Errorf("Policy ID invalid UUID")
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

	// calculate the hash for this policy
	policy.SetHash(true)

	req := &structs.ACLPolicyBatchSetRequest{
		Policies: structs.ACLPolicies{policy},
	}

	resp, err := a.srv.raftApply(structs.ACLPolicySetRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply policy upsert request: %v", err)
	}

	// Remove from the cache to prevent stale cache usage
	a.srv.acls.cache.RemovePolicy(policy.ID)

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	if _, policy, err := a.srv.fsm.State().ACLPolicyGetByID(nil, policy.ID); err == nil && policy != nil {
		*reply = *policy
	}

	return nil
}

func (a *ACL) PolicyDelete(args *structs.ACLPolicyDeleteRequest, reply *string) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.InACLDatacenter() {
		args.Datacenter = a.srv.config.ACLDatacenter
	}

	if done, err := a.srv.forward("ACL.PolicyDelete", args, args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "policy", "delete"}, time.Now())

	// Verify token is permitted to modify ACLs
	if rule, err := a.srv.ResolveToken(args.Token); err != nil {
		return err
	} else if rule == nil || !rule.ACLWrite() {
		return acl.ErrPermissionDenied
	}

	_, policy, err := a.srv.fsm.State().ACLPolicyGetByID(nil, args.PolicyID)
	if err != nil {
		return err
	}

	if policy == nil {
		return nil
	}

	if policy.ID == structs.ACLPolicyGlobalManagementID {
		return fmt.Errorf("Delete operation not permitted on the builtin global-management policy")
	}

	req := structs.ACLPolicyBatchDeleteRequest{
		PolicyIDs: []string{args.PolicyID},
	}

	resp, err := a.srv.raftApply(structs.ACLPolicyDeleteRequestType, &req)
	if err != nil {
		return fmt.Errorf("Failed to apply policy delete request: %v", err)
	}

	a.srv.acls.cache.RemovePolicy(policy.ID)

	if resp == nil {
		return nil
	}

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	if policy != nil {
		*reply = policy.Name
	}

	return nil
}

func (a *ACL) PolicyList(args *structs.ACLPolicyListRequest, reply *structs.ACLPolicyListResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

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

			var stubs structs.ACLPolicyListStubs
			for _, policy := range policies {
				stubs = append(stubs, policy.Stub())
			}

			reply.Index, reply.Policies = index, stubs
			return nil
		})
}

// PolicyResolve is used to retrieve a subset of the policies associated with a given token
// The policy ids in the args simply act as a filter on the policy set assigned to the token
func (a *ACL) PolicyResolve(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if done, err := a.srv.forward("ACL.PolicyResolve", args, args, reply); done {
		return err
	}

	// get full list of policies for this token
	policies, err := a.srv.acls.resolveTokenToPolicies(args.Token)
	if err != nil {
		return err
	}

	idMap := make(map[string]*structs.ACLPolicy)
	for _, policy := range policies {
		idMap[policy.ID] = policy
	}

	for _, policyID := range args.PolicyIDs {
		if policy, ok := idMap[policyID]; ok {
			reply.Policies = append(reply.Policies, policy)
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
	if done, err := a.srv.forward("ACL.GetPolicy", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if a.srv.config.ACLDatacenter != a.srv.config.Datacenter {
		return acl.ErrDisabled
	}

	// Get the policy via the cache
	parent := a.srv.config.ACLDefaultPolicy

	policy, err := a.srv.acls.GetMergedPolicyForToken(args.ACL)
	if err != nil {
		return err
	}

	// translates the structures internals to most closely match what could be expressed in the original rule language
	policy = policy.ConvertToLegacy()

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

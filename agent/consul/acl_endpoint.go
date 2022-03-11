package consul

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	uuid "github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/template"
)

const (
	// aclBootstrapReset is the file name to create in the data dir. It's only contents
	// should be the reset index
	aclBootstrapReset = "acl-bootstrap-reset"
)

var ACLEndpointSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"acl", "token", "clone"},
		Help: "",
	},
	{
		Name: []string{"acl", "token", "upsert"},
		Help: "",
	},
	{
		Name: []string{"acl", "token", "delete"},
		Help: "",
	},
	{
		Name: []string{"acl", "policy", "upsert"},
		Help: "",
	},
	{
		Name: []string{"acl", "policy", "delete"},
		Help: "",
	},
	{
		Name: []string{"acl", "policy", "delete"},
		Help: "",
	},
	{
		Name: []string{"acl", "role", "upsert"},
		Help: "",
	},
	{
		Name: []string{"acl", "role", "delete"},
		Help: "",
	},
	{
		Name: []string{"acl", "bindingrule", "upsert"},
		Help: "",
	},
	{
		Name: []string{"acl", "bindingrule", "delete"},
		Help: "",
	},
	{
		Name: []string{"acl", "authmethod", "upsert"},
		Help: "",
	},
	{
		Name: []string{"acl", "authmethod", "delete"},
		Help: "",
	},
	{
		Name: []string{"acl", "login"},
		Help: "",
	},
	{
		Name: []string{"acl", "login"},
		Help: "",
	},
	{
		Name: []string{"acl", "logout"},
		Help: "",
	},
	{
		Name: []string{"acl", "logout"},
		Help: "",
	},
}

// Regex for matching
var (
	validPolicyName              = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,128}$`)
	validServiceIdentityName     = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`)
	serviceIdentityNameMaxLength = 256
	validNodeIdentityName        = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`)
	nodeIdentityNameMaxLength    = 256
	validRoleName                = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,256}$`)
	validAuthMethod              = regexp.MustCompile(`^[A-Za-z0-9\-_]{1,128}$`)
)

// ACL endpoint is used to manipulate ACLs
type ACL struct {
	srv    *Server
	logger hclog.Logger
}

// fileBootstrapResetIndex retrieves the reset index specified by the administrator from
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
			a.logger.Error("bootstrap: failed to read path",
				"path", path,
				"error", err,
			)
		}
		return 0
	}

	// Attempt to parse the file
	var resetIdx uint64
	if _, err := fmt.Sscanf(string(raw), "%d", &resetIdx); err != nil {
		a.logger.Error("failed to parse bootstrap reset index path", "path", path, "error", err)
		return 0
	}

	// Return the reset index
	a.logger.Debug("parsed bootstrap reset index path", "path", path, "reset_index", resetIdx)
	return resetIdx
}

func (a *ACL) removeBootstrapResetFile() {
	if err := os.Remove(filepath.Join(a.srv.config.DataDir, aclBootstrapReset)); err != nil {
		a.logger.Warn("failed to remove bootstrap file", "error", err)
	}
}

func (a *ACL) aclPreCheck() error {
	if !a.srv.config.ACLsEnabled {
		return acl.ErrDisabled
	}
	return nil
}

// BootstrapTokens is used to perform a one-time ACL bootstrap operation on
// a cluster to get the first management token.
func (a *ACL) BootstrapTokens(args *structs.DCSpecificRequest, reply *structs.ACLToken) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}
	if done, err := a.srv.ForwardRPC("ACL.BootstrapTokens", args, reply); done {
		return err
	}

	if err := a.srv.aclBootstrapAllowed(); err != nil {
		return err
	}

	// Verify we are allowed to serve this request
	if !a.srv.InPrimaryDatacenter() {
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
			CreateTime:     time.Now(),
			Local:          false,
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		},
		ResetIndex: specifiedIndex,
	}

	req.Token.SetHash(true)

	_, err = a.srv.raftApply(structs.ACLBootstrapRequestType, &req)
	if err != nil {
		return err
	}

	if _, token, err := state.ACLTokenGetByAccessor(nil, accessor, structs.DefaultEnterpriseMetaInDefaultPartition()); err == nil {
		*reply = *token
	}

	a.logger.Info("ACL bootstrap completed")
	return nil
}

func (a *ACL) TokenRead(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	// clients will not know whether the server has local token store. In the case
	// where it doesn't we will transparently forward requests.
	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.TokenRead", args, reply); done {
		return err
	}

	var authz acl.Authorizer

	if args.TokenIDType == structs.ACLTokenAccessor {
		var err error
		var authzContext acl.AuthorizerContext
		// Only ACLRead privileges are required to list tokens
		// However if you do not have ACLWrite as well the token
		// secrets will be redacted
		if authz, err = a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
			return err
		} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
			return err
		}
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var token *structs.ACLToken
			var err error

			if args.TokenIDType == structs.ACLTokenAccessor {
				index, token, err = state.ACLTokenGetByAccessor(ws, args.TokenID, &args.EnterpriseMeta)
				if token != nil {
					a.srv.filterACLWithAuthorizer(authz, &token)

					// token secret was redacted
					if token.SecretID == redactedToken {
						reply.Redacted = true
					}
				}
			} else {
				index, token, err = state.ACLTokenGetBySecret(ws, args.TokenID, nil)
				// no extra validation is needed here. If you have the secret ID you can read it.
			}

			if token != nil && token.IsExpired(time.Now()) {
				token = nil
			}

			if err != nil {
				return err
			}

			reply.Index, reply.Token = index, token
			reply.SourceDatacenter = args.Datacenter
			if token == nil {
				return errNotFound
			}
			return nil
		})
}

func (a *ACL) TokenClone(args *structs.ACLTokenSetRequest, reply *structs.ACLToken) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.ACLToken.EnterpriseMeta, true); err != nil {
		return err
	}

	// clients will not know whether the server has local token store. In the case
	// where it doesn't we will transparently forward requests.
	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.TokenClone", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "token", "clone"}, time.Now())

	var authzContext acl.AuthorizerContext
	authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.ACLToken.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	_, token, err := a.srv.fsm.State().ACLTokenGetByAccessor(nil, args.ACLToken.AccessorID, &args.ACLToken.EnterpriseMeta)
	if err != nil {
		return err
	} else if token == nil || token.IsExpired(time.Now()) {
		return acl.ErrNotFound
	} else if !a.srv.InPrimaryDatacenter() && !token.Local {
		// global token writes must be forwarded to the primary DC
		args.Datacenter = a.srv.config.PrimaryDatacenter
		return a.srv.forwardDC("ACL.TokenClone", a.srv.config.PrimaryDatacenter, args, reply)
	}

	if token.AuthMethod != "" {
		return fmt.Errorf("Cannot clone a token created from an auth method")
	}

	if token.Rules != "" {
		return fmt.Errorf("Cannot clone a legacy ACL with this endpoint")
	}

	cloneReq := structs.ACLTokenSetRequest{
		Datacenter: args.Datacenter,
		ACLToken: structs.ACLToken{
			Policies:          token.Policies,
			Roles:             token.Roles,
			ServiceIdentities: token.ServiceIdentities,
			NodeIdentities:    token.NodeIdentities,
			Local:             token.Local,
			Description:       token.Description,
			ExpirationTime:    token.ExpirationTime,
			EnterpriseMeta:    args.ACLToken.EnterpriseMeta,
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

	if err := a.srv.validateEnterpriseRequest(&args.ACLToken.EnterpriseMeta, true); err != nil {
		return err
	}

	// Global token creation/modification always goes to the ACL DC
	if !args.ACLToken.Local {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	} else if !a.srv.LocalTokensEnabled() {
		return fmt.Errorf("Local tokens are disabled")
	}

	if done, err := a.srv.ForwardRPC("ACL.TokenSet", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "token", "upsert"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext
	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.ACLToken.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	return a.tokenSetInternal(args, reply, false)
}

func (a *ACL) tokenSetInternal(args *structs.ACLTokenSetRequest, reply *structs.ACLToken, fromLogin bool) error {
	token := &args.ACLToken

	if !a.srv.LocalTokensEnabled() {
		// local token operations
		return fmt.Errorf("Cannot upsert tokens within this datacenter")
	} else if !a.srv.InPrimaryDatacenter() && !token.Local {
		return fmt.Errorf("Cannot upsert global tokens within this datacenter")
	}

	state := a.srv.fsm.State()

	var accessorMatch *structs.ACLToken
	var secretMatch *structs.ACLToken
	var err error

	if token.AccessorID != "" {
		_, accessorMatch, err = state.ACLTokenGetByAccessor(nil, token.AccessorID, nil)
		if err != nil {
			return fmt.Errorf("Failed acl token lookup by accessor: %v", err)
		}
	}
	if token.SecretID != "" {
		_, secretMatch, err = state.ACLTokenGetBySecret(nil, token.SecretID, nil)
		if err != nil {
			return fmt.Errorf("Failed acl token lookup by secret: %v", err)
		}
	}

	if token.AccessorID == "" || args.Create {
		// Token Create

		// Generate the AccessorID if not specified
		if token.AccessorID == "" {
			token.AccessorID, err = lib.GenerateUUID(a.srv.checkTokenUUID)
			if err != nil {
				return err
			}
		} else if _, err := uuid.ParseUUID(token.AccessorID); err != nil {
			return fmt.Errorf("Invalid Token: AccessorID is not a valid UUID")
		} else if accessorMatch != nil {
			return fmt.Errorf("Invalid Token: AccessorID is already in use")
		} else if _, match, err := state.ACLTokenGetBySecret(nil, token.AccessorID, nil); err != nil || match != nil {
			if err != nil {
				return fmt.Errorf("Failed to lookup the acl token: %v", err)
			}
			return fmt.Errorf("Invalid Token: AccessorID is already in use")
		} else if structs.ACLIDReserved(token.AccessorID) {
			return fmt.Errorf("Invalid Token: UUIDs with the prefix %q are reserved", structs.ACLReservedPrefix)
		}

		// Generate the SecretID if not specified
		if token.SecretID == "" {
			token.SecretID, err = lib.GenerateUUID(a.srv.checkTokenUUID)
			if err != nil {
				return err
			}
		} else if _, err := uuid.ParseUUID(token.SecretID); err != nil {
			return fmt.Errorf("Invalid Token: SecretID is not a valid UUID")
		} else if secretMatch != nil {
			return fmt.Errorf("Invalid Token: SecretID is already in use")
		} else if _, match, err := state.ACLTokenGetByAccessor(nil, token.SecretID, nil); err != nil || match != nil {
			if err != nil {
				return fmt.Errorf("Failed to lookup the acl token: %v", err)
			}
			return fmt.Errorf("Invalid Token: SecretID is already in use")
		} else if structs.ACLIDReserved(token.SecretID) {
			return fmt.Errorf("Invalid Token: UUIDs with the prefix %q are reserved", structs.ACLReservedPrefix)
		}

		token.CreateTime = time.Now()

		if fromLogin {
			if token.AuthMethod == "" {
				return fmt.Errorf("AuthMethod field is required during Login")
			}
		} else {
			if token.AuthMethod != "" {
				return fmt.Errorf("AuthMethod field is disallowed outside of Login")
			}
		}

		// Ensure an ExpirationTTL is valid if provided.
		if token.ExpirationTTL != 0 {
			if token.ExpirationTTL < 0 {
				return fmt.Errorf("Token Expiration TTL '%s' should be > 0", token.ExpirationTTL)
			}
			if token.HasExpirationTime() {
				return fmt.Errorf("Token Expiration TTL and Expiration Time cannot both be set")
			}

			token.ExpirationTime = timePointer(token.CreateTime.Add(token.ExpirationTTL))
			token.ExpirationTTL = 0
		}

		if token.HasExpirationTime() {
			if token.CreateTime.After(*token.ExpirationTime) {
				return fmt.Errorf("ExpirationTime cannot be before CreateTime")
			}

			expiresIn := token.ExpirationTime.Sub(token.CreateTime)
			if expiresIn > a.srv.config.ACLTokenMaxExpirationTTL {
				return fmt.Errorf("ExpirationTime cannot be more than %s in the future (was %s)",
					a.srv.config.ACLTokenMaxExpirationTTL, expiresIn)
			} else if expiresIn < a.srv.config.ACLTokenMinExpirationTTL {
				return fmt.Errorf("ExpirationTime cannot be less than %s in the future (was %s)",
					a.srv.config.ACLTokenMinExpirationTTL, expiresIn)
			}
		}
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
		if accessorMatch == nil || accessorMatch.IsExpired(time.Now()) {
			return fmt.Errorf("Cannot find token %q", token.AccessorID)
		}
		if token.SecretID == "" {
			token.SecretID = accessorMatch.SecretID
		} else if accessorMatch.SecretID != token.SecretID {
			return fmt.Errorf("Changing a tokens SecretID is not permitted")
		}

		// Cannot toggle the "Global" mode
		if token.Local != accessorMatch.Local {
			return fmt.Errorf("cannot toggle local mode of %s", token.AccessorID)
		}

		if token.AuthMethod == "" {
			token.AuthMethod = accessorMatch.AuthMethod
		} else if token.AuthMethod != accessorMatch.AuthMethod {
			return fmt.Errorf("Cannot change AuthMethod of %s", token.AccessorID)
		}

		if token.ExpirationTTL != 0 {
			return fmt.Errorf("Cannot change expiration time of %s", token.AccessorID)
		}

		if !token.HasExpirationTime() {
			token.ExpirationTime = accessorMatch.ExpirationTime
		} else if !accessorMatch.HasExpirationTime() {
			return fmt.Errorf("Cannot change expiration time of %s", token.AccessorID)
		} else if !token.ExpirationTime.Equal(*accessorMatch.ExpirationTime) {
			return fmt.Errorf("Cannot change expiration time of %s", token.AccessorID)
		}

		token.CreateTime = accessorMatch.CreateTime
	}

	policyIDs := make(map[string]struct{})
	var policies []structs.ACLTokenPolicyLink

	// Validate all the policy names and convert them to policy IDs
	for _, link := range token.Policies {
		if link.ID == "" {
			_, policy, err := state.ACLPolicyGetByName(nil, link.Name, &token.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("Error looking up policy for name %q: %v", link.Name, err)
			}
			if policy == nil {
				return fmt.Errorf("No such ACL policy with name %q", link.Name)
			}
			link.ID = policy.ID
		} else {
			_, policy, err := state.ACLPolicyGetByID(nil, link.ID, &token.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("Error looking up policy for id %q: %v", link.ID, err)
			}

			if policy == nil {
				return fmt.Errorf("No such ACL policy with ID %q", link.ID)
			}
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

	roleIDs := make(map[string]struct{})
	var roles []structs.ACLTokenRoleLink

	// Validate all the role names and convert them to role IDs.
	for _, link := range token.Roles {
		if link.ID == "" {
			_, role, err := state.ACLRoleGetByName(nil, link.Name, &token.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("Error looking up role for name %q: %v", link.Name, err)
			}
			if role == nil {
				return fmt.Errorf("No such ACL role with name %q", link.Name)
			}
			link.ID = role.ID
		} else {
			_, role, err := state.ACLRoleGetByID(nil, link.ID, &token.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("Error looking up role for id %q: %v", link.ID, err)
			}

			if role == nil {
				return fmt.Errorf("No such ACL role with ID %q", link.ID)
			}
		}

		// Do not store the role name within raft/memdb as the role could be renamed in the future.
		link.Name = ""

		// dedup role links by id
		if _, ok := roleIDs[link.ID]; !ok {
			roles = append(roles, link)
			roleIDs[link.ID] = struct{}{}
		}
	}
	token.Roles = roles

	for _, svcid := range token.ServiceIdentities {
		if svcid.ServiceName == "" {
			return fmt.Errorf("Service identity is missing the service name field on this token")
		}
		if token.Local && len(svcid.Datacenters) > 0 {
			return fmt.Errorf("Service identity %q cannot specify a list of datacenters on a local token", svcid.ServiceName)
		}
		if !isValidServiceIdentityName(svcid.ServiceName) {
			return fmt.Errorf("Service identity %q has an invalid name. Only alphanumeric characters, '-' and '_' are allowed", svcid.ServiceName)
		}
	}
	token.ServiceIdentities = dedupeServiceIdentities(token.ServiceIdentities)

	for _, nodeid := range token.NodeIdentities {
		if nodeid.NodeName == "" {
			return fmt.Errorf("Node identity is missing the node name field on this token")
		}
		if nodeid.Datacenter == "" {
			return fmt.Errorf("Node identity is missing the datacenter field on this token")
		}
		if !isValidNodeIdentityName(nodeid.NodeName) {
			return fmt.Errorf("Node identity has an invalid name. Only alphanumeric characters, '-' and '_' are allowed")
		}
	}
	token.NodeIdentities = dedupeNodeIdentities(token.NodeIdentities)

	if token.Rules != "" {
		return fmt.Errorf("Rules cannot be specified for this token")
	}

	if token.Type != "" {
		return fmt.Errorf("Type cannot be specified for this token")
	}

	token.SetHash(true)

	// validate the enterprise specific fields
	if err = a.tokenUpsertValidateEnterprise(token, accessorMatch); err != nil {
		return err
	}

	req := &structs.ACLTokenBatchSetRequest{
		Tokens: structs.ACLTokens{token},
		CAS:    false,
	}

	if fromLogin {
		// Logins may attempt to link to roles that do not exist. These
		// may be persisted, but don't allow tokens to be created that
		// have no privileges (i.e. role links that point nowhere).
		req.AllowMissingLinks = true
		req.ProhibitUnprivileged = true
	}

	_, err = a.srv.raftApply(structs.ACLTokenSetRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply token write request: %v", err)
	}

	// Purge the identity from the cache to prevent using the previous definition of the identity
	a.srv.ACLResolver.cache.RemoveIdentity(tokenSecretCacheID(token.SecretID))

	// Don't check expiration times here as it doesn't really matter.
	if _, updatedToken, err := a.srv.fsm.State().ACLTokenGetByAccessor(nil, token.AccessorID, nil); err == nil && updatedToken != nil {
		*reply = *updatedToken
	} else {
		return fmt.Errorf("Failed to retrieve the token after insertion")
	}

	return nil
}

func validateBindingRuleBindName(bindType, bindName string, availableFields []string) (bool, error) {
	if bindType == "" || bindName == "" {
		return false, nil
	}

	fakeVarMap := make(map[string]string)
	for _, v := range availableFields {
		fakeVarMap[v] = "fake"
	}

	_, valid, err := computeBindingRuleBindName(bindType, bindName, fakeVarMap)
	if err != nil {
		return false, err
	}
	return valid, nil
}

// computeBindingRuleBindName processes the HIL for the provided bind type+name
// using the projected variables.
//
// - If the HIL is invalid ("", false, AN_ERROR) is returned.
// - If the computed name is not valid for the type ("INVALID_NAME", false, nil) is returned.
// - If the computed name is valid for the type ("VALID_NAME", true, nil) is returned.
func computeBindingRuleBindName(bindType, bindName string, projectedVars map[string]string) (string, bool, error) {
	bindName, err := template.InterpolateHIL(bindName, projectedVars, true)
	if err != nil {
		return "", false, err
	}

	valid := false

	switch bindType {
	case structs.BindingRuleBindTypeService:
		valid = isValidServiceIdentityName(bindName)
	case structs.BindingRuleBindTypeNode:
		valid = isValidNodeIdentityName(bindName)
	case structs.BindingRuleBindTypeRole:
		valid = validRoleName.MatchString(bindName)

	default:
		return "", false, fmt.Errorf("unknown binding rule bind type: %s", bindType)
	}

	return bindName, valid, nil
}

// isValidServiceIdentityName returns true if the provided name can be used as
// an ACLServiceIdentity ServiceName. This is more restrictive than standard
// catalog registration, which basically takes the view that "everything is
// valid".
func isValidServiceIdentityName(name string) bool {
	if len(name) < 1 || len(name) > serviceIdentityNameMaxLength {
		return false
	}
	return validServiceIdentityName.MatchString(name)
}

// isValidNodeIdentityName returns true if the provided name can be used as
// an ACLNodeIdentity NodeName. This is more restrictive than standard
// catalog registration, which basically takes the view that "everything is
// valid".
func isValidNodeIdentityName(name string) bool {
	if len(name) < 1 || len(name) > nodeIdentityNameMaxLength {
		return false
	}
	return validNodeIdentityName.MatchString(name)
}

func (a *ACL) TokenDelete(args *structs.ACLTokenDeleteRequest, reply *string) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.TokenDelete", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "token", "delete"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext
	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	if _, err := uuid.ParseUUID(args.TokenID); err != nil {
		return fmt.Errorf("Accessor ID is missing or an invalid UUID")
	}

	if args.TokenID == structs.ACLTokenAnonymousID {
		return fmt.Errorf("Delete operation not permitted on the anonymous token")
	}

	// grab the token here so we can invalidate our cache later on
	_, token, err := a.srv.fsm.State().ACLTokenGetByAccessor(nil, args.TokenID, &args.EnterpriseMeta)
	if err != nil {
		return err
	}

	if token != nil {
		if args.Token == token.SecretID {
			return fmt.Errorf("Deletion of the request's authorization token is not permitted")
		}

		// No need to check expiration time because it's being deleted.

		// token found in secondary DC but its not local so it must be deleted in the primary
		if !a.srv.InPrimaryDatacenter() && !token.Local {
			args.Datacenter = a.srv.config.PrimaryDatacenter
			return a.srv.forwardDC("ACL.TokenDelete", a.srv.config.PrimaryDatacenter, args, reply)
		}
	} else if !a.srv.InPrimaryDatacenter() {
		// token not found in secondary DC - attempt to delete within the primary
		args.Datacenter = a.srv.config.PrimaryDatacenter
		return a.srv.forwardDC("ACL.TokenDelete", a.srv.config.PrimaryDatacenter, args, reply)
	} else {
		// in Primary Datacenter but the token does not exist - return early as there is nothing to do.
		return nil
	}

	req := &structs.ACLTokenBatchDeleteRequest{
		TokenIDs: []string{args.TokenID},
	}

	_, err = a.srv.raftApply(structs.ACLTokenDeleteRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply token delete request: %v", err)
	}

	// Purge the identity from the cache to prevent using the previous definition of the identity
	a.srv.ACLResolver.cache.RemoveIdentity(tokenSecretCacheID(token.SecretID))

	if reply != nil {
		*reply = token.AccessorID
	}

	return nil
}

func (a *ACL) TokenList(args *structs.ACLTokenListRequest, reply *structs.ACLTokenListResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		if args.Datacenter != a.srv.config.PrimaryDatacenter {
			args.Datacenter = a.srv.config.PrimaryDatacenter
			args.IncludeLocal = false
			args.IncludeGlobal = true
		}
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.TokenList", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext
	var requestMeta structs.EnterpriseMeta
	authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &requestMeta, &authzContext)
	if err != nil {
		return err
	}
	// merge the token default meta into the requests meta
	args.EnterpriseMeta.Merge(&requestMeta)
	args.EnterpriseMeta.FillAuthzContext(&authzContext)
	if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	var methodMeta *structs.EnterpriseMeta
	if args.AuthMethod != "" {
		methodMeta = args.ACLAuthMethodEnterpriseMeta.ToEnterpriseMeta()
		// attempt to merge in the overall meta, wildcards will not be merged
		methodMeta.MergeNoWildcard(&args.EnterpriseMeta)
		// in the event that the meta above didn't merge due to being a wildcard
		// we ensure that proper token based meta inference occurs
		methodMeta.Merge(&requestMeta)
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, tokens, err := state.ACLTokenList(ws, args.IncludeLocal, args.IncludeGlobal, args.Policy, args.Role, args.AuthMethod, methodMeta, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			now := time.Now()

			stubs := make([]*structs.ACLTokenListStub, 0, len(tokens))
			for _, token := range tokens {
				if token.IsExpired(now) {
					continue
				}
				stubs = append(stubs, token.Stub())
			}

			// filter down to just the tokens that the requester has permissions to read
			a.srv.filterACLWithAuthorizer(authz, &stubs)

			reply.Index, reply.Tokens = index, stubs
			return nil
		})
}

func (a *ACL) TokenBatchRead(args *structs.ACLTokenBatchGetRequest, reply *structs.ACLTokenBatchResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.TokenBatchRead", args, reply); done {
		return err
	}

	authz, err := a.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, tokens, err := state.ACLTokenBatchGet(ws, args.AccessorIDs)
			if err != nil {
				return err
			}

			// This RPC is used for replication, so don't filter out expired tokens here.

			// Filter the tokens down to just what we have permission to see - also redact
			// secrets based on allowed permissions. We could just call filterACLWithAuthorizer
			// on the whole token list but then it would require another pass through the token
			// list to determine if any secrets were redacted. Its a small amount of code to
			// process the loop so it was duplicated here and we instead call the filter func
			// with just a single token.
			ret := make(structs.ACLTokens, 0, len(tokens))
			for _, token := range tokens {
				final := token
				a.srv.filterACLWithAuthorizer(authz, &final)
				if final != nil {
					ret = append(ret, final)
					if final.SecretID == redactedToken {
						reply.Redacted = true
					}
				} else {
					reply.Removed = true
				}
			}

			reply.Index, reply.Tokens = index, ret
			return nil
		})
}

func (a *ACL) PolicyRead(args *structs.ACLPolicyGetRequest, reply *structs.ACLPolicyResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.PolicyRead", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext
	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var (
				index  uint64
				policy *structs.ACLPolicy
				err    error
			)
			if args.PolicyID != "" {
				index, policy, err = state.ACLPolicyGetByID(ws, args.PolicyID, &args.EnterpriseMeta)
			} else {
				index, policy, err = state.ACLPolicyGetByName(ws, args.PolicyName, &args.EnterpriseMeta)
			}

			if err != nil {
				return err
			}

			reply.Index, reply.Policy = index, policy
			if policy == nil {
				return errNotFound
			}
			return nil
		})
}

func (a *ACL) PolicyBatchRead(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.PolicyBatchRead", args, reply); done {
		return err
	}

	authz, err := a.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, policies, err := state.ACLPolicyBatchGet(ws, args.PolicyIDs)
			if err != nil {
				return err
			}

			a.srv.filterACLWithAuthorizer(authz, &policies)

			reply.Index, reply.Policies = index, policies
			return nil
		})
}

func (a *ACL) PolicySet(args *structs.ACLPolicySetRequest, reply *structs.ACLPolicy) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.Policy.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.InPrimaryDatacenter() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.PolicySet", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "policy", "upsert"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.Policy.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
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

	var idMatch *structs.ACLPolicy
	var nameMatch *structs.ACLPolicy
	var err error

	if policy.ID != "" {
		if _, err := uuid.ParseUUID(policy.ID); err != nil {
			return fmt.Errorf("Policy ID invalid UUID")
		}

		_, idMatch, err = state.ACLPolicyGetByID(nil, policy.ID, nil)
		if err != nil {
			return fmt.Errorf("acl policy lookup by id failed: %v", err)
		}
	}
	_, nameMatch, err = state.ACLPolicyGetByName(nil, policy.Name, &policy.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("acl policy lookup by name failed: %v", err)
	}

	if policy.ID == "" {
		// with no policy ID one will be generated
		var err error
		policy.ID, err = lib.GenerateUUID(a.srv.checkPolicyUUID)
		if err != nil {
			return err
		}

		// validate the name is unique
		if nameMatch != nil {
			return fmt.Errorf("Invalid Policy: A Policy with Name %q already exists", policy.Name)
		}
	} else {
		// Verify the policy exists
		if idMatch == nil {
			return fmt.Errorf("cannot find policy %s", policy.ID)
		}

		// Verify that the name isn't changing or that the name is not already used
		if idMatch.Name != policy.Name && nameMatch != nil {
			return fmt.Errorf("Invalid Policy: A policy with name %q already exists", policy.Name)
		}

		if policy.ID == structs.ACLPolicyGlobalManagementID {
			if policy.Datacenters != nil || len(policy.Datacenters) > 0 {
				return fmt.Errorf("Changing the Datacenters of the builtin global-management policy is not permitted")
			}

			if policy.Rules != idMatch.Rules {
				return fmt.Errorf("Changing the Rules for the builtin global-management policy is not permitted")
			}
		}
	}

	// validate the rules
	_, err = acl.NewPolicyFromSource(policy.Rules, policy.Syntax, a.srv.aclConfig, policy.EnterprisePolicyMeta())
	if err != nil {
		return err
	}

	// validate the enterprise specific fields
	if err = a.policyUpsertValidateEnterprise(policy, idMatch); err != nil {
		return err
	}

	// calculate the hash for this policy
	policy.SetHash(true)

	req := &structs.ACLPolicyBatchSetRequest{
		Policies: structs.ACLPolicies{policy},
	}

	_, err = a.srv.raftApply(structs.ACLPolicySetRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply policy upsert request: %v", err)
	}

	// Remove from the cache to prevent stale cache usage
	a.srv.ACLResolver.cache.RemovePolicy(policy.ID)

	if _, policy, err := a.srv.fsm.State().ACLPolicyGetByID(nil, policy.ID, &policy.EnterpriseMeta); err == nil && policy != nil {
		*reply = *policy
	}

	return nil
}

func (a *ACL) PolicyDelete(args *structs.ACLPolicyDeleteRequest, reply *string) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.InPrimaryDatacenter() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.PolicyDelete", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "policy", "delete"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	_, policy, err := a.srv.fsm.State().ACLPolicyGetByID(nil, args.PolicyID, &args.EnterpriseMeta)
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

	_, err = a.srv.raftApply(structs.ACLPolicyDeleteRequestType, &req)
	if err != nil {
		return fmt.Errorf("Failed to apply policy delete request: %v", err)
	}

	a.srv.ACLResolver.cache.RemovePolicy(policy.ID)

	*reply = policy.Name

	return nil
}

func (a *ACL) PolicyList(args *structs.ACLPolicyListRequest, reply *structs.ACLPolicyListResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.PolicyList", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext

	authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, policies, err := state.ACLPolicyList(ws, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			// filter down to just what the requester has permissions to see
			a.srv.filterACLWithAuthorizer(authz, &policies)

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

	if done, err := a.srv.ForwardRPC("ACL.PolicyResolve", args, reply); done {
		return err
	}

	// get full list of policies for this token
	identity, policies, err := a.srv.ACLResolver.resolveTokenToIdentityAndPolicies(args.Token)
	if err != nil {
		return err
	}

	entIdentity, entPolicies, err := a.srv.ACLResolver.resolveEnterpriseIdentityAndPolicies(identity)
	if err != nil {
		return err
	}

	idMap := make(map[string]*structs.ACLPolicy)
	for _, policyID := range identity.PolicyIDs() {
		idMap[policyID] = nil
	}
	if entIdentity != nil {
		for _, policyID := range entIdentity.PolicyIDs() {
			idMap[policyID] = nil
		}
	}

	for _, policy := range policies {
		idMap[policy.ID] = policy
	}
	for _, policy := range entPolicies {
		idMap[policy.ID] = policy
	}

	for _, policyID := range args.PolicyIDs {
		if policy, ok := idMap[policyID]; ok {
			// only add non-deleted policies
			if policy != nil {
				reply.Policies = append(reply.Policies, policy)
			}
		} else {
			// send a permission denied to indicate that the request included
			// policy ids not associated with this token
			return acl.ErrPermissionDenied
		}
	}

	a.srv.setQueryMeta(&reply.QueryMeta, args.Token)

	return nil
}

// ReplicationStatus is used to retrieve the current ACL replication status.
func (a *ACL) ReplicationStatus(args *structs.DCSpecificRequest,
	reply *structs.ACLReplicationStatus) error {
	// This must be sent to the leader, so we fix the args since we are
	// re-using a structure where we don't support all the options.
	args.RequireConsistent = true
	args.AllowStale = false
	if done, err := a.srv.ForwardRPC("ACL.ReplicationStatus", args, reply); done {
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

func timePointer(t time.Time) *time.Time {
	return &t
}

func (a *ACL) RoleRead(args *structs.ACLRoleGetRequest, reply *structs.ACLRoleResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.RoleRead", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var (
				index uint64
				role  *structs.ACLRole
				err   error
			)
			if args.RoleID != "" {
				index, role, err = state.ACLRoleGetByID(ws, args.RoleID, &args.EnterpriseMeta)
			} else {
				index, role, err = state.ACLRoleGetByName(ws, args.RoleName, &args.EnterpriseMeta)
			}

			if err != nil {
				return err
			}

			reply.Index, reply.Role = index, role
			if role == nil {
				return errNotFound
			}
			return nil
		})
}

func (a *ACL) RoleBatchRead(args *structs.ACLRoleBatchGetRequest, reply *structs.ACLRoleBatchResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.RoleBatchRead", args, reply); done {
		return err
	}

	authz, err := a.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, roles, err := state.ACLRoleBatchGet(ws, args.RoleIDs)
			if err != nil {
				return err
			}

			a.srv.filterACLWithAuthorizer(authz, &roles)

			reply.Index, reply.Roles = index, roles
			return nil
		})
}

func (a *ACL) RoleSet(args *structs.ACLRoleSetRequest, reply *structs.ACLRole) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.Role.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.InPrimaryDatacenter() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.RoleSet", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "role", "upsert"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.Role.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	role := &args.Role
	state := a.srv.fsm.State()

	// Almost all of the checks here are also done in the state store. However,
	// we want to prevent the raft operations when we know they are going to fail
	// so we still do them here.

	// ensure a name is set
	if role.Name == "" {
		return fmt.Errorf("Invalid Role: no Name is set")
	}

	if !validRoleName.MatchString(role.Name) {
		return fmt.Errorf("Invalid Role: invalid Name. Only alphanumeric characters, '-' and '_' are allowed")
	}

	var existing *structs.ACLRole
	var err error
	if role.ID == "" {
		// with no role ID one will be generated
		role.ID, err = lib.GenerateUUID(a.srv.checkRoleUUID)
		if err != nil {
			return err
		}

		// validate the name is unique
		if _, existing, err = state.ACLRoleGetByName(nil, role.Name, &role.EnterpriseMeta); err != nil {
			return fmt.Errorf("acl role lookup by name failed: %v", err)
		} else if existing != nil {
			return fmt.Errorf("Invalid Role: A Role with Name %q already exists", role.Name)
		}
	} else {
		if _, err := uuid.ParseUUID(role.ID); err != nil {
			return fmt.Errorf("Role ID invalid UUID")
		}

		// Verify the role exists
		_, existing, err = state.ACLRoleGetByID(nil, role.ID, nil)
		if err != nil {
			return fmt.Errorf("acl role lookup failed: %v", err)
		} else if existing == nil {
			return fmt.Errorf("cannot find role %s", role.ID)
		}

		if existing.Name != role.Name {
			if _, nameMatch, err := state.ACLRoleGetByName(nil, role.Name, &role.EnterpriseMeta); err != nil {
				return fmt.Errorf("acl role lookup by name failed: %v", err)
			} else if nameMatch != nil {
				return fmt.Errorf("Invalid Role: A role with name %q already exists", role.Name)
			}
		}
	}

	// validate the enterprise specific fields
	if err := a.roleUpsertValidateEnterprise(role, existing); err != nil {
		return err
	}

	policyIDs := make(map[string]struct{})
	var policies []structs.ACLRolePolicyLink

	// Validate all the policy names and convert them to policy IDs
	for _, link := range role.Policies {
		if link.ID == "" {
			_, policy, err := state.ACLPolicyGetByName(nil, link.Name, &role.EnterpriseMeta)
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
	role.Policies = policies

	for _, svcid := range role.ServiceIdentities {
		if svcid.ServiceName == "" {
			return fmt.Errorf("Service identity is missing the service name field on this role")
		}
		if !isValidServiceIdentityName(svcid.ServiceName) {
			return fmt.Errorf("Service identity %q has an invalid name. Only alphanumeric characters, '-' and '_' are allowed", svcid.ServiceName)
		}
	}
	role.ServiceIdentities = dedupeServiceIdentities(role.ServiceIdentities)

	for _, nodeid := range role.NodeIdentities {
		if nodeid.NodeName == "" {
			return fmt.Errorf("Node identity is missing the node name field on this role")
		}
		if nodeid.Datacenter == "" {
			return fmt.Errorf("Node identity is missing the datacenter field on this role")
		}
		if !isValidNodeIdentityName(nodeid.NodeName) {
			return fmt.Errorf("Node identity has an invalid name. Only alphanumeric characters, '-' and '_' are allowed")
		}
	}
	role.NodeIdentities = dedupeNodeIdentities(role.NodeIdentities)

	// calculate the hash for this role
	role.SetHash(true)

	req := &structs.ACLRoleBatchSetRequest{
		Roles: structs.ACLRoles{role},
	}

	_, err = a.srv.raftApply(structs.ACLRoleSetRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply role upsert request: %v", err)
	}

	// Remove from the cache to prevent stale cache usage
	a.srv.ACLResolver.cache.RemoveRole(role.ID)

	if _, role, err := a.srv.fsm.State().ACLRoleGetByID(nil, role.ID, &role.EnterpriseMeta); err == nil && role != nil {
		*reply = *role
	}

	return nil
}

func (a *ACL) RoleDelete(args *structs.ACLRoleDeleteRequest, reply *string) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.InPrimaryDatacenter() {
		args.Datacenter = a.srv.config.PrimaryDatacenter
	}

	if done, err := a.srv.ForwardRPC("ACL.RoleDelete", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "role", "delete"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	_, role, err := a.srv.fsm.State().ACLRoleGetByID(nil, args.RoleID, &args.EnterpriseMeta)
	if err != nil {
		return err
	}

	if role == nil {
		return nil
	}

	req := structs.ACLRoleBatchDeleteRequest{
		RoleIDs: []string{args.RoleID},
	}

	_, err = a.srv.raftApply(structs.ACLRoleDeleteRequestType, &req)
	if err != nil {
		return fmt.Errorf("Failed to apply role delete request: %v", err)
	}

	a.srv.ACLResolver.cache.RemoveRole(role.ID)

	*reply = role.Name

	return nil
}

func (a *ACL) RoleList(args *structs.ACLRoleListRequest, reply *structs.ACLRoleListResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.RoleList", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext

	authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, roles, err := state.ACLRoleList(ws, args.Policy, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			a.srv.filterACLWithAuthorizer(authz, &roles)

			reply.Index, reply.Roles = index, roles
			return nil
		})
}

// RoleResolve is used to retrieve a subset of the roles associated with a given token
// The role ids in the args simply act as a filter on the role set assigned to the token
func (a *ACL) RoleResolve(args *structs.ACLRoleBatchGetRequest, reply *structs.ACLRoleBatchResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.RoleResolve", args, reply); done {
		return err
	}

	// get full list of roles for this token
	identity, roles, err := a.srv.ACLResolver.resolveTokenToIdentityAndRoles(args.Token)
	if err != nil {
		return err
	}

	entIdentity, entRoles, err := a.srv.ACLResolver.resolveEnterpriseIdentityAndRoles(identity)
	if err != nil {
		return err
	}

	idMap := make(map[string]*structs.ACLRole)
	for _, roleID := range identity.RoleIDs() {
		idMap[roleID] = nil
	}
	if entIdentity != nil {
		for _, roleID := range entIdentity.RoleIDs() {
			idMap[roleID] = nil
		}
	}

	for _, role := range roles {
		idMap[role.ID] = role
	}
	for _, role := range entRoles {
		idMap[role.ID] = role
	}

	for _, roleID := range args.RoleIDs {
		if role, ok := idMap[roleID]; ok {
			// only add non-deleted roles
			if role != nil {
				reply.Roles = append(reply.Roles, role)
			}
		} else {
			// send a permission denied to indicate that the request included
			// role ids not associated with this token
			return acl.ErrPermissionDenied
		}
	}

	a.srv.setQueryMeta(&reply.QueryMeta, args.Token)

	return nil
}

var errAuthMethodsRequireTokenReplication = errors.New("Token replication is required for auth methods to function")

func (a *ACL) BindingRuleRead(args *structs.ACLBindingRuleGetRequest, reply *structs.ACLBindingRuleResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.BindingRuleRead", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext

	authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, rule, err := state.ACLBindingRuleGetByID(ws, args.BindingRuleID, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.BindingRule = index, rule
			if rule == nil {
				return errNotFound
			}
			return nil
		})
}

func (a *ACL) BindingRuleSet(args *structs.ACLBindingRuleSetRequest, reply *structs.ACLBindingRule) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.BindingRule.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.BindingRuleSet", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "bindingrule", "upsert"}, time.Now())

	var authzContext acl.AuthorizerContext

	// Verify token is permitted to modify ACLs
	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.BindingRule.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	var existing *structs.ACLBindingRule
	rule := &args.BindingRule
	state := a.srv.fsm.State()

	if rule.ID == "" {
		// with no binding rule ID one will be generated
		var err error

		rule.ID, err = lib.GenerateUUID(a.srv.checkBindingRuleUUID)
		if err != nil {
			return err
		}
	} else {
		if _, err := uuid.ParseUUID(rule.ID); err != nil {
			return fmt.Errorf("Binding Rule ID invalid UUID")
		}

		// Verify the role exists
		var err error
		// specifically disregarding the enterprise meta here
		_, existing, err = state.ACLBindingRuleGetByID(nil, rule.ID, nil)
		if err != nil {
			return fmt.Errorf("acl binding rule lookup failed: %v", err)
		} else if existing == nil {
			return fmt.Errorf("cannot find binding rule %s", rule.ID)
		}

		if rule.AuthMethod == "" {
			rule.AuthMethod = existing.AuthMethod
		} else if existing.AuthMethod != rule.AuthMethod {
			return fmt.Errorf("the AuthMethod field of an Binding Rule is immutable")
		}
	}

	if rule.AuthMethod == "" {
		return fmt.Errorf("Invalid Binding Rule: no AuthMethod is set")
	}

	// this is done early here to produce better errors
	if err := state.ACLBindingRuleUpsertValidateEnterprise(rule, existing); err != nil {
		return err
	}

	methodIdx, method, err := state.ACLAuthMethodGetByName(nil, rule.AuthMethod, &args.BindingRule.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("acl auth method lookup failed: %v", err)
	} else if method == nil {
		return fmt.Errorf("cannot find auth method with name %q", rule.AuthMethod)
	}
	validator, err := a.srv.loadAuthMethodValidator(methodIdx, method)
	if err != nil {
		return err
	}

	// Create a blank placeholder identity for use in validation below.
	blankID := validator.NewIdentity()

	if rule.Selector != "" {
		if _, err := bexpr.CreateEvaluatorForType(rule.Selector, nil, blankID.SelectableFields); err != nil {
			return fmt.Errorf("invalid Binding Rule: Selector is invalid: %v", err)
		}
	}

	if rule.BindType == "" {
		return fmt.Errorf("Invalid Binding Rule: no BindType is set")
	}

	if rule.BindName == "" {
		return fmt.Errorf("Invalid Binding Rule: no BindName is set")
	}

	switch rule.BindType {
	case structs.BindingRuleBindTypeService:
	case structs.BindingRuleBindTypeNode:
	case structs.BindingRuleBindTypeRole:
	default:
		return fmt.Errorf("Invalid Binding Rule: unknown BindType %q", rule.BindType)
	}

	if valid, err := validateBindingRuleBindName(rule.BindType, rule.BindName, blankID.ProjectedVarNames()); err != nil {
		return fmt.Errorf("Invalid Binding Rule: invalid BindName: %v", err)
	} else if !valid {
		return fmt.Errorf("Invalid Binding Rule: invalid BindName")
	}

	req := &structs.ACLBindingRuleBatchSetRequest{
		BindingRules: structs.ACLBindingRules{rule},
	}

	_, err = a.srv.raftApply(structs.ACLBindingRuleSetRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply binding rule upsert request: %v", err)
	}

	if _, rule, err := a.srv.fsm.State().ACLBindingRuleGetByID(nil, rule.ID, &rule.EnterpriseMeta); err == nil && rule != nil {
		*reply = *rule
	}

	return nil
}

func (a *ACL) BindingRuleDelete(args *structs.ACLBindingRuleDeleteRequest, reply *bool) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.BindingRuleDelete", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "bindingrule", "delete"}, time.Now())

	var authzContext acl.AuthorizerContext

	// Verify token is permitted to modify ACLs
	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	_, rule, err := a.srv.fsm.State().ACLBindingRuleGetByID(nil, args.BindingRuleID, &args.EnterpriseMeta)
	switch {
	case err != nil:
		return err
	case rule == nil:
		return nil
	}

	req := structs.ACLBindingRuleBatchDeleteRequest{
		BindingRuleIDs: []string{args.BindingRuleID},
	}

	_, err = a.srv.raftApply(structs.ACLBindingRuleDeleteRequestType, &req)
	if err != nil {
		return fmt.Errorf("Failed to apply binding rule delete request: %v", err)
	}

	*reply = true

	return nil
}

func (a *ACL) BindingRuleList(args *structs.ACLBindingRuleListRequest, reply *structs.ACLBindingRuleListResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.BindingRuleList", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext

	authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, rules, err := state.ACLBindingRuleList(ws, args.AuthMethod, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			a.srv.filterACLWithAuthorizer(authz, &rules)

			reply.Index, reply.BindingRules = index, rules
			return nil
		})
}

func (a *ACL) AuthMethodRead(args *structs.ACLAuthMethodGetRequest, reply *structs.ACLAuthMethodResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.AuthMethodRead", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, method, err := state.ACLAuthMethodGetByName(ws, args.AuthMethodName, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.AuthMethod = index, method
			if method == nil {
				return errNotFound
			}

			_ = a.enterpriseAuthMethodTypeValidation(method.Type)
			return nil
		})
}

func (a *ACL) AuthMethodSet(args *structs.ACLAuthMethodSetRequest, reply *structs.ACLAuthMethod) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.AuthMethod.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.AuthMethodSet", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "authmethod", "upsert"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.AuthMethod.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	method := &args.AuthMethod
	state := a.srv.fsm.State()

	// ensure a name is set
	if method.Name == "" {
		return fmt.Errorf("Invalid Auth Method: no Name is set")
	}
	if !validAuthMethod.MatchString(method.Name) {
		return fmt.Errorf("Invalid Auth Method: invalid Name. Only alphanumeric characters, '-' and '_' are allowed")
	}

	if err := a.enterpriseAuthMethodTypeValidation(method.Type); err != nil {
		return err
	}

	// Check to see if the method exists first.
	_, existing, err := state.ACLAuthMethodGetByName(nil, method.Name, &method.EnterpriseMeta)
	if err != nil {
		return fmt.Errorf("acl auth method lookup failed: %v", err)
	}

	if existing != nil {
		if method.Type == "" {
			method.Type = existing.Type
		} else if existing.Type != method.Type {
			return fmt.Errorf("the Type field of an Auth Method is immutable")
		}
	}

	if !authmethod.IsRegisteredType(method.Type) {
		return fmt.Errorf("Invalid Auth Method: Type should be one of: %v", authmethod.Types())
	}

	if method.MaxTokenTTL != 0 {
		if method.MaxTokenTTL > a.srv.config.ACLTokenMaxExpirationTTL {
			return fmt.Errorf("MaxTokenTTL %s cannot be more than %s",
				method.MaxTokenTTL, a.srv.config.ACLTokenMaxExpirationTTL)
		} else if method.MaxTokenTTL < a.srv.config.ACLTokenMinExpirationTTL {
			return fmt.Errorf("MaxTokenTTL %s cannot be less than %s",
				method.MaxTokenTTL, a.srv.config.ACLTokenMinExpirationTTL)
		}
	}

	switch method.TokenLocality {
	case "local", "":
	case "global":
		if !a.srv.InPrimaryDatacenter() {
			return fmt.Errorf("Invalid Auth Method: TokenLocality 'global' can only be used in the primary datacenter")
		}
	default:
		return fmt.Errorf("Invalid Auth Method: TokenLocality should be one of 'local' or 'global'")
	}

	// Instantiate a validator but do not cache it yet. This will validate the
	// configuration.
	validator, err := authmethod.NewValidator(a.srv.logger, method)
	if err != nil {
		return fmt.Errorf("Invalid Auth Method: %v", err)
	}

	if err := enterpriseAuthMethodValidation(method, validator); err != nil {
		return err
	}

	if err := a.srv.fsm.State().ACLAuthMethodUpsertValidateEnterprise(method, existing); err != nil {
		return err
	}

	req := &structs.ACLAuthMethodBatchSetRequest{
		AuthMethods: structs.ACLAuthMethods{method},
	}

	_, err = a.srv.raftApply(structs.ACLAuthMethodSetRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply auth method upsert request: %v", err)
	}

	if _, method, err := a.srv.fsm.State().ACLAuthMethodGetByName(nil, method.Name, &method.EnterpriseMeta); err == nil && method != nil {
		*reply = *method
	}

	return nil
}

func (a *ACL) AuthMethodDelete(args *structs.ACLAuthMethodDeleteRequest, reply *bool) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, true); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.AuthMethodDelete", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "authmethod", "delete"}, time.Now())

	// Verify token is permitted to modify ACLs
	var authzContext acl.AuthorizerContext

	if authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLWriteAllowed(&authzContext); err != nil {
		return err
	}

	_, method, err := a.srv.fsm.State().ACLAuthMethodGetByName(nil, args.AuthMethodName, &args.EnterpriseMeta)
	if err != nil {
		return err
	}

	if method == nil {
		return nil
	}

	if err := a.enterpriseAuthMethodTypeValidation(method.Type); err != nil {
		return err
	}

	req := structs.ACLAuthMethodBatchDeleteRequest{
		AuthMethodNames: []string{args.AuthMethodName},
		EnterpriseMeta:  args.EnterpriseMeta,
	}

	_, err = a.srv.raftApply(structs.ACLAuthMethodDeleteRequestType, &req)
	if err != nil {
		return fmt.Errorf("Failed to apply auth method delete request: %v", err)
	}

	*reply = true

	return nil
}

func (a *ACL) AuthMethodList(args *structs.ACLAuthMethodListRequest, reply *structs.ACLAuthMethodListResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if err := a.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if done, err := a.srv.ForwardRPC("ACL.AuthMethodList", args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext

	authz, err := a.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	} else if err := authz.ToAllowAuthorizer().ACLReadAllowed(&authzContext); err != nil {
		return err
	}

	return a.srv.blockingQuery(&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, methods, err := state.ACLAuthMethodList(ws, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			a.srv.filterACLWithAuthorizer(authz, &methods)

			var stubs structs.ACLAuthMethodListStubs
			for _, method := range methods {
				_ = a.enterpriseAuthMethodTypeValidation(method.Type)
				stubs = append(stubs, method.Stub())
			}

			reply.Index, reply.AuthMethods = index, stubs
			return nil
		})
}

func (a *ACL) Login(args *structs.ACLLoginRequest, reply *structs.ACLToken) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if args.Auth == nil {
		return fmt.Errorf("Invalid Login request: Missing auth parameters")
	}

	if err := a.srv.validateEnterpriseRequest(&args.Auth.EnterpriseMeta, true); err != nil {
		return err
	}

	if args.Token != "" { // This shouldn't happen.
		return errors.New("do not provide a token when logging in")
	}

	if done, err := a.srv.ForwardRPC("ACL.Login", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "login"}, time.Now())

	auth := args.Auth

	// 1. take args.Data.AuthMethod to get an AuthMethod Validator
	idx, method, err := a.srv.fsm.State().ACLAuthMethodGetByName(nil, auth.AuthMethod, &auth.EnterpriseMeta)
	if err != nil {
		return err
	} else if method == nil {
		return fmt.Errorf("%w: auth method %q not found", acl.ErrNotFound, auth.AuthMethod)
	}

	if err := a.enterpriseAuthMethodTypeValidation(method.Type); err != nil {
		return err
	}

	validator, err := a.srv.loadAuthMethodValidator(idx, method)
	if err != nil {
		return err
	}

	// 2. Send args.Data.BearerToken to method validator and get back a fields map
	verifiedIdentity, err := validator.ValidateLogin(context.Background(), auth.BearerToken)
	if err != nil {
		return err
	}

	return a.tokenSetFromAuthMethod(
		method,
		&auth.EnterpriseMeta,
		"token created via login",
		auth.Meta,
		validator,
		verifiedIdentity,
		&structs.ACLTokenSetRequest{
			Datacenter:   args.Datacenter,
			WriteRequest: args.WriteRequest,
		},
		reply,
	)
}

func (a *ACL) tokenSetFromAuthMethod(
	method *structs.ACLAuthMethod,
	entMeta *structs.EnterpriseMeta,
	tokenDescriptionPrefix string,
	tokenMetadata map[string]string,
	validator authmethod.Validator,
	verifiedIdentity *authmethod.Identity,
	createReq *structs.ACLTokenSetRequest, // this should be prepopulated with datacenter+writerequest
	reply *structs.ACLToken,
) error {
	// This always will return a valid pointer
	targetMeta, err := computeTargetEnterpriseMeta(method, verifiedIdentity)
	if err != nil {
		return err
	}

	// 3. send map through role bindings
	bindings, err := a.srv.evaluateRoleBindings(validator, verifiedIdentity, entMeta, targetMeta)
	if err != nil {
		return err
	}

	// We try to prevent the creation of a useless token without taking a trip
	// through the state store if we can.
	if bindings == nil || (len(bindings.serviceIdentities) == 0 && len(bindings.nodeIdentities) == 0 && len(bindings.roles) == 0) {
		return acl.ErrPermissionDenied
	}

	// TODO(sso): add a CapturedField to ACLAuthMethod that would pluck fields from the returned identity and stuff into `auth.Meta`.

	description := tokenDescriptionPrefix
	loginMeta, err := encodeLoginMeta(tokenMetadata)
	if err != nil {
		return err
	}
	if loginMeta != "" {
		description += ": " + loginMeta
	}

	// 4. create token
	createReq.ACLToken = structs.ACLToken{
		Description:       description,
		Local:             true,
		AuthMethod:        method.Name,
		ServiceIdentities: bindings.serviceIdentities,
		NodeIdentities:    bindings.nodeIdentities,
		Roles:             bindings.roles,
		ExpirationTTL:     method.MaxTokenTTL,
		EnterpriseMeta:    *targetMeta,
	}

	if method.TokenLocality == "global" {
		if !a.srv.InPrimaryDatacenter() {
			return errors.New("creating global tokens via auth methods is only permitted in the primary datacenter")
		}
		createReq.ACLToken.Local = false
	}

	createReq.ACLToken.ACLAuthMethodEnterpriseMeta.FillWithEnterpriseMeta(entMeta)

	// 5. return token information like a TokenCreate would
	err = a.tokenSetInternal(createReq, reply, true)

	// If we were in a slight race with a role delete operation then we may
	// still end up failing to insert an unprivileged token in the state
	// machine instead.  Return the same error as earlier so it doesn't
	// actually matter which one prevents the insertion.
	if err != nil && err.Error() == state.ErrTokenHasNoPrivileges.Error() {
		return acl.ErrPermissionDenied
	}

	return err
}

func encodeLoginMeta(meta map[string]string) (string, error) {
	if len(meta) == 0 {
		return "", nil
	}

	d, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(d), nil
}

func (a *ACL) Logout(args *structs.ACLLogoutRequest, reply *bool) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if !a.srv.LocalTokensEnabled() {
		return errAuthMethodsRequireTokenReplication
	}

	if args.Token == "" {
		return acl.ErrNotFound
	}

	if done, err := a.srv.ForwardRPC("ACL.Logout", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"acl", "logout"}, time.Now())

	_, token, err := a.srv.fsm.State().ACLTokenGetBySecret(nil, args.Token, nil)
	if err != nil {
		return err

	} else if token == nil {
		return acl.ErrNotFound

	} else if token.AuthMethod == "" {
		// Can't "logout" of a token that wasn't a result of login.
		return acl.ErrPermissionDenied

	} else if !a.srv.InPrimaryDatacenter() && !token.Local {
		// global token writes must be forwarded to the primary DC
		args.Datacenter = a.srv.config.PrimaryDatacenter
		return a.srv.forwardDC("ACL.Logout", a.srv.config.PrimaryDatacenter, args, reply)
	}

	// No need to check expiration time because it's being deleted.

	req := &structs.ACLTokenBatchDeleteRequest{
		TokenIDs: []string{token.AccessorID},
	}

	_, err = a.srv.raftApply(structs.ACLTokenDeleteRequestType, req)
	if err != nil {
		return fmt.Errorf("Failed to apply token delete request: %v", err)
	}

	// Purge the identity from the cache to prevent using the previous definition of the identity
	a.srv.ACLResolver.cache.RemoveIdentity(tokenSecretCacheID(token.SecretID))

	*reply = true

	return nil
}

func (a *ACL) Authorize(args *structs.RemoteACLAuthorizationRequest, reply *[]structs.ACLAuthorizationResponse) error {
	if err := a.aclPreCheck(); err != nil {
		return err
	}

	if done, err := a.srv.ForwardRPC("ACL.Authorize", args, reply); done {
		return err
	}

	authz, err := a.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	responses, err := structs.CreateACLAuthorizationResponses(authz, args.Requests)
	if err != nil {
		return err
	}

	*reply = responses
	return nil
}

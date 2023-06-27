// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/logging"
)

var ACLCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"acl", "token", "cache_hit"},
		Help: "Increments if Consul is able to resolve a token's identity, or a legacy token, from the cache.",
	},
	{
		Name: []string{"acl", "token", "cache_miss"},
		Help: "Increments if Consul cannot resolve a token's identity, or a legacy token, from the cache.",
	},
}

var ACLSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"acl", "ResolveToken"},
		Help: "This measures the time it takes to resolve an ACL token.",
	},
}

// These must be kept in sync with the constants in command/agent/acl.go.
const (
	// anonymousToken is the token SecretID we re-write to if there is no token ID
	// provided.
	anonymousToken = "anonymous"

	// aclTokenReapingRateLimit is the number of batch token reaping requests per second allowed.
	aclTokenReapingRateLimit rate.Limit = 1.0

	// aclTokenReapingBurst is the number of batch token reaping requests per second
	// that can burst after a period of idleness.
	aclTokenReapingBurst = 5

	// aclBatchDeleteSize is the number of deletions to send in a single batch operation. 4096 should produce a batch that is <150KB
	// in size but should be sufficiently large to handle 1 replication round in a single batch
	aclBatchDeleteSize = 4096

	// aclBatchUpsertSize is the target size in bytes we want to submit for a batch upsert request. We estimate the size at runtime
	// due to the data being more variable in its size.
	aclBatchUpsertSize = 256 * 1024

	// Maximum number of re-resolution requests to be made if the token is modified between
	// resolving the token and resolving its policies that would remove one of its policies.
	tokenPolicyResolutionMaxRetries = 5

	// Maximum number of re-resolution requests to be made if the token is modified between
	// resolving the token and resolving its roles that would remove one of its roles.
	tokenRoleResolutionMaxRetries = 5
)

// missingIdentity is used to return some identity in the event that the real identity cannot be ascertained
type missingIdentity struct {
	reason string
	token  string
}

func (id *missingIdentity) ID() string {
	return id.reason
}

func (id *missingIdentity) SecretToken() string {
	return id.token
}

func (id *missingIdentity) PolicyIDs() []string {
	return nil
}

func (id *missingIdentity) RoleIDs() []string {
	return nil
}

func (id *missingIdentity) ServiceIdentityList() []*structs.ACLServiceIdentity {
	return nil
}

func (id *missingIdentity) NodeIdentityList() []*structs.ACLNodeIdentity {
	return nil
}

func (id *missingIdentity) IsExpired(asOf time.Time) bool {
	return false
}

func (id *missingIdentity) IsLocal() bool {
	return false
}

func (id *missingIdentity) EnterpriseMetadata() *acl.EnterpriseMeta {
	return structs.DefaultEnterpriseMetaInDefaultPartition()
}

type ACLRemoteError struct {
	Err error
}

func (e ACLRemoteError) Error() string {
	return fmt.Sprintf("Error communicating with the ACL Datacenter: %v", e.Err)
}

func IsACLRemoteError(err error) bool {
	_, ok := err.(ACLRemoteError)
	return ok
}

func tokenSecretCacheID(token string) string {
	return "token-secret:" + token
}

type ACLResolverBackend interface {
	ACLDatacenter() string
	ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error)
	ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error)
	ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error)
	IsServerManagementToken(token string) bool
	// TODO: separate methods for each RPC call (there are 4)
	RPC(ctx context.Context, method string, args interface{}, reply interface{}) error
	EnterpriseACLResolverDelegate
}

type policyOrRoleTokenError struct {
	Err   error
	token string
}

func (e policyOrRoleTokenError) Error() string {
	return e.Err.Error()
}

// ACLResolverConfig holds all the configuration necessary to create an ACLResolver
type ACLResolverConfig struct {
	// TODO: rename this field?
	Config ACLResolverSettings
	Logger hclog.Logger

	// CacheConfig is a pass through configuration for ACL cache limits
	CacheConfig *structs.ACLCachesConfig

	// Backend is used to retrieve data from the state store, or perform RPCs
	// to fetch data from other Datacenters.
	Backend ACLResolverBackend

	// DisableDuration is the length of time to leave ACLs disabled when an RPC
	// request to a server indicates that the ACL system is disabled. If set to
	// 0 then ACLs will not be disabled locally. This value is always set to 0 on
	// Servers.
	DisableDuration time.Duration

	// ACLConfig is the configuration necessary to pass through to the acl package when creating authorizers
	// and when authorizing access
	ACLConfig *acl.Config

	// Tokens is the token store of locally managed tokens
	Tokens *token.Store
}

const aclClientDisabledTTL = 30 * time.Second

// TODO: rename the fields to remove the ACL prefix
type ACLResolverSettings struct {
	ACLsEnabled    bool
	Datacenter     string
	NodeName       string
	EnterpriseMeta acl.EnterpriseMeta

	// ACLPolicyTTL is used to control the time-to-live of cached ACL policies. This has
	// a major impact on performance. By default, it is set to 30 seconds.
	ACLPolicyTTL time.Duration
	// ACLTokenTTL is used to control the time-to-live of cached ACL tokens. This has
	// a major impact on performance. By default, it is set to 30 seconds.
	ACLTokenTTL time.Duration
	// ACLRoleTTL is used to control the time-to-live of cached ACL roles. This has
	// a major impact on performance. By default, it is set to 30 seconds.
	ACLRoleTTL time.Duration

	// ACLDownPolicy is used to control the ACL interaction when we cannot
	// reach the PrimaryDatacenter and the token is not in the cache.
	// There are the following modes:
	//   * allow - Allow all requests
	//   * deny - Deny all requests
	//   * extend-cache - Ignore the cache expiration, and allow cached
	//                    ACL's to be used to service requests. This
	//                    is the default. If the ACL is not in the cache,
	//                    this acts like deny.
	//   * async-cache - Same behavior as extend-cache, but perform ACL
	//                   Lookups asynchronously when cache TTL is expired.
	ACLDownPolicy string

	// ACLDefaultPolicy is used to control the ACL interaction when
	// there is no defined policy. This can be "allow" which means
	// ACLs are used to deny-list, or "deny" which means ACLs are
	// allow-lists.
	ACLDefaultPolicy string
}

// ACLResolver is the type to handle all your token and policy resolution needs.
//
// Supports:
//   - Resolving tokens locally via the ACLResolverBackend
//   - Resolving policies locally via the ACLResolverBackend
//   - Resolving roles locally via the ACLResolverBackend
//   - Resolving legacy tokens remotely via an ACL.GetPolicy RPC
//   - Resolving tokens remotely via an ACL.TokenRead RPC
//   - Resolving policies remotely via an ACL.PolicyResolve RPC
//   - Resolving roles remotely via an ACL.RoleResolve RPC
//
// Remote Resolution:
//
//	Remote resolution can be done synchronously or asynchronously depending
//	on the ACLDownPolicy in the Config passed to the resolver.
//
//	When the down policy is set to async-cache and we have already cached values
//	then go routines will be spawned to perform the RPCs in the background
//	and then will update the cache with either the positive or negative result.
//
//	When the down policy is set to extend-cache or the token/policy/role is not already
//	cached then the same go routines are spawned to do the RPCs in the background.
//	However in this mode channels are created to receive the results of the RPC
//	and are registered with the resolver. Those channels are immediately read/blocked
//	upon.
type ACLResolver struct {
	config ACLResolverSettings
	logger hclog.Logger

	backend ACLResolverBackend
	aclConf *acl.Config

	tokens *token.Store

	cache         *structs.ACLCaches
	identityGroup singleflight.Group
	policyGroup   singleflight.Group
	roleGroup     singleflight.Group
	legacyGroup   singleflight.Group

	down acl.Authorizer

	disableDuration time.Duration
	disabledUntil   time.Time
	// disabledLock synchronizes access to disabledUntil
	disabledLock sync.RWMutex

	agentRecoveryAuthz acl.Authorizer
}

func agentRecoveryAuthorizer(nodeName string, entMeta *acl.EnterpriseMeta, aclConf *acl.Config) (acl.Authorizer, error) {
	var conf acl.Config
	if aclConf != nil {
		conf = *aclConf
	}
	setEnterpriseConf(entMeta, &conf)

	// Build a policy for the agent recovery token.
	//
	// The builtin agent recovery policy allows reading any node information
	// and allows writes to the agent with the node name of the running agent
	// only. This used to allow a prefix match on agent names but that seems
	// entirely unnecessary so it is now using an exact match.
	policy, err := acl.NewPolicyFromSource(fmt.Sprintf(`
	agent "%s" {
		policy = "write"
	}
	node_prefix "" {
		policy = "read"
	}
	`, nodeName), &conf, entMeta.ToEnterprisePolicyMeta())
	if err != nil {
		return nil, err
	}

	return acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, &conf)
}

func NewACLResolver(config *ACLResolverConfig) (*ACLResolver, error) {
	if config == nil {
		return nil, fmt.Errorf("ACL Resolver must be initialized with a config")
	}
	if config.Backend == nil {
		return nil, fmt.Errorf("ACL Resolver must be initialized with a valid backend")
	}

	if config.Logger == nil {
		config.Logger = hclog.New(&hclog.LoggerOptions{})
	}

	cache, err := structs.NewACLCaches(config.CacheConfig)
	if err != nil {
		return nil, err
	}

	var down acl.Authorizer
	switch config.Config.ACLDownPolicy {
	case "allow":
		down = acl.AllowAll()
	case "deny":
		down = acl.DenyAll()
	case "async-cache", "extend-cache":
		down = acl.RootAuthorizer(config.Config.ACLDefaultPolicy)
	default:
		return nil, fmt.Errorf("invalid ACL down policy %q", config.Config.ACLDownPolicy)
	}

	authz, err := agentRecoveryAuthorizer(config.Config.NodeName, &config.Config.EnterpriseMeta, config.ACLConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the agent recovery authorizer")
	}

	return &ACLResolver{
		config:             config.Config,
		logger:             config.Logger.Named(logging.ACL),
		backend:            config.Backend,
		aclConf:            config.ACLConfig,
		cache:              cache,
		disableDuration:    config.DisableDuration,
		down:               down,
		tokens:             config.Tokens,
		agentRecoveryAuthz: authz,
	}, nil
}

func (r *ACLResolver) Close() {
	r.aclConf.Close()
}

func (r *ACLResolver) fetchAndCacheIdentityFromToken(token string, cached *structs.IdentityCacheEntry) (structs.ACLIdentity, error) {
	req := structs.ACLTokenGetRequest{
		Datacenter:  r.backend.ACLDatacenter(),
		TokenID:     token,
		TokenIDType: structs.ACLTokenSecret,
		QueryOptions: structs.QueryOptions{
			Token:      token,
			AllowStale: true,
		},
	}

	var resp structs.ACLTokenResponse
	err := r.backend.RPC(context.Background(), "ACL.TokenRead", &req, &resp)
	if err == nil {
		if resp.Token == nil {
			r.cache.RemoveIdentityWithSecretToken(token)
			return nil, acl.ErrNotFound
		} else if resp.Token.Local && r.config.Datacenter != resp.SourceDatacenter {
			r.cache.RemoveIdentityWithSecretToken(token)
			return nil, acl.PermissionDeniedError{Cause: fmt.Sprintf("This is a local token in datacenter %q", resp.SourceDatacenter)}
		} else {
			r.cache.PutIdentityWithSecretToken(token, resp.Token)
			return resp.Token, nil
		}
	}

	if acl.IsErrNotFound(err) {
		// Make sure to remove from the cache if it was deleted
		r.cache.RemoveIdentityWithSecretToken(token)
		return nil, acl.ErrNotFound

	}

	// some other RPC error
	if cached != nil && (r.config.ACLDownPolicy == "extend-cache" || r.config.ACLDownPolicy == "async-cache") {
		// extend the cache
		r.cache.PutIdentityWithSecretToken(token, cached.Identity)
		return cached.Identity, nil
	}

	r.cache.RemoveIdentityWithSecretToken(token)
	return nil, err
}

// resolveIdentityFromToken takes a token secret as a string and returns an ACLIdentity.
// We read the value from ACLResolver's cache if available, and if the read misses
// we initiate an RPC for the value.
func (r *ACLResolver) resolveIdentityFromToken(token string) (structs.ACLIdentity, error) {
	// Attempt to resolve locally first (local results are not cached)
	if done, identity, err := r.backend.ResolveIdentityFromToken(token); done {
		return identity, err
	}

	// Check the cache before making any RPC requests
	cacheEntry := r.cache.GetIdentityWithSecretToken(token)
	if cacheEntry != nil && cacheEntry.Age() <= r.config.ACLTokenTTL {
		metrics.IncrCounter([]string{"acl", "token", "cache_hit"}, 1)
		return cacheEntry.Identity, nil
	}

	metrics.IncrCounter([]string{"acl", "token", "cache_miss"}, 1)

	// Background a RPC request and wait on it if we must
	waitChan := r.identityGroup.DoChan(token, func() (interface{}, error) {
		identity, err := r.fetchAndCacheIdentityFromToken(token, cacheEntry)
		return identity, err
	})

	waitForResult := cacheEntry == nil || r.config.ACLDownPolicy != "async-cache"
	if !waitForResult {
		// waitForResult being false requires the cacheEntry to not be nil
		return cacheEntry.Identity, nil
	}

	// block on the read here, this is why we don't need chan buffering
	res := <-waitChan

	var identity structs.ACLIdentity
	if res.Val != nil { // avoid a nil-not-nil bug
		identity = res.Val.(structs.ACLIdentity)
	}

	if res.Err != nil && !acl.IsErrNotFound(res.Err) {
		return identity, ACLRemoteError{Err: res.Err}
	}
	return identity, res.Err
}

func (r *ACLResolver) fetchAndCachePoliciesForIdentity(identity structs.ACLIdentity, policyIDs []string, cached map[string]*structs.PolicyCacheEntry) (map[string]*structs.ACLPolicy, error) {
	req := structs.ACLPolicyBatchGetRequest{
		Datacenter: r.backend.ACLDatacenter(),
		PolicyIDs:  policyIDs,
		QueryOptions: structs.QueryOptions{
			Token:      identity.SecretToken(),
			AllowStale: true,
		},
	}

	var resp structs.ACLPolicyBatchResponse
	err := r.backend.RPC(context.Background(), "ACL.PolicyResolve", &req, &resp)
	if err == nil {
		out := make(map[string]*structs.ACLPolicy)
		for _, policy := range resp.Policies {
			out[policy.ID] = policy
		}

		for _, policyID := range policyIDs {
			if policy, ok := out[policyID]; ok {
				r.cache.PutPolicy(policyID, policy)
			} else {
				r.cache.PutPolicy(policyID, nil)
			}
		}
		return out, nil
	}

	if handledErr := r.maybeHandleIdentityErrorDuringFetch(identity, err); handledErr != nil {
		return nil, handledErr
	}

	// other RPC error - use cache if available

	extendCache := r.config.ACLDownPolicy == "extend-cache" || r.config.ACLDownPolicy == "async-cache"

	out := make(map[string]*structs.ACLPolicy)
	insufficientCache := false
	for _, policyID := range policyIDs {
		if entry, ok := cached[policyID]; extendCache && ok {
			r.cache.PutPolicy(policyID, entry.Policy)
			if entry.Policy != nil {
				out[policyID] = entry.Policy
			}
		} else {
			r.cache.PutPolicy(policyID, nil)
			insufficientCache = true
		}
	}
	if insufficientCache {
		return nil, ACLRemoteError{Err: err}
	}
	return out, nil
}

func (r *ACLResolver) fetchAndCacheRolesForIdentity(identity structs.ACLIdentity, roleIDs []string, cached map[string]*structs.RoleCacheEntry) (map[string]*structs.ACLRole, error) {
	req := structs.ACLRoleBatchGetRequest{
		Datacenter: r.backend.ACLDatacenter(),
		RoleIDs:    roleIDs,
		QueryOptions: structs.QueryOptions{
			Token:      identity.SecretToken(),
			AllowStale: true,
		},
	}

	var resp structs.ACLRoleBatchResponse
	err := r.backend.RPC(context.Background(), "ACL.RoleResolve", &req, &resp)
	if err == nil {
		out := make(map[string]*structs.ACLRole)
		for _, role := range resp.Roles {
			out[role.ID] = role
		}

		for _, roleID := range roleIDs {
			if role, ok := out[roleID]; ok {
				r.cache.PutRole(roleID, role)
			} else {
				r.cache.PutRole(roleID, nil)
			}
		}
		return out, nil
	}

	if handledErr := r.maybeHandleIdentityErrorDuringFetch(identity, err); handledErr != nil {
		return nil, handledErr
	}

	// other RPC error - use cache if available

	extendCache := r.config.ACLDownPolicy == "extend-cache" || r.config.ACLDownPolicy == "async-cache"

	out := make(map[string]*structs.ACLRole)
	insufficientCache := false
	for _, roleID := range roleIDs {
		if entry, ok := cached[roleID]; extendCache && ok {
			r.cache.PutRole(roleID, entry.Role)
			if entry.Role != nil {
				out[roleID] = entry.Role
			}
		} else {
			r.cache.PutRole(roleID, nil)
			insufficientCache = true
		}
	}

	if insufficientCache {
		return nil, ACLRemoteError{Err: err}
	}

	return out, nil
}

func (r *ACLResolver) maybeHandleIdentityErrorDuringFetch(identity structs.ACLIdentity, err error) error {
	if acl.IsErrNotFound(err) {
		// make sure to indicate that this identity is no longer valid within
		// the cache
		r.cache.RemoveIdentityWithSecretToken(identity.SecretToken())

		// Do not touch the cache. Getting a top level ACL not found error
		// only indicates that the secret token used in the request
		// no longer exists
		return &policyOrRoleTokenError{acl.ErrNotFound, identity.SecretToken()}
	}

	if acl.IsErrPermissionDenied(err) {
		// invalidate our ID cache so that identity resolution will take place
		// again in the future
		r.cache.RemoveIdentityWithSecretToken(identity.SecretToken())

		// Do not remove from the cache for permission denied
		// what this does indicate is that our view of the token is out of date
		return &policyOrRoleTokenError{acl.ErrPermissionDenied, identity.SecretToken()}
	}

	return nil
}

func (r *ACLResolver) filterPoliciesByScope(policies structs.ACLPolicies) structs.ACLPolicies {
	var out structs.ACLPolicies
	for _, policy := range policies {
		if len(policy.Datacenters) == 0 {
			out = append(out, policy)
			continue
		}

		for _, dc := range policy.Datacenters {
			if dc == r.config.Datacenter {
				out = append(out, policy)
				continue
			}
		}
	}

	return out
}

func (r *ACLResolver) resolvePoliciesForIdentity(identity structs.ACLIdentity) (structs.ACLPolicies, error) {
	var (
		policyIDs         = identity.PolicyIDs()
		roleIDs           = identity.RoleIDs()
		serviceIdentities = structs.ACLServiceIdentities(identity.ServiceIdentityList())
		nodeIdentities    = structs.ACLNodeIdentities(identity.NodeIdentityList())
	)

	if len(policyIDs) == 0 && len(serviceIdentities) == 0 && len(roleIDs) == 0 && len(nodeIdentities) == 0 {
		// In this case the default policy will be all that is in effect.
		return nil, nil
	}

	// Collect all of the roles tied to this token.
	roles, err := r.collectRolesForIdentity(identity, roleIDs)
	if err != nil {
		return nil, err
	}

	// Merge the policies and service identities across Token and Role fields.
	for _, role := range roles {
		for _, link := range role.Policies {
			policyIDs = append(policyIDs, link.ID)
		}
		serviceIdentities = append(serviceIdentities, role.ServiceIdentities...)
		nodeIdentities = append(nodeIdentities, role.NodeIdentityList()...)
	}

	// Now deduplicate any policies or service identities that occur more than once.
	policyIDs = dedupeStringSlice(policyIDs)
	serviceIdentities = serviceIdentities.Deduplicate()
	nodeIdentities = nodeIdentities.Deduplicate()

	// Generate synthetic policies for all service identities in effect.
	syntheticPolicies := r.synthesizePoliciesForServiceIdentities(serviceIdentities, identity.EnterpriseMetadata())
	syntheticPolicies = append(syntheticPolicies, r.synthesizePoliciesForNodeIdentities(nodeIdentities, identity.EnterpriseMetadata())...)

	// For the new ACLs policy replication is mandatory for correct operation on servers. Therefore
	// we only attempt to resolve policies locally
	policies, err := r.collectPoliciesForIdentity(identity, policyIDs, len(syntheticPolicies))
	if err != nil {
		return nil, err
	}

	policies = append(policies, syntheticPolicies...)
	filtered := r.filterPoliciesByScope(policies)
	if len(policies) > 0 && len(filtered) == 0 {
		r.logger.Warn("ACL token used lacks permissions in this datacenter: its associated ACL policies, service identities, and/or node identities are scoped to other datacenters", "accessor_id", identity.ID(), "datacenter", r.config.Datacenter)
	}

	return filtered, nil
}

func (r *ACLResolver) synthesizePoliciesForServiceIdentities(serviceIdentities []*structs.ACLServiceIdentity, entMeta *acl.EnterpriseMeta) []*structs.ACLPolicy {
	if len(serviceIdentities) == 0 {
		return nil
	}

	syntheticPolicies := make([]*structs.ACLPolicy, 0, len(serviceIdentities))
	for _, s := range serviceIdentities {
		syntheticPolicies = append(syntheticPolicies, s.SyntheticPolicy(entMeta))
	}

	return syntheticPolicies
}

func (r *ACLResolver) synthesizePoliciesForNodeIdentities(nodeIdentities []*structs.ACLNodeIdentity, entMeta *acl.EnterpriseMeta) []*structs.ACLPolicy {
	if len(nodeIdentities) == 0 {
		return nil
	}

	syntheticPolicies := make([]*structs.ACLPolicy, 0, len(nodeIdentities))
	for _, n := range nodeIdentities {
		syntheticPolicies = append(syntheticPolicies, n.SyntheticPolicy(entMeta))
	}

	return syntheticPolicies
}

func mergeStringSlice(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return dedupeStringSlice(out)
}

func dedupeStringSlice(in []string) []string {
	// From: https://github.com/golang/go/wiki/SliceTricks#in-place-deduplicate-comparable

	if len(in) <= 1 {
		return in
	}

	sort.Strings(in)

	j := 0
	for i := 1; i < len(in); i++ {
		if in[j] == in[i] {
			continue
		}
		j++
		in[j] = in[i]
	}

	return in[:j+1]
}

func (r *ACLResolver) collectPoliciesForIdentity(identity structs.ACLIdentity, policyIDs []string, extraCap int) ([]*structs.ACLPolicy, error) {
	policies := make([]*structs.ACLPolicy, 0, len(policyIDs)+extraCap)

	// Get all associated policies
	var missing []string
	var expired []*structs.ACLPolicy
	expCacheMap := make(map[string]*structs.PolicyCacheEntry)

	var accessorID string
	if identity != nil {
		accessorID = identity.ID()
	}

	for _, policyID := range policyIDs {
		if done, policy, err := r.backend.ResolvePolicyFromID(policyID); done {
			if err != nil && !acl.IsErrNotFound(err) {
				return nil, err
			}

			if policy != nil {
				policies = append(policies, policy)
			} else {
				r.logger.Warn("policy not found for identity",
					"policy", policyID,
					"accessorID", acl.AliasIfAnonymousToken(accessorID),
				)
			}

			continue
		}

		// create the missing list which we can execute an RPC to get all the missing policies at once
		entry := r.cache.GetPolicy(policyID)
		if entry == nil {
			missing = append(missing, policyID)
			continue
		}

		if entry.Policy == nil {
			// this happens when we cache a negative response for the policy's existence
			continue
		}

		if entry.Age() >= r.config.ACLPolicyTTL {
			expired = append(expired, entry.Policy)
			expCacheMap[policyID] = entry
		} else {
			policies = append(policies, entry.Policy)
		}
	}

	// Hot-path if we have no missing or expired policies
	if len(missing)+len(expired) == 0 {
		return policies, nil
	}

	hasMissing := len(missing) > 0

	fetchIDs := missing
	for _, policy := range expired {
		fetchIDs = append(fetchIDs, policy.ID)
	}

	// Background a RPC request and wait on it if we must
	waitChan := r.policyGroup.DoChan(identity.SecretToken(), func() (interface{}, error) {
		policies, err := r.fetchAndCachePoliciesForIdentity(identity, fetchIDs, expCacheMap)
		return policies, err
	})

	waitForResult := hasMissing || r.config.ACLDownPolicy != "async-cache"
	if !waitForResult {
		// waitForResult being false requires that all the policies were cached already
		policies = append(policies, expired...)
		return policies, nil
	}

	res := <-waitChan

	if res.Err != nil {
		return nil, res.Err
	}

	if res.Val != nil {
		foundPolicies := res.Val.(map[string]*structs.ACLPolicy)

		for _, policy := range foundPolicies {
			policies = append(policies, policy)
		}
	}

	return policies, nil
}

func (r *ACLResolver) resolveRolesForIdentity(identity structs.ACLIdentity) (structs.ACLRoles, error) {
	return r.collectRolesForIdentity(identity, identity.RoleIDs())
}

func (r *ACLResolver) collectRolesForIdentity(identity structs.ACLIdentity, roleIDs []string) (structs.ACLRoles, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}

	// For the new ACLs policy & role replication is mandatory for correct operation
	// on servers. Therefore we only attempt to resolve roles locally
	roles := make([]*structs.ACLRole, 0, len(roleIDs))

	var missing []string
	var expired []*structs.ACLRole
	expCacheMap := make(map[string]*structs.RoleCacheEntry)

	for _, roleID := range roleIDs {
		if done, role, err := r.backend.ResolveRoleFromID(roleID); done {
			if err != nil && !acl.IsErrNotFound(err) {
				return nil, err
			}

			if role != nil {
				roles = append(roles, role)
			} else {
				var accessorID string
				if identity != nil {
					accessorID = identity.ID()
				}
				r.logger.Warn("role not found for identity",
					"role", roleID,
					"accessorID", acl.AliasIfAnonymousToken(accessorID),
				)
			}

			continue
		}

		// create the missing list which we can execute an RPC to get all the missing roles at once
		entry := r.cache.GetRole(roleID)
		if entry == nil {
			missing = append(missing, roleID)
			continue
		}

		if entry.Role == nil {
			// this happens when we cache a negative response for the role's existence
			continue
		}

		if entry.Age() >= r.config.ACLRoleTTL {
			expired = append(expired, entry.Role)
			expCacheMap[roleID] = entry
		} else {
			roles = append(roles, entry.Role)
		}
	}

	// Hot-path if we have no missing or expired roles
	if len(missing)+len(expired) == 0 {
		return roles, nil
	}

	hasMissing := len(missing) > 0

	fetchIDs := missing
	for _, role := range expired {
		fetchIDs = append(fetchIDs, role.ID)
	}

	waitChan := r.roleGroup.DoChan(identity.SecretToken(), func() (interface{}, error) {
		roles, err := r.fetchAndCacheRolesForIdentity(identity, fetchIDs, expCacheMap)
		return roles, err
	})

	waitForResult := hasMissing || r.config.ACLDownPolicy != "async-cache"
	if !waitForResult {
		// waitForResult being false requires that all the roles were cached already
		roles = append(roles, expired...)
		return roles, nil
	}

	res := <-waitChan

	if res.Err != nil {
		return nil, res.Err
	}

	if res.Val != nil {
		foundRoles := res.Val.(map[string]*structs.ACLRole)

		for _, role := range foundRoles {
			roles = append(roles, role)
		}
	}

	return roles, nil
}

func (r *ACLResolver) resolveTokenToIdentityAndPolicies(token string) (structs.ACLIdentity, structs.ACLPolicies, error) {
	var lastErr error
	var lastIdentity structs.ACLIdentity

	for i := 0; i < tokenPolicyResolutionMaxRetries; i++ {
		// Resolve the token to an ACLIdentity
		identity, err := r.resolveIdentityFromToken(token)
		if err != nil {
			return nil, nil, err
		} else if identity == nil {
			return nil, nil, acl.ErrNotFound
		} else if identity.IsExpired(time.Now()) {
			return nil, nil, acl.ErrNotFound
		}

		lastIdentity = identity

		policies, err := r.resolvePoliciesForIdentity(identity)
		if err == nil {
			return identity, policies, nil
		}
		lastErr = err

		if tokenErr, ok := err.(*policyOrRoleTokenError); ok {
			if acl.IsErrNotFound(err) && tokenErr.token == identity.SecretToken() {
				// token was deleted while resolving policies
				return nil, nil, acl.ErrNotFound
			}

			// other types of policyOrRoleTokenErrors should cause retrying the whole token
			// resolution process
		} else {
			return identity, nil, err
		}
	}

	return lastIdentity, nil, lastErr
}

func (r *ACLResolver) resolveTokenToIdentityAndRoles(token string) (structs.ACLIdentity, structs.ACLRoles, error) {
	var lastErr error
	var lastIdentity structs.ACLIdentity

	for i := 0; i < tokenRoleResolutionMaxRetries; i++ {
		// Resolve the token to an ACLIdentity
		identity, err := r.resolveIdentityFromToken(token)
		if err != nil {
			return nil, nil, err
		} else if identity == nil {
			return nil, nil, acl.ErrNotFound
		} else if identity.IsExpired(time.Now()) {
			return nil, nil, acl.ErrNotFound
		}

		lastIdentity = identity

		roles, err := r.resolveRolesForIdentity(identity)
		if err == nil {
			return identity, roles, nil
		}
		lastErr = err

		if tokenErr, ok := err.(*policyOrRoleTokenError); ok {
			if acl.IsErrNotFound(err) && tokenErr.token == identity.SecretToken() {
				// token was deleted while resolving roles
				return nil, nil, acl.ErrNotFound
			}

			// other types of policyOrRoleTokenErrors should cause retrying the whole token
			// resolution process
		} else {
			return identity, nil, err
		}
	}

	return lastIdentity, nil, lastErr
}

func (r *ACLResolver) handleACLDisabledError(err error) {
	if r.disableDuration == 0 || err == nil || !acl.IsErrDisabled(err) {
		return
	}

	r.logger.Debug("ACLs disabled on servers, will retry", "retry_interval", r.disableDuration)
	r.disabledLock.Lock()
	r.disabledUntil = time.Now().Add(r.disableDuration)
	r.disabledLock.Unlock()
}

func (r *ACLResolver) resolveLocallyManagedToken(token string) (structs.ACLIdentity, acl.Authorizer, bool) {
	// can only resolve local tokens if we were given a token store
	if r.tokens == nil {
		return nil, nil, false
	}

	if r.tokens.IsAgentRecoveryToken(token) {
		return structs.NewAgentRecoveryTokenIdentity(r.config.NodeName, token), r.agentRecoveryAuthz, true
	}

	if r.backend.IsServerManagementToken(token) {
		return structs.NewACLServerIdentity(token), acl.ManageAll(), true
	}

	return r.resolveLocallyManagedEnterpriseToken(token)
}

// ResolveToken to an acl.Authorizer and structs.ACLIdentity. The acl.Authorizer
// can be used to check permissions granted to the token using its secret, and the
// ACLIdentity describes the token and any defaults applied to it.
func (r *ACLResolver) ResolveToken(tokenSecretID string) (resolver.Result, error) {
	if !r.ACLsEnabled() {
		return resolver.Result{Authorizer: acl.ManageAll()}, nil
	}

	if acl.RootAuthorizer(tokenSecretID) != nil {
		return resolver.Result{}, acl.ErrRootDenied
	}

	// handle the anonymous token
	if tokenSecretID == "" {
		tokenSecretID = anonymousToken
	}

	if ident, authz, ok := r.resolveLocallyManagedToken(tokenSecretID); ok {
		return resolver.Result{Authorizer: authz, ACLIdentity: ident}, nil
	}

	defer metrics.MeasureSince([]string{"acl", "ResolveToken"}, time.Now())

	identity, policies, err := r.resolveTokenToIdentityAndPolicies(tokenSecretID)
	if err != nil {
		r.handleACLDisabledError(err)
		if IsACLRemoteError(err) {
			r.logger.Error("Error resolving token", "error", err)
			ident := &missingIdentity{reason: "primary-dc-down", token: tokenSecretID}
			return resolver.Result{Authorizer: r.down, ACLIdentity: ident}, nil
		}

		return resolver.Result{}, err
	}

	// Build the Authorizer
	var chain []acl.Authorizer
	var conf acl.Config
	if r.aclConf != nil {
		conf = *r.aclConf
	}
	setEnterpriseConf(identity.EnterpriseMetadata(), &conf)

	authz, err := policies.Compile(r.cache, &conf)
	if err != nil {
		return resolver.Result{}, err
	}
	chain = append(chain, authz)

	authz, err = r.resolveEnterpriseDefaultsForIdentity(identity)
	if err != nil {
		if IsACLRemoteError(err) {
			r.logger.Error("Error resolving identity defaults", "error", err)
			return resolver.Result{Authorizer: r.down, ACLIdentity: identity}, nil
		}
		return resolver.Result{}, err
	} else if authz != nil {
		chain = append(chain, authz)
	}

	chain = append(chain, acl.RootAuthorizer(r.config.ACLDefaultPolicy))
	return resolver.Result{Authorizer: acl.NewChainedAuthorizer(chain), ACLIdentity: identity}, nil
}

func (r *ACLResolver) ACLsEnabled() bool {
	// Whether we desire ACLs to be enabled according to configuration
	if !r.config.ACLsEnabled {
		return false
	}

	if r.disableDuration != 0 {
		// Whether ACLs are disabled according to RPCs failing with a ACLs Disabled error
		r.disabledLock.RLock()
		defer r.disabledLock.RUnlock()
		return time.Now().After(r.disabledUntil)
	}

	return true
}

func (r *ACLResolver) ResolveTokenAndDefaultMeta(
	tokenSecretID string,
	entMeta *acl.EnterpriseMeta,
	authzContext *acl.AuthorizerContext,
) (resolver.Result, error) {
	result, err := r.ResolveToken(tokenSecretID)
	if err != nil {
		return resolver.Result{}, err
	}

	if entMeta == nil {
		entMeta = &acl.EnterpriseMeta{}
	}

	// Default the EnterpriseMeta based on the Tokens meta or actual defaults
	// in the case of unknown identity
	switch {
	case authzContext.PeerOrEmpty() == "" && result.ACLIdentity != nil:
		entMeta.Merge(result.ACLIdentity.EnterpriseMetadata())

	case result.ACLIdentity != nil:
		// We _do not_ normalize the enterprise meta from the token when a peer
		// name was specified because namespaces across clusters are not
		// equivalent. A local namespace is _never_ correct for a remote query.
		entMeta.Merge(
			structs.DefaultEnterpriseMetaInPartition(
				result.ACLIdentity.EnterpriseMetadata().PartitionOrDefault(),
			),
		)
	default:
		entMeta.Merge(structs.DefaultEnterpriseMetaInDefaultPartition())
	}

	// Use the meta to fill in the ACL authorization context
	entMeta.FillAuthzContext(authzContext)

	return result, err
}

func filterACLWithAuthorizer(logger hclog.Logger, authorizer acl.Authorizer, subj interface{}) {
	aclfilter.New(authorizer, logger).Filter(subj)
}

// filterACL uses the ACLResolver to resolve the token in an acl.Authorizer,
// then uses the acl.Authorizer to filter subj. Any entities in subj that are
// not authorized for read access will be removed from subj.
func filterACL(r *ACLResolver, tokenSecretID string, subj interface{}) error {
	// Get the ACL from the token
	authorizer, err := r.ResolveToken(tokenSecretID)
	if err != nil {
		return err
	}
	filterACLWithAuthorizer(r.logger, authorizer, subj)
	return nil
}

type partitionInfoNoop struct{}

func (p *partitionInfoNoop) ExportsForPartition(partition string) acl.ExportedServices {
	return acl.ExportedServices{}
}

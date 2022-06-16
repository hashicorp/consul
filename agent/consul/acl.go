package consul

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
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
	// anonymousToken is the token ID we re-write to if there is no token ID
	// provided.
	anonymousToken = "anonymous"

	// redactedToken is shown in structures with embedded tokens when they
	// are not allowed to be displayed.
	redactedToken = "<hidden>"

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
	// TODO: separate methods for each RPC call (there are 4)
	RPC(method string, args interface{}, reply interface{}) error
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
//   Remote resolution can be done synchronously or asynchronously depending
//   on the ACLDownPolicy in the Config passed to the resolver.
//
//   When the down policy is set to async-cache and we have already cached values
//   then go routines will be spawned to perform the RPCs in the background
//   and then will update the cache with either the positive or negative result.
//
//   When the down policy is set to extend-cache or the token/policy/role is not already
//   cached then the same go routines are spawned to do the RPCs in the background.
//   However in this mode channels are created to receive the results of the RPC
//   and are registered with the resolver. Those channels are immediately read/blocked
//   upon.
//
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
	`, nodeName), acl.SyntaxCurrent, &conf, entMeta.ToEnterprisePolicyMeta())
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
	err := r.backend.RPC("ACL.TokenRead", &req, &resp)
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
	err := r.backend.RPC("ACL.PolicyResolve", &req, &resp)
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
	err := r.backend.RPC("ACL.RoleResolve", &req, &resp)
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
					"accessorID", accessorID,
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
					"accessorID", accessorID,
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

	return r.resolveLocallyManagedEnterpriseToken(token)
}

// ResolveToken to an acl.Authorizer and structs.ACLIdentity. The acl.Authorizer
// can be used to check permissions granted to the token, and the ACLIdentity
// describes the token and any defaults applied to it.
func (r *ACLResolver) ResolveToken(token string) (resolver.Result, error) {
	if !r.ACLsEnabled() {
		return resolver.Result{Authorizer: acl.ManageAll()}, nil
	}

	if acl.RootAuthorizer(token) != nil {
		return resolver.Result{}, acl.ErrRootDenied
	}

	// handle the anonymous token
	if token == "" {
		token = anonymousToken
	}

	if ident, authz, ok := r.resolveLocallyManagedToken(token); ok {
		return resolver.Result{Authorizer: authz, ACLIdentity: ident}, nil
	}

	defer metrics.MeasureSince([]string{"acl", "ResolveToken"}, time.Now())

	identity, policies, err := r.resolveTokenToIdentityAndPolicies(token)
	if err != nil {
		r.handleACLDisabledError(err)
		if IsACLRemoteError(err) {
			r.logger.Error("Error resolving token", "error", err)
			ident := &missingIdentity{reason: "primary-dc-down", token: token}
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

// TODO(peering): fix all calls to use the new signature and rename it back
func (r *ACLResolver) ResolveTokenAndDefaultMeta(
	token string,
	entMeta *acl.EnterpriseMeta,
	authzContext *acl.AuthorizerContext,
) (resolver.Result, error) {
	return r.ResolveTokenAndDefaultMetaWithPeerName(token, entMeta, structs.DefaultPeerKeyword, authzContext)
}

func (r *ACLResolver) ResolveTokenAndDefaultMetaWithPeerName(
	token string,
	entMeta *acl.EnterpriseMeta,
	peerName string,
	authzContext *acl.AuthorizerContext,
) (resolver.Result, error) {
	result, err := r.ResolveToken(token)
	if err != nil {
		return resolver.Result{}, err
	}

	if entMeta == nil {
		entMeta = &acl.EnterpriseMeta{}
	}

	// Default the EnterpriseMeta based on the Tokens meta or actual defaults
	// in the case of unknown identity
	switch {
	case peerName == "" && result.ACLIdentity != nil:
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

// aclFilter is used to filter results from our state store based on ACL rules
// configured for the provided token.
type aclFilter struct {
	authorizer acl.Authorizer
	logger     hclog.Logger
}

// newACLFilter constructs a new aclFilter.
func newACLFilter(authorizer acl.Authorizer, logger hclog.Logger) *aclFilter {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{})
	}
	return &aclFilter{
		authorizer: authorizer,
		logger:     logger,
	}
}

// allowNode is used to determine if a node is accessible for an ACL.
func (f *aclFilter) allowNode(node string, ent *acl.AuthorizerContext) bool {
	return f.authorizer.NodeRead(node, ent) == acl.Allow
}

// allowNode is used to determine if the gateway and service are accessible for an ACL
func (f *aclFilter) allowGateway(gs *structs.GatewayService) bool {
	var authzContext acl.AuthorizerContext

	// Need read on service and gateway. Gateway may have different EnterpriseMeta so we fill authzContext twice
	gs.Gateway.FillAuthzContext(&authzContext)
	if !f.allowService(gs.Gateway.Name, &authzContext) {
		return false
	}

	gs.Service.FillAuthzContext(&authzContext)
	if !f.allowService(gs.Service.Name, &authzContext) {
		return false
	}
	return true
}

// allowService is used to determine if a service is accessible for an ACL.
func (f *aclFilter) allowService(service string, ent *acl.AuthorizerContext) bool {
	if service == "" {
		return true
	}

	return f.authorizer.ServiceRead(service, ent) == acl.Allow
}

// allowSession is used to determine if a session for a node is accessible for
// an ACL.
func (f *aclFilter) allowSession(node string, ent *acl.AuthorizerContext) bool {
	return f.authorizer.SessionRead(node, ent) == acl.Allow
}

// filterHealthChecks is used to filter a set of health checks down based on
// the configured ACL rules for a token. Returns true if any elements were
// removed.
func (f *aclFilter) filterHealthChecks(checks *structs.HealthChecks) bool {
	hc := *checks
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(hc); i++ {
		check := hc[i]
		check.FillAuthzContext(&authzContext)
		if f.allowNode(check.Node, &authzContext) && f.allowService(check.ServiceName, &authzContext) {
			continue
		}

		f.logger.Debug("dropping check from result due to ACLs", "check", check.CheckID)
		removed = true
		hc = append(hc[:i], hc[i+1:]...)
		i--
	}
	*checks = hc
	return removed
}

// filterServices is used to filter a set of services based on ACLs. Returns
// true if any elements were removed.
func (f *aclFilter) filterServices(services structs.Services, entMeta *acl.EnterpriseMeta) bool {
	var authzContext acl.AuthorizerContext
	entMeta.FillAuthzContext(&authzContext)

	var removed bool

	for svc := range services {
		if f.allowService(svc, &authzContext) {
			continue
		}
		f.logger.Debug("dropping service from result due to ACLs", "service", svc)
		removed = true
		delete(services, svc)
	}

	return removed
}

// filterServiceNodes is used to filter a set of nodes for a given service
// based on the configured ACL rules. Returns true if any elements were removed.
func (f *aclFilter) filterServiceNodes(nodes *structs.ServiceNodes) bool {
	sn := *nodes
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(sn); i++ {
		node := sn[i]

		node.FillAuthzContext(&authzContext)
		if f.allowNode(node.Node, &authzContext) && f.allowService(node.ServiceName, &authzContext) {
			continue
		}
		removed = true
		f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node.Node, &node.EnterpriseMeta))
		sn = append(sn[:i], sn[i+1:]...)
		i--
	}
	*nodes = sn
	return removed
}

// filterNodeServices is used to filter services on a given node base on ACLs.
// Returns true if any elements were removed
func (f *aclFilter) filterNodeServices(services **structs.NodeServices) bool {
	if *services == nil {
		return false
	}

	var authzContext acl.AuthorizerContext
	(*services).Node.FillAuthzContext(&authzContext)
	if !f.allowNode((*services).Node.Node, &authzContext) {
		*services = nil
		return true
	}

	var removed bool
	for svcName, svc := range (*services).Services {
		svc.FillAuthzContext(&authzContext)

		if f.allowNode((*services).Node.Node, &authzContext) && f.allowService(svcName, &authzContext) {
			continue
		}
		f.logger.Debug("dropping service from result due to ACLs", "service", svc.CompoundServiceID())
		removed = true
		delete((*services).Services, svcName)
	}

	return removed
}

// filterNodeServices is used to filter services on a given node base on ACLs.
// Returns true if any elements were removed.
func (f *aclFilter) filterNodeServiceList(services *structs.NodeServiceList) bool {
	if services.Node == nil {
		return false
	}

	var authzContext acl.AuthorizerContext
	services.Node.FillAuthzContext(&authzContext)
	if !f.allowNode(services.Node.Node, &authzContext) {
		*services = structs.NodeServiceList{}
		return true
	}

	var removed bool
	svcs := services.Services
	for i := 0; i < len(svcs); i++ {
		svc := svcs[i]
		svc.FillAuthzContext(&authzContext)

		if f.allowService(svc.Service, &authzContext) {
			continue
		}

		f.logger.Debug("dropping service from result due to ACLs", "service", svc.CompoundServiceID())
		svcs = append(svcs[:i], svcs[i+1:]...)
		i--
		removed = true
	}
	services.Services = svcs

	return removed
}

// filterCheckServiceNodes is used to filter nodes based on ACL rules. Returns
// true if any elements were removed.
func (f *aclFilter) filterCheckServiceNodes(nodes *structs.CheckServiceNodes) bool {
	csn := *nodes
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(csn); i++ {
		node := csn[i]
		node.Service.FillAuthzContext(&authzContext)
		if f.allowNode(node.Node.Node, &authzContext) && f.allowService(node.Service.Service, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node.Node.Node, node.Node.GetEnterpriseMeta()))
		removed = true
		csn = append(csn[:i], csn[i+1:]...)
		i--
	}
	*nodes = csn
	return removed
}

// filterServiceTopology is used to filter upstreams/downstreams based on ACL rules.
// this filter is unlike others in that it also returns whether the result was filtered by ACLs
func (f *aclFilter) filterServiceTopology(topology *structs.ServiceTopology) bool {
	filteredUpstreams := f.filterCheckServiceNodes(&topology.Upstreams)
	filteredDownstreams := f.filterCheckServiceNodes(&topology.Downstreams)
	return filteredUpstreams || filteredDownstreams
}

// filterDatacenterCheckServiceNodes is used to filter nodes based on ACL rules.
// Returns true if any elements are removed.
func (f *aclFilter) filterDatacenterCheckServiceNodes(datacenterNodes *map[string]structs.CheckServiceNodes) bool {
	dn := *datacenterNodes
	out := make(map[string]structs.CheckServiceNodes)
	var removed bool
	for dc := range dn {
		nodes := dn[dc]
		if f.filterCheckServiceNodes(&nodes) {
			removed = true
		}
		if len(nodes) > 0 {
			out[dc] = nodes
		}
	}
	*datacenterNodes = out
	return removed
}

// filterSessions is used to filter a set of sessions based on ACLs. Returns
// true if any elements were removed.
func (f *aclFilter) filterSessions(sessions *structs.Sessions) bool {
	s := *sessions

	var removed bool
	for i := 0; i < len(s); i++ {
		session := s[i]

		var entCtx acl.AuthorizerContext
		session.FillAuthzContext(&entCtx)

		if f.allowSession(session.Node, &entCtx) {
			continue
		}
		removed = true
		f.logger.Debug("dropping session from result due to ACLs", "session", session.ID)
		s = append(s[:i], s[i+1:]...)
		i--
	}
	*sessions = s
	return removed
}

// filterCoordinates is used to filter nodes in a coordinate dump based on ACL
// rules. Returns true if any elements were removed.
func (f *aclFilter) filterCoordinates(coords *structs.Coordinates) bool {
	c := *coords
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(c); i++ {
		c[i].FillAuthzContext(&authzContext)
		node := c[i].Node
		if f.allowNode(node, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node, c[i].GetEnterpriseMeta()))
		removed = true
		c = append(c[:i], c[i+1:]...)
		i--
	}
	*coords = c
	return removed
}

// filterIntentions is used to filter intentions based on ACL rules.
// We prune entries the user doesn't have access to, and we redact any tokens
// if the user doesn't have a management token. Returns true if any elements
// were removed.
func (f *aclFilter) filterIntentions(ixns *structs.Intentions) bool {
	ret := make(structs.Intentions, 0, len(*ixns))
	var removed bool
	for _, ixn := range *ixns {
		if !ixn.CanRead(f.authorizer) {
			removed = true
			f.logger.Debug("dropping intention from result due to ACLs", "intention", ixn.ID)
			continue
		}

		ret = append(ret, ixn)
	}

	*ixns = ret
	return removed
}

// filterNodeDump is used to filter through all parts of a node dump and
// remove elements the provided ACL token cannot access. Returns true if
// any elements were removed.
func (f *aclFilter) filterNodeDump(dump *structs.NodeDump) bool {
	nd := *dump

	var authzContext acl.AuthorizerContext
	var removed bool
	for i := 0; i < len(nd); i++ {
		info := nd[i]

		// Filter nodes
		info.FillAuthzContext(&authzContext)
		if node := info.Node; !f.allowNode(node, &authzContext) {
			f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node, info.GetEnterpriseMeta()))
			removed = true
			nd = append(nd[:i], nd[i+1:]...)
			i--
			continue
		}

		// Filter services
		for j := 0; j < len(info.Services); j++ {
			svc := info.Services[j].Service
			info.Services[j].FillAuthzContext(&authzContext)
			if f.allowNode(info.Node, &authzContext) && f.allowService(svc, &authzContext) {
				continue
			}
			f.logger.Debug("dropping service from result due to ACLs", "service", svc)
			removed = true
			info.Services = append(info.Services[:j], info.Services[j+1:]...)
			j--
		}

		// Filter checks
		for j := 0; j < len(info.Checks); j++ {
			chk := info.Checks[j]
			chk.FillAuthzContext(&authzContext)
			if f.allowNode(info.Node, &authzContext) && f.allowService(chk.ServiceName, &authzContext) {
				continue
			}
			f.logger.Debug("dropping check from result due to ACLs", "check", chk.CheckID)
			removed = true
			info.Checks = append(info.Checks[:j], info.Checks[j+1:]...)
			j--
		}
	}
	*dump = nd
	return removed
}

// filterServiceDump is used to filter nodes based on ACL rules. Returns true
// if any elements were removed.
func (f *aclFilter) filterServiceDump(services *structs.ServiceDump) bool {
	svcs := *services
	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(svcs); i++ {
		service := svcs[i]

		if f.allowGateway(service.GatewayService) {
			// ServiceDump might only have gateway config and no node information
			if service.Node == nil {
				continue
			}

			service.Service.FillAuthzContext(&authzContext)
			if f.allowNode(service.Node.Node, &authzContext) {
				continue
			}
		}

		f.logger.Debug("dropping service from result due to ACLs", "service", service.GatewayService.Service)
		removed = true
		svcs = append(svcs[:i], svcs[i+1:]...)
		i--
	}
	*services = svcs
	return removed
}

// filterNodes is used to filter through all parts of a node list and remove
// elements the provided ACL token cannot access. Returns true if any elements
// were removed.
func (f *aclFilter) filterNodes(nodes *structs.Nodes) bool {
	n := *nodes

	var authzContext acl.AuthorizerContext
	var removed bool

	for i := 0; i < len(n); i++ {
		n[i].FillAuthzContext(&authzContext)
		node := n[i].Node
		if f.allowNode(node, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", structs.NodeNameString(node, n[i].GetEnterpriseMeta()))
		removed = true
		n = append(n[:i], n[i+1:]...)
		i--
	}
	*nodes = n
	return removed
}

// redactPreparedQueryTokens will redact any tokens unless the client has a
// management token. This eases the transition to delegated authority over
// prepared queries, since it was easy to capture management tokens in Consul
// 0.6.3 and earlier, and we don't want to willy-nilly show those. This does
// have the limitation of preventing delegated non-management users from seeing
// captured tokens, but they can at least see whether or not a token is set.
func (f *aclFilter) redactPreparedQueryTokens(query **structs.PreparedQuery) {
	// Management tokens can see everything with no filtering.
	var authzContext acl.AuthorizerContext
	structs.DefaultEnterpriseMetaInDefaultPartition().FillAuthzContext(&authzContext)
	if f.authorizer.ACLWrite(&authzContext) == acl.Allow {
		return
	}

	// Let the user see if there's a blank token, otherwise we need
	// to redact it, since we know they don't have a management
	// token.
	if (*query).Token != "" {
		// Redact the token, using a copy of the query structure
		// since we could be pointed at a live instance from the
		// state store so it's not safe to modify it. Note that
		// this clone will still point to things like underlying
		// arrays in the original, but for modifying just the
		// token it will be safe to use.
		clone := *(*query)
		clone.Token = redactedToken
		*query = &clone
	}
}

// filterPreparedQueries is used to filter prepared queries based on ACL rules.
// We prune entries the user doesn't have access to, and we redact any tokens
// if the user doesn't have a management token. Returns true if any (named)
// queries were removed - un-named queries are meant to be ephemeral and can
// only be enumerated by a management token
func (f *aclFilter) filterPreparedQueries(queries *structs.PreparedQueries) bool {
	var authzContext acl.AuthorizerContext
	structs.DefaultEnterpriseMetaInDefaultPartition().FillAuthzContext(&authzContext)
	// Management tokens can see everything with no filtering.
	// TODO  is this check even necessary - this looks like a search replace from
	// the 1.4 ACL rewrite. The global-management token will provide unrestricted query privileges
	// so asking for ACLWrite should be unnecessary.
	if f.authorizer.ACLWrite(&authzContext) == acl.Allow {
		return false
	}

	// Otherwise, we need to see what the token has access to.
	var namedQueriesRemoved bool
	ret := make(structs.PreparedQueries, 0, len(*queries))
	for _, query := range *queries {
		// If no prefix ACL applies to this query then filter it, since
		// we know at this point the user doesn't have a management
		// token, otherwise see what the policy says.
		prefix, hasName := query.GetACLPrefix()
		switch {
		case hasName && f.authorizer.PreparedQueryRead(prefix, &authzContext) != acl.Allow:
			namedQueriesRemoved = true
			fallthrough
		case !hasName:
			f.logger.Debug("dropping prepared query from result due to ACLs", "query", query.ID)
			continue
		}

		// Redact any tokens if necessary. We make a copy of just the
		// pointer so we don't mess with the caller's slice.
		final := query
		f.redactPreparedQueryTokens(&final)
		ret = append(ret, final)
	}
	*queries = ret
	return namedQueriesRemoved
}

func (f *aclFilter) filterToken(token **structs.ACLToken) {
	var entCtx acl.AuthorizerContext
	if token == nil || *token == nil || f == nil {
		return
	}

	(*token).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*token = nil
	} else if f.authorizer.ACLWrite(&entCtx) != acl.Allow {
		// no write permissions - redact secret
		clone := *(*token)
		clone.SecretID = redactedToken
		*token = &clone
	}
}

func (f *aclFilter) filterTokens(tokens *structs.ACLTokens) {
	ret := make(structs.ACLTokens, 0, len(*tokens))
	for _, token := range *tokens {
		final := token
		f.filterToken(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*tokens = ret
}

func (f *aclFilter) filterTokenStub(token **structs.ACLTokenListStub) {
	var entCtx acl.AuthorizerContext
	if token == nil || *token == nil || f == nil {
		return
	}

	(*token).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		*token = nil
	} else if f.authorizer.ACLWrite(&entCtx) != acl.Allow {
		// no write permissions - redact secret
		clone := *(*token)
		clone.SecretID = redactedToken
		*token = &clone
	}
}

func (f *aclFilter) filterTokenStubs(tokens *[]*structs.ACLTokenListStub) {
	ret := make(structs.ACLTokenListStubs, 0, len(*tokens))
	for _, token := range *tokens {
		final := token
		f.filterTokenStub(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*tokens = ret
}

func (f *aclFilter) filterPolicy(policy **structs.ACLPolicy) {
	var entCtx acl.AuthorizerContext
	if policy == nil || *policy == nil || f == nil {
		return
	}

	(*policy).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*policy = nil
	}
}

func (f *aclFilter) filterPolicies(policies *structs.ACLPolicies) {
	ret := make(structs.ACLPolicies, 0, len(*policies))
	for _, policy := range *policies {
		final := policy
		f.filterPolicy(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*policies = ret
}

func (f *aclFilter) filterRole(role **structs.ACLRole) {
	var entCtx acl.AuthorizerContext
	if role == nil || *role == nil || f == nil {
		return
	}

	(*role).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*role = nil
	}
}

func (f *aclFilter) filterRoles(roles *structs.ACLRoles) {
	ret := make(structs.ACLRoles, 0, len(*roles))
	for _, role := range *roles {
		final := role
		f.filterRole(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*roles = ret
}

func (f *aclFilter) filterBindingRule(rule **structs.ACLBindingRule) {
	var entCtx acl.AuthorizerContext
	if rule == nil || *rule == nil || f == nil {
		return
	}

	(*rule).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*rule = nil
	}
}

func (f *aclFilter) filterBindingRules(rules *structs.ACLBindingRules) {
	ret := make(structs.ACLBindingRules, 0, len(*rules))
	for _, rule := range *rules {
		final := rule
		f.filterBindingRule(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*rules = ret
}

func (f *aclFilter) filterAuthMethod(method **structs.ACLAuthMethod) {
	var entCtx acl.AuthorizerContext
	if method == nil || *method == nil || f == nil {
		return
	}

	(*method).FillAuthzContext(&entCtx)

	if f.authorizer.ACLRead(&entCtx) != acl.Allow {
		// no permissions to read
		*method = nil
	}
}

func (f *aclFilter) filterAuthMethods(methods *structs.ACLAuthMethods) {
	ret := make(structs.ACLAuthMethods, 0, len(*methods))
	for _, method := range *methods {
		final := method
		f.filterAuthMethod(&final)
		if final != nil {
			ret = append(ret, final)
		}
	}
	*methods = ret
}

func (f *aclFilter) filterServiceList(services *structs.ServiceList) bool {
	ret := make(structs.ServiceList, 0, len(*services))
	var removed bool
	for _, svc := range *services {
		var authzContext acl.AuthorizerContext

		svc.FillAuthzContext(&authzContext)

		if f.authorizer.ServiceRead(svc.Name, &authzContext) != acl.Allow {
			removed = true
			sid := structs.NewServiceID(svc.Name, &svc.EnterpriseMeta)
			f.logger.Debug("dropping service from result due to ACLs", "service", sid.String())
			continue
		}

		ret = append(ret, svc)
	}

	*services = ret
	return removed
}

// filterGatewayServices is used to filter gateway to service mappings based on ACL rules.
// Returns true if any elements were removed.
func (f *aclFilter) filterGatewayServices(mappings *structs.GatewayServices) bool {
	ret := make(structs.GatewayServices, 0, len(*mappings))
	var removed bool
	for _, s := range *mappings {
		// This filter only checks ServiceRead on the linked service.
		// ServiceRead on the gateway is checked in the GatewayServices endpoint before filtering.
		var authzContext acl.AuthorizerContext
		s.Service.FillAuthzContext(&authzContext)

		if f.authorizer.ServiceRead(s.Service.Name, &authzContext) != acl.Allow {
			f.logger.Debug("dropping service from result due to ACLs", "service", s.Service.String())
			removed = true
			continue
		}
		ret = append(ret, s)
	}
	*mappings = ret
	return removed
}

func filterACLWithAuthorizer(logger hclog.Logger, authorizer acl.Authorizer, subj interface{}) {
	if authorizer == nil {
		return
	}
	filt := newACLFilter(authorizer, logger)

	switch v := subj.(type) {
	case *structs.CheckServiceNodes:
		filt.filterCheckServiceNodes(v)

	case *structs.IndexedCheckServiceNodes:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterCheckServiceNodes(&v.Nodes)

	case *structs.PreparedQueryExecuteResponse:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterCheckServiceNodes(&v.Nodes)

	case *structs.IndexedServiceTopology:
		filtered := filt.filterServiceTopology(v.ServiceTopology)
		if filtered {
			v.FilteredByACLs = true
			v.QueryMeta.ResultsFilteredByACLs = true
		}

	case *structs.DatacenterIndexedCheckServiceNodes:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterDatacenterCheckServiceNodes(&v.DatacenterNodes)

	case *structs.IndexedCoordinates:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterCoordinates(&v.Coordinates)

	case *structs.IndexedHealthChecks:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterHealthChecks(&v.HealthChecks)

	case *structs.IndexedIntentions:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterIntentions(&v.Intentions)

	case *structs.IndexedNodeDump:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterNodeDump(&v.Dump)

	case *structs.IndexedServiceDump:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterServiceDump(&v.Dump)

	case *structs.IndexedNodes:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterNodes(&v.Nodes)

	case *structs.IndexedNodeServices:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterNodeServices(&v.NodeServices)

	case *structs.IndexedNodeServiceList:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterNodeServiceList(&v.NodeServices)

	case *structs.IndexedServiceNodes:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterServiceNodes(&v.ServiceNodes)

	case *structs.IndexedServices:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterServices(v.Services, &v.EnterpriseMeta)

	case *structs.IndexedSessions:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterSessions(&v.Sessions)

	case *structs.IndexedPreparedQueries:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterPreparedQueries(&v.Queries)

	case **structs.PreparedQuery:
		filt.redactPreparedQueryTokens(v)

	case *structs.ACLTokens:
		filt.filterTokens(v)
	case **structs.ACLToken:
		filt.filterToken(v)
	case *[]*structs.ACLTokenListStub:
		filt.filterTokenStubs(v)
	case **structs.ACLTokenListStub:
		filt.filterTokenStub(v)

	case *structs.ACLPolicies:
		filt.filterPolicies(v)
	case **structs.ACLPolicy:
		filt.filterPolicy(v)

	case *structs.ACLRoles:
		filt.filterRoles(v)
	case **structs.ACLRole:
		filt.filterRole(v)

	case *structs.ACLBindingRules:
		filt.filterBindingRules(v)
	case **structs.ACLBindingRule:
		filt.filterBindingRule(v)

	case *structs.ACLAuthMethods:
		filt.filterAuthMethods(v)
	case **structs.ACLAuthMethod:
		filt.filterAuthMethod(v)

	case *structs.IndexedServiceList:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterServiceList(&v.Services)

	case *structs.IndexedExportedServiceList:
		for peer, peerServices := range v.Services {
			v.QueryMeta.ResultsFilteredByACLs = filt.filterServiceList(&peerServices)
			if len(peerServices) == 0 {
				delete(v.Services, peer)
			} else {
				v.Services[peer] = peerServices
			}
		}

	case *structs.IndexedGatewayServices:
		v.QueryMeta.ResultsFilteredByACLs = filt.filterGatewayServices(&v.Services)

	case *structs.IndexedNodesWithGateways:
		if filt.filterCheckServiceNodes(&v.Nodes) {
			v.QueryMeta.ResultsFilteredByACLs = true
		}
		if filt.filterGatewayServices(&v.Gateways) {
			v.QueryMeta.ResultsFilteredByACLs = true
		}

	default:
		panic(fmt.Errorf("Unhandled type passed to ACL filter: %T %#v", subj, subj))
	}
}

// filterACL uses the ACLResolver to resolve the token in an acl.Authorizer,
// then uses the acl.Authorizer to filter subj. Any entities in subj that are
// not authorized for read access will be removed from subj.
func filterACL(r *ACLResolver, token string, subj interface{}) error {
	// Get the ACL from the token
	authorizer, err := r.ResolveToken(token)
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

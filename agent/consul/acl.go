package consul

import (
	"fmt"
	"sort"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

// These must be kept in sync with the constants in command/agent/acl.go.
const (
	// anonymousToken is the token ID we re-write to if there is no token ID
	// provided.
	anonymousToken = "anonymous"

	// redactedToken is shown in structures with embedded tokens when they
	// are not allowed to be displayed.
	redactedToken = "<hidden>"

	// aclUpgradeBatchSize controls how many tokens we look at during each round of upgrading. Individual raft logs
	// will be further capped using the aclBatchUpsertSize. This limit just prevents us from creating a single slice
	// with all tokens in it.
	aclUpgradeBatchSize = 128

	// aclUpgradeRateLimit is the number of batch upgrade requests per second allowed.
	aclUpgradeRateLimit rate.Limit = 1.0

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

	// DEPRECATED (ACL-Legacy-Compat) aclModeCheck* are all only for legacy usage
	// aclModeCheckMinInterval is the minimum amount of time between checking if the
	// agent should be using the new or legacy ACL system. All the places it is
	// currently used will backoff as it detects that it is remaining in legacy mode.
	// However the initial min value is kept small so that new cluster creation
	// can enter into new ACL mode quickly.
	aclModeCheckMinInterval = 50 * time.Millisecond

	// aclModeCheckMaxInterval controls the maximum interval for how often the agent
	// checks if it should be using the new or legacy ACL system.
	aclModeCheckMaxInterval = 30 * time.Second

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

func (id *missingIdentity) EmbeddedPolicy() *structs.ACLPolicy {
	return nil
}

func (id *missingIdentity) ServiceIdentityList() []*structs.ACLServiceIdentity {
	return nil
}

func (id *missingIdentity) IsExpired(asOf time.Time) bool {
	return false
}

func (id *missingIdentity) EnterpriseMetadata() *structs.EnterpriseMeta {
	return structs.DefaultEnterpriseMeta()
}

func minTTL(a time.Duration, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
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

type ACLResolverDelegate interface {
	ACLsEnabled() bool
	ACLDatacenter(legacy bool) string
	UseLegacyACLs() bool
	ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error)
	ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error)
	ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error)
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
	Config *Config
	Logger hclog.Logger

	// CacheConfig is a pass through configuration for ACL cache limits
	CacheConfig *structs.ACLCachesConfig

	// Delegate that implements some helper functionality that is server/client specific
	Delegate ACLResolverDelegate

	// AutoDisable indicates that RPC responses should be checked and if they indicate ACLs are disabled
	// remotely then disable them locally as well. This is particularly useful for the client agent
	// so that it can detect when the servers have gotten ACLs enabled.
	AutoDisable bool

	// ACLConfig is the configuration necessary to pass through to the acl package when creating authorizers
	// and when authorizing access
	ACLConfig *acl.Config
}

// ACLResolver is the type to handle all your token and policy resolution needs.
//
// Supports:
//   - Resolving tokens locally via the ACLResolverDelegate
//   - Resolving policies locally via the ACLResolverDelegate
//   - Resolving roles locally via the ACLResolverDelegate
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
	config *Config
	logger hclog.Logger

	delegate ACLResolverDelegate
	aclConf  *acl.Config

	cache         *structs.ACLCaches
	identityGroup singleflight.Group
	policyGroup   singleflight.Group
	roleGroup     singleflight.Group
	legacyGroup   singleflight.Group

	down acl.Authorizer

	autoDisable  bool
	disabled     time.Time
	disabledLock sync.RWMutex
}

func NewACLResolver(config *ACLResolverConfig) (*ACLResolver, error) {
	if config == nil {
		return nil, fmt.Errorf("ACL Resolver must be initialized with a config")
	}

	if config.Config == nil {
		return nil, fmt.Errorf("ACLResolverConfig.Config must not be nil")
	}

	if config.Delegate == nil {
		return nil, fmt.Errorf("ACL Resolver must be initialized with a valid delegate")
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
		// Leave the down policy as nil to signal this.
	default:
		return nil, fmt.Errorf("invalid ACL down policy %q", config.Config.ACLDownPolicy)
	}

	return &ACLResolver{
		config:      config.Config,
		logger:      config.Logger.Named(logging.ACL),
		delegate:    config.Delegate,
		aclConf:     config.ACLConfig,
		cache:       cache,
		autoDisable: config.AutoDisable,
		down:        down,
	}, nil
}

func (r *ACLResolver) Close() {
	r.aclConf.Close()
}

func (r *ACLResolver) fetchAndCacheTokenLegacy(token string, cached *structs.AuthorizerCacheEntry) (acl.Authorizer, error) {
	req := structs.ACLPolicyResolveLegacyRequest{
		Datacenter: r.delegate.ACLDatacenter(true),
		ACL:        token,
	}

	cacheTTL := r.config.ACLTokenTTL
	if cached != nil {
		cacheTTL = cached.TTL
	}

	var reply structs.ACLPolicyResolveLegacyResponse
	err := r.delegate.RPC("ACL.GetPolicy", &req, &reply)
	if err == nil {
		parent := acl.RootAuthorizer(reply.Parent)
		if parent == nil {
			var authorizer acl.Authorizer
			if cached != nil {
				authorizer = cached.Authorizer
			}
			r.cache.PutAuthorizerWithTTL(token, authorizer, cacheTTL)
			return authorizer, acl.ErrInvalidParent
		}

		var policies []*acl.Policy
		policy := reply.Policy
		if policy != nil {
			policies = append(policies, policy.ConvertFromLegacy())
		}

		authorizer, err := acl.NewPolicyAuthorizerWithDefaults(parent, policies, r.aclConf)

		r.cache.PutAuthorizerWithTTL(token, authorizer, reply.TTL)
		return authorizer, err
	}

	if acl.IsErrNotFound(err) {
		// Make sure to remove from the cache if it was deleted
		r.cache.PutAuthorizerWithTTL(token, nil, cacheTTL)
		return nil, acl.ErrNotFound

	}

	// some other RPC error
	switch r.config.ACLDownPolicy {
	case "allow":
		r.cache.PutAuthorizerWithTTL(token, acl.AllowAll(), cacheTTL)
		return acl.AllowAll(), nil
	case "async-cache", "extend-cache":
		if cached != nil {
			r.cache.PutAuthorizerWithTTL(token, cached.Authorizer, cacheTTL)
			return cached.Authorizer, nil
		}
		fallthrough
	default:
		r.cache.PutAuthorizerWithTTL(token, acl.DenyAll(), cacheTTL)
		return acl.DenyAll(), nil
	}
}

func (r *ACLResolver) resolveTokenLegacy(token string) (structs.ACLIdentity, acl.Authorizer, error) {
	defer metrics.MeasureSince([]string{"acl", "resolveTokenLegacy"}, time.Now())

	// Attempt to resolve locally first (local results are not cached)
	// This is only useful for servers where either legacy replication is being
	// done or the server is within the primary datacenter.
	if done, identity, err := r.delegate.ResolveIdentityFromToken(token); done {
		if err == nil && identity != nil {
			policies, err := r.resolvePoliciesForIdentity(identity)
			if err != nil {
				return identity, nil, err
			}

			authz, err := policies.Compile(r.cache, r.aclConf)
			if err != nil {
				return identity, nil, err
			}

			return identity, acl.NewChainedAuthorizer([]acl.Authorizer{authz, acl.RootAuthorizer(r.config.ACLDefaultPolicy)}), nil
		}

		return nil, nil, err
	}

	identity := &missingIdentity{
		reason: "legacy-token",
		token:  token,
	}

	// Look in the cache prior to making a RPC request
	entry := r.cache.GetAuthorizer(token)

	if entry != nil && entry.Age() <= minTTL(entry.TTL, r.config.ACLTokenTTL) {
		metrics.IncrCounter([]string{"acl", "token", "cache_hit"}, 1)
		if entry.Authorizer != nil {
			return identity, entry.Authorizer, nil
		}
		return identity, nil, acl.ErrNotFound
	}

	metrics.IncrCounter([]string{"acl", "token", "cache_miss"}, 1)

	// Resolve the token in the background and wait on the result if we must
	waitChan := r.legacyGroup.DoChan(token, func() (interface{}, error) {
		authorizer, err := r.fetchAndCacheTokenLegacy(token, entry)
		return authorizer, err
	})

	waitForResult := entry == nil || r.config.ACLDownPolicy != "async-cache"
	if !waitForResult {
		// waitForResult being false requires the cacheEntry to not be nil
		if entry.Authorizer != nil {
			return identity, entry.Authorizer, nil
		}
		return identity, nil, acl.ErrNotFound
	}

	// block waiting for the async RPC to finish.
	res := <-waitChan

	var authorizer acl.Authorizer
	if res.Val != nil { // avoid a nil-not-nil bug
		authorizer = res.Val.(acl.Authorizer)
	}

	return identity, authorizer, res.Err
}

func (r *ACLResolver) fetchAndCacheIdentityFromToken(token string, cached *structs.IdentityCacheEntry) (structs.ACLIdentity, error) {
	cacheID := tokenSecretCacheID(token)

	req := structs.ACLTokenGetRequest{
		Datacenter:  r.delegate.ACLDatacenter(false),
		TokenID:     token,
		TokenIDType: structs.ACLTokenSecret,
		QueryOptions: structs.QueryOptions{
			Token:      token,
			AllowStale: true,
		},
	}

	var resp structs.ACLTokenResponse
	err := r.delegate.RPC("ACL.TokenRead", &req, &resp)
	if err == nil {
		if resp.Token == nil {
			r.cache.PutIdentity(cacheID, nil)
			return nil, acl.ErrNotFound
		} else {
			r.cache.PutIdentity(cacheID, resp.Token)
			return resp.Token, nil
		}
	}

	if acl.IsErrNotFound(err) {
		// Make sure to remove from the cache if it was deleted
		r.cache.PutIdentity(cacheID, nil)
		return nil, acl.ErrNotFound

	}

	// some other RPC error
	if cached != nil && (r.config.ACLDownPolicy == "extend-cache" || r.config.ACLDownPolicy == "async-cache") {
		// extend the cache
		r.cache.PutIdentity(cacheID, cached.Identity)
		return cached.Identity, nil
	}

	r.cache.PutIdentity(cacheID, nil)
	return nil, err
}

// resolveIdentityFromToken takes a token secret as a string and returns an ACLIdentity.
// We read the value from ACLResolver's cache if available, and if the read misses
// we initiate an RPC for the value.
func (r *ACLResolver) resolveIdentityFromToken(token string) (structs.ACLIdentity, error) {
	// Attempt to resolve locally first (local results are not cached)
	if done, identity, err := r.delegate.ResolveIdentityFromToken(token); done {
		return identity, err
	}

	// Check the cache before making any RPC requests
	cacheEntry := r.cache.GetIdentity(tokenSecretCacheID(token))
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
		Datacenter: r.delegate.ACLDatacenter(false),
		PolicyIDs:  policyIDs,
		QueryOptions: structs.QueryOptions{
			Token:      identity.SecretToken(),
			AllowStale: true,
		},
	}

	var resp structs.ACLPolicyBatchResponse
	err := r.delegate.RPC("ACL.PolicyResolve", &req, &resp)
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
		Datacenter: r.delegate.ACLDatacenter(false),
		RoleIDs:    roleIDs,
		QueryOptions: structs.QueryOptions{
			Token:      identity.SecretToken(),
			AllowStale: true,
		},
	}

	var resp structs.ACLRoleBatchResponse
	err := r.delegate.RPC("ACL.RoleResolve", &req, &resp)
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
		r.cache.PutIdentity(tokenSecretCacheID(identity.SecretToken()), nil)

		// Do not touch the cache. Getting a top level ACL not found error
		// only indicates that the secret token used in the request
		// no longer exists
		return &policyOrRoleTokenError{acl.ErrNotFound, identity.SecretToken()}
	}

	if acl.IsErrPermissionDenied(err) {
		// invalidate our ID cache so that identity resolution will take place
		// again in the future
		r.cache.RemoveIdentity(tokenSecretCacheID(identity.SecretToken()))

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
	policyIDs := identity.PolicyIDs()
	roleIDs := identity.RoleIDs()
	serviceIdentities := identity.ServiceIdentityList()

	if len(policyIDs) == 0 && len(serviceIdentities) == 0 && len(roleIDs) == 0 {
		policy := identity.EmbeddedPolicy()
		if policy != nil {
			return []*structs.ACLPolicy{policy}, nil
		}

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
	}

	// Now deduplicate any policies or service identities that occur more than once.
	policyIDs = dedupeStringSlice(policyIDs)
	serviceIdentities = dedupeServiceIdentities(serviceIdentities)

	// Generate synthetic policies for all service identities in effect.
	syntheticPolicies := r.synthesizePoliciesForServiceIdentities(serviceIdentities, identity.EnterpriseMetadata())

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

func (r *ACLResolver) synthesizePoliciesForServiceIdentities(serviceIdentities []*structs.ACLServiceIdentity, entMeta *structs.EnterpriseMeta) []*structs.ACLPolicy {
	if len(serviceIdentities) == 0 {
		return nil
	}

	syntheticPolicies := make([]*structs.ACLPolicy, 0, len(serviceIdentities))
	for _, s := range serviceIdentities {
		syntheticPolicies = append(syntheticPolicies, s.SyntheticPolicy(entMeta))
	}

	return syntheticPolicies
}

func dedupeServiceIdentities(in []*structs.ACLServiceIdentity) []*structs.ACLServiceIdentity {
	// From: https://github.com/golang/go/wiki/SliceTricks#in-place-deduplicate-comparable

	if len(in) <= 1 {
		return in
	}

	sort.Slice(in, func(i, j int) bool {
		return in[i].ServiceName < in[j].ServiceName
	})

	j := 0
	for i := 1; i < len(in); i++ {
		if in[j].ServiceName == in[i].ServiceName {
			// Prefer increasing scope.
			if len(in[j].Datacenters) == 0 || len(in[i].Datacenters) == 0 {
				in[j].Datacenters = nil
			} else {
				in[j].Datacenters = mergeStringSlice(in[j].Datacenters, in[i].Datacenters)
			}
			continue
		}
		j++
		in[j] = in[i]
	}

	// Discard the skipped items.
	for i := j + 1; i < len(in); i++ {
		in[i] = nil
	}

	return in[:j+1]
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
		if done, policy, err := r.delegate.ResolvePolicyFromID(policyID); done {
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
		if done, role, err := r.delegate.ResolveRoleFromID(roleID); done {
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

func (r *ACLResolver) resolveTokenToPolicies(token string) (structs.ACLPolicies, error) {
	_, policies, err := r.resolveTokenToIdentityAndPolicies(token)
	return policies, err
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

func (r *ACLResolver) disableACLsWhenUpstreamDisabled(err error) error {
	if !r.autoDisable || err == nil || !acl.IsErrDisabled(err) {
		return err
	}

	r.logger.Debug("ACLs disabled on upstream servers, will retry", "retry_interval", r.config.ACLDisabledTTL)
	r.disabledLock.Lock()
	r.disabled = time.Now().Add(r.config.ACLDisabledTTL)
	r.disabledLock.Unlock()

	return err
}

func (r *ACLResolver) ResolveTokenToIdentityAndAuthorizer(token string) (structs.ACLIdentity, acl.Authorizer, error) {
	if !r.ACLsEnabled() {
		return nil, nil, nil
	}

	if acl.RootAuthorizer(token) != nil {
		return nil, nil, acl.ErrRootDenied
	}

	// handle the anonymous token
	if token == "" {
		token = anonymousToken
	}

	if r.delegate.UseLegacyACLs() {
		identity, authorizer, err := r.resolveTokenLegacy(token)
		return identity, authorizer, r.disableACLsWhenUpstreamDisabled(err)
	}

	defer metrics.MeasureSince([]string{"acl", "ResolveToken"}, time.Now())

	identity, policies, err := r.resolveTokenToIdentityAndPolicies(token)
	if err != nil {
		r.disableACLsWhenUpstreamDisabled(err)
		if IsACLRemoteError(err) {
			r.logger.Error("Error resolving token", "error", err)
			return &missingIdentity{reason: "primary-dc-down", token: token}, r.down, nil
		}

		return nil, nil, err
	}

	// Build the Authorizer
	var chain []acl.Authorizer

	authz, err := policies.Compile(r.cache, r.aclConf)
	if err != nil {
		return nil, nil, err
	}
	chain = append(chain, authz)

	authz, err = r.resolveEnterpriseDefaultsForIdentity(identity)
	if err != nil {
		if IsACLRemoteError(err) {
			r.logger.Error("Error resolving identity defaults", "error", err)
			return identity, r.down, nil
		}
		return nil, nil, err
	} else if authz != nil {
		chain = append(chain, authz)
	}

	chain = append(chain, acl.RootAuthorizer(r.config.ACLDefaultPolicy))
	return identity, acl.NewChainedAuthorizer(chain), nil
}

func (r *ACLResolver) ResolveToken(token string) (acl.Authorizer, error) {
	_, authz, err := r.ResolveTokenToIdentityAndAuthorizer(token)
	return authz, err
}

func (r *ACLResolver) ACLsEnabled() bool {
	// Whether we desire ACLs to be enabled according to configuration
	if !r.delegate.ACLsEnabled() {
		return false
	}

	if r.autoDisable {
		// Whether ACLs are disabled according to RPCs failing with a ACLs Disabled error
		r.disabledLock.RLock()
		defer r.disabledLock.RUnlock()
		return !time.Now().Before(r.disabled)
	}

	return true
}

func (r *ACLResolver) GetMergedPolicyForToken(token string) (*acl.Policy, error) {
	policies, err := r.resolveTokenToPolicies(token)
	if err != nil {
		return nil, err
	}
	if len(policies) == 0 {
		return nil, acl.ErrNotFound
	}

	return policies.Merge(r.cache, r.aclConf)
}

// aclFilter is used to filter results from our state store based on ACL rules
// configured for the provided token.
type aclFilter struct {
	authorizer      acl.Authorizer
	logger          hclog.Logger
	enforceVersion8 bool
}

// newACLFilter constructs a new aclFilter.
func newACLFilter(authorizer acl.Authorizer, logger hclog.Logger, enforceVersion8 bool) *aclFilter {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{})
	}
	return &aclFilter{
		authorizer:      authorizer,
		logger:          logger,
		enforceVersion8: enforceVersion8,
	}
}

// allowNode is used to determine if a node is accessible for an ACL.
func (f *aclFilter) allowNode(node string, ent *acl.AuthorizerContext) bool {
	if !f.enforceVersion8 {
		return true
	}

	return f.authorizer.NodeRead(node, ent) == acl.Allow
}

// allowService is used to determine if a service is accessible for an ACL.
func (f *aclFilter) allowService(service string, ent *acl.AuthorizerContext) bool {
	if service == "" {
		return true
	}

	if !f.enforceVersion8 && service == structs.ConsulServiceID {
		return true
	}
	return f.authorizer.ServiceRead(service, ent) == acl.Allow
}

// allowSession is used to determine if a session for a node is accessible for
// an ACL.
func (f *aclFilter) allowSession(node string, ent *acl.AuthorizerContext) bool {
	if !f.enforceVersion8 {
		return true
	}
	return f.authorizer.SessionRead(node, ent) == acl.Allow
}

// filterHealthChecks is used to filter a set of health checks down based on
// the configured ACL rules for a token.
func (f *aclFilter) filterHealthChecks(checks *structs.HealthChecks) {
	hc := *checks
	var authzContext acl.AuthorizerContext

	for i := 0; i < len(hc); i++ {
		check := hc[i]
		check.FillAuthzContext(&authzContext)
		if f.allowNode(check.Node, &authzContext) && f.allowService(check.ServiceName, &authzContext) {
			continue
		}

		f.logger.Debug("dropping check from result due to ACLs", "check", check.CheckID)
		hc = append(hc[:i], hc[i+1:]...)
		i--
	}
	*checks = hc
}

// filterServices is used to filter a set of services based on ACLs.
func (f *aclFilter) filterServices(services structs.Services, entMeta *structs.EnterpriseMeta) {
	var authzContext acl.AuthorizerContext
	entMeta.FillAuthzContext(&authzContext)

	for svc := range services {
		if f.allowService(svc, &authzContext) {
			continue
		}
		f.logger.Debug("dropping service from result due to ACLs", "service", svc)
		delete(services, svc)
	}
}

// filterServiceNodes is used to filter a set of nodes for a given service
// based on the configured ACL rules.
func (f *aclFilter) filterServiceNodes(nodes *structs.ServiceNodes) {
	sn := *nodes
	var authzContext acl.AuthorizerContext

	for i := 0; i < len(sn); i++ {
		node := sn[i]

		node.FillAuthzContext(&authzContext)
		if f.allowNode(node.Node, &authzContext) && f.allowService(node.ServiceName, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", node.Node)
		sn = append(sn[:i], sn[i+1:]...)
		i--
	}
	*nodes = sn
}

// filterNodeServices is used to filter services on a given node base on ACLs.
func (f *aclFilter) filterNodeServices(services **structs.NodeServices) {
	if *services == nil {
		return
	}

	var authzContext acl.AuthorizerContext
	structs.WildcardEnterpriseMeta().FillAuthzContext(&authzContext)
	if !f.allowNode((*services).Node.Node, &authzContext) {
		*services = nil
		return
	}

	for svcName, svc := range (*services).Services {
		svc.FillAuthzContext(&authzContext)

		if f.allowNode((*services).Node.Node, &authzContext) && f.allowService(svcName, &authzContext) {
			continue
		}
		f.logger.Debug("dropping service from result due to ACLs", "service", svc.CompoundServiceID())
		delete((*services).Services, svcName)
	}
}

// filterNodeServices is used to filter services on a given node base on ACLs.
func (f *aclFilter) filterNodeServiceList(services **structs.NodeServiceList) {
	if services == nil || *services == nil {
		return
	}

	var authzContext acl.AuthorizerContext
	structs.WildcardEnterpriseMeta().FillAuthzContext(&authzContext)
	if !f.allowNode((*services).Node.Node, &authzContext) {
		*services = nil
		return
	}

	svcs := (*services).Services
	modified := false
	for i := 0; i < len(svcs); i++ {
		svc := svcs[i]
		svc.FillAuthzContext(&authzContext)

		if f.allowNode((*services).Node.Node, &authzContext) && f.allowService(svc.Service, &authzContext) {
			continue
		}
		f.logger.Debug("dropping service from result due to ACLs", "service", svc.CompoundServiceID())
		svcs = append(svcs[:i], svcs[i+1:]...)
		i--
		modified = true
	}

	if modified {
		*services = &structs.NodeServiceList{
			Node:     (*services).Node,
			Services: svcs,
		}
	}
}

// filterCheckServiceNodes is used to filter nodes based on ACL rules.
func (f *aclFilter) filterCheckServiceNodes(nodes *structs.CheckServiceNodes) {
	csn := *nodes
	var authzContext acl.AuthorizerContext

	for i := 0; i < len(csn); i++ {
		node := csn[i]
		node.Service.FillAuthzContext(&authzContext)
		if f.allowNode(node.Node.Node, &authzContext) && f.allowService(node.Service.Service, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", node.Node.Node)
		csn = append(csn[:i], csn[i+1:]...)
		i--
	}
	*nodes = csn
}

// filterDatacenterCheckServiceNodes is used to filter nodes based on ACL rules.
func (f *aclFilter) filterDatacenterCheckServiceNodes(datacenterNodes *map[string]structs.CheckServiceNodes) {
	dn := *datacenterNodes
	out := make(map[string]structs.CheckServiceNodes)
	for dc, _ := range dn {
		nodes := dn[dc]
		f.filterCheckServiceNodes(&nodes)
		if len(nodes) > 0 {
			out[dc] = nodes
		}
	}
	*datacenterNodes = out
}

// filterSessions is used to filter a set of sessions based on ACLs.
func (f *aclFilter) filterSessions(sessions *structs.Sessions) {
	s := *sessions
	for i := 0; i < len(s); i++ {
		session := s[i]

		var entCtx acl.AuthorizerContext
		session.FillAuthzContext(&entCtx)

		if f.allowSession(session.Node, &entCtx) {
			continue
		}
		f.logger.Debug("dropping session from result due to ACLs", "session", session.ID)
		s = append(s[:i], s[i+1:]...)
		i--
	}
	*sessions = s
}

// filterCoordinates is used to filter nodes in a coordinate dump based on ACL
// rules.
func (f *aclFilter) filterCoordinates(coords *structs.Coordinates) {
	c := *coords
	var authzContext acl.AuthorizerContext
	structs.WildcardEnterpriseMeta().FillAuthzContext(&authzContext)

	for i := 0; i < len(c); i++ {
		node := c[i].Node
		if f.allowNode(node, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", node)
		c = append(c[:i], c[i+1:]...)
		i--
	}
	*coords = c
}

// filterIntentions is used to filter intentions based on ACL rules.
// We prune entries the user doesn't have access to, and we redact any tokens
// if the user doesn't have a management token.
func (f *aclFilter) filterIntentions(ixns *structs.Intentions) {
	ret := make(structs.Intentions, 0, len(*ixns))
	for _, ixn := range *ixns {
		if !ixn.CanRead(f.authorizer) {
			f.logger.Debug("dropping intention from result due to ACLs", "intention", ixn.ID)
			continue
		}

		ret = append(ret, ixn)
	}

	*ixns = ret
}

// filterNodeDump is used to filter through all parts of a node dump and
// remove elements the provided ACL token cannot access.
func (f *aclFilter) filterNodeDump(dump *structs.NodeDump) {
	nd := *dump

	var authzContext acl.AuthorizerContext
	for i := 0; i < len(nd); i++ {
		info := nd[i]

		// Filter nodes
		structs.WildcardEnterpriseMeta().FillAuthzContext(&authzContext)
		if node := info.Node; !f.allowNode(node, &authzContext) {
			f.logger.Debug("dropping node from result due to ACLs", "node", node)
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
			info.Checks = append(info.Checks[:j], info.Checks[j+1:]...)
			j--
		}
	}
	*dump = nd
}

// filterNodes is used to filter through all parts of a node list and remove
// elements the provided ACL token cannot access.
func (f *aclFilter) filterNodes(nodes *structs.Nodes) {
	n := *nodes

	var authzContext acl.AuthorizerContext
	structs.WildcardEnterpriseMeta().FillAuthzContext(&authzContext)

	for i := 0; i < len(n); i++ {
		node := n[i].Node
		if f.allowNode(node, &authzContext) {
			continue
		}
		f.logger.Debug("dropping node from result due to ACLs", "node", node)
		n = append(n[:i], n[i+1:]...)
		i--
	}
	*nodes = n
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
	structs.DefaultEnterpriseMeta().FillAuthzContext(&authzContext)
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
// if the user doesn't have a management token.
func (f *aclFilter) filterPreparedQueries(queries *structs.PreparedQueries) {
	var authzContext acl.AuthorizerContext
	structs.DefaultEnterpriseMeta().FillAuthzContext(&authzContext)
	// Management tokens can see everything with no filtering.
	// TODO  is this check even necessary - this looks like a search replace from
	// the 1.4 ACL rewrite. The global-management token will provide unrestricted query privileges
	// so asking for ACLWrite should be unnecessary.
	if f.authorizer.ACLWrite(&authzContext) == acl.Allow {
		return
	}

	// Otherwise, we need to see what the token has access to.
	ret := make(structs.PreparedQueries, 0, len(*queries))
	for _, query := range *queries {
		// If no prefix ACL applies to this query then filter it, since
		// we know at this point the user doesn't have a management
		// token, otherwise see what the policy says.
		prefix, ok := query.GetACLPrefix()
		if !ok || f.authorizer.PreparedQueryRead(prefix, &authzContext) != acl.Allow {
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

func (f *aclFilter) filterServiceList(services *structs.ServiceList) {
	ret := make(structs.ServiceList, 0, len(*services))
	for _, svc := range *services {
		var authzContext acl.AuthorizerContext

		svc.FillAuthzContext(&authzContext)

		if f.authorizer.ServiceRead(svc.Name, &authzContext) != acl.Allow {
			sid := structs.NewServiceID(svc.Name, &svc.EnterpriseMeta)
			f.logger.Debug("dropping service from result due to ACLs", "service", sid.String())
			continue
		}

		ret = append(ret, svc)
	}

	*services = ret
}

func (r *ACLResolver) filterACLWithAuthorizer(authorizer acl.Authorizer, subj interface{}) error {
	if authorizer == nil {
		return nil
	}
	// Create the filter
	filt := newACLFilter(authorizer, r.logger, r.config.ACLEnforceVersion8)

	switch v := subj.(type) {
	case *structs.CheckServiceNodes:
		filt.filterCheckServiceNodes(v)

	case *structs.IndexedCheckServiceNodes:
		filt.filterCheckServiceNodes(&v.Nodes)

	case *structs.DatacenterIndexedCheckServiceNodes:
		filt.filterDatacenterCheckServiceNodes(&v.DatacenterNodes)

	case *structs.IndexedCoordinates:
		filt.filterCoordinates(&v.Coordinates)

	case *structs.IndexedHealthChecks:
		filt.filterHealthChecks(&v.HealthChecks)

	case *structs.IndexedIntentions:
		filt.filterIntentions(&v.Intentions)

	case *structs.IndexedNodeDump:
		filt.filterNodeDump(&v.Dump)

	case *structs.IndexedNodes:
		filt.filterNodes(&v.Nodes)

	case *structs.IndexedNodeServices:
		filt.filterNodeServices(&v.NodeServices)

	case **structs.NodeServiceList:
		filt.filterNodeServiceList(v)

	case *structs.IndexedServiceNodes:
		filt.filterServiceNodes(&v.ServiceNodes)

	case *structs.IndexedServices:
		filt.filterServices(v.Services, &v.EnterpriseMeta)

	case *structs.IndexedSessions:
		filt.filterSessions(&v.Sessions)

	case *structs.IndexedPreparedQueries:
		filt.filterPreparedQueries(&v.Queries)

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
		filt.filterServiceList(&v.Services)
	default:
		panic(fmt.Errorf("Unhandled type passed to ACL filter: %T %#v", subj, subj))
	}

	return nil
}

// filterACL is used to filter results from our service catalog based on the
// rules configured for the provided token.
func (r *ACLResolver) filterACL(token string, subj interface{}) error {
	// Get the ACL from the token
	authorizer, err := r.ResolveToken(token)
	if err != nil {
		return err
	}

	// Fast path if ACLs are not enabled
	if authorizer == nil {
		return nil
	}

	return r.filterACLWithAuthorizer(authorizer, subj)
}

// vetRegisterWithACL applies the given ACL's policy to the catalog update and
// determines if it is allowed. Since the catalog register request is so
// dynamic, this is a pretty complex algorithm and was worth breaking out of the
// endpoint. The NodeServices record for the node must be supplied, and can be
// nil.
//
// This is a bit racy because we have to check the state store outside of a
// transaction. It's the best we can do because we don't want to flow ACL
// checking down there. The node information doesn't change in practice, so this
// will be fine. If we expose ways to change node addresses in a later version,
// then we should split the catalog API at the node and service level so we can
// address this race better (even then it would be super rare, and would at
// worst let a service update revert a recent node update, so it doesn't open up
// too much abuse).
func vetRegisterWithACL(rule acl.Authorizer, subj *structs.RegisterRequest,
	ns *structs.NodeServices) error {
	// Fast path if ACLs are not enabled.
	if rule == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	subj.FillAuthzContext(&authzContext)

	// Vet the node info. This allows service updates to re-post the required
	// node info for each request without having to have node "write"
	// privileges.
	needsNode := ns == nil || subj.ChangesNode(ns.Node)

	if needsNode && rule.NodeWrite(subj.Node, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	// Vet the service change. This includes making sure they can register
	// the given service, and that we can write to any existing service that
	// is being modified by id (if any).
	if subj.Service != nil {
		if rule.ServiceWrite(subj.Service.Service, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}

		if ns != nil {
			other, ok := ns.Services[subj.Service.ID]

			if ok {
				// This is effectively a delete, so we DO NOT apply the
				// sentinel scope to the service we are overwriting, just
				// the regular ACL policy.
				var secondaryCtx acl.AuthorizerContext
				other.FillAuthzContext(&secondaryCtx)

				if rule.ServiceWrite(other.Service, &secondaryCtx) != acl.Allow {
					return acl.ErrPermissionDenied
				}
			}
		}
	}

	// Make sure that the member was flattened before we got there. This
	// keeps us from having to verify this check as well.
	if subj.Check != nil {
		return fmt.Errorf("check member must be nil")
	}

	// Vet the checks. Node-level checks require node write, and
	// service-level checks require service write.
	for _, check := range subj.Checks {
		// Make sure that the node matches - we don't allow you to mix
		// checks from other nodes because we'd have to pull a bunch
		// more state store data to check this. If ACLs are enabled then
		// we simply require them to match in a given request. There's a
		// note in state_store.go to ban this down there in Consul 0.8,
		// but it's good to leave this here because it's required for
		// correctness wrt. ACLs.
		if check.Node != subj.Node {
			return fmt.Errorf("Node '%s' for check '%s' doesn't match register request node '%s'",
				check.Node, check.CheckID, subj.Node)
		}

		// Node-level check.
		if check.ServiceID == "" {
			if rule.NodeWrite(subj.Node, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
			continue
		}

		// Service-level check, check the common case where it
		// matches the service part of this request, which has
		// already been vetted above, and might be being registered
		// along with its checks.
		if subj.Service != nil && subj.Service.ID == check.ServiceID {
			continue
		}

		// Service-level check for some other service. Make sure they've
		// got write permissions for that service.
		if ns == nil {
			return fmt.Errorf("Unknown service '%s' for check '%s'", check.ServiceID, check.CheckID)
		}

		other, ok := ns.Services[check.ServiceID]
		if !ok {
			return fmt.Errorf("Unknown service '%s' for check '%s'", check.ServiceID, check.CheckID)
		}

		// We are only adding a check here, so we don't add the scope,
		// since the sentinel policy doesn't apply to adding checks at
		// this time.
		var secondaryCtx acl.AuthorizerContext
		other.FillAuthzContext(&secondaryCtx)

		if rule.ServiceWrite(other.Service, &secondaryCtx) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

// vetDeregisterWithACL applies the given ACL's policy to the catalog update and
// determines if it is allowed. Since the catalog deregister request is so
// dynamic, this is a pretty complex algorithm and was worth breaking out of the
// endpoint. The NodeService for the referenced service must be supplied, and can
// be nil; similar for the HealthCheck for the referenced health check.
func vetDeregisterWithACL(rule acl.Authorizer, subj *structs.DeregisterRequest,
	ns *structs.NodeService, nc *structs.HealthCheck) error {

	// Fast path if ACLs are not enabled.
	if rule == nil {
		return nil
	}

	// We don't apply sentinel in this path, since at this time sentinel
	// only applies to create and update operations.

	var authzContext acl.AuthorizerContext
	// fill with the defaults for use with the NodeWrite check
	subj.FillAuthzContext(&authzContext)

	// Allow service deregistration if the token has write permission for the node.
	// This accounts for cases where the agent no longer has a token with write permission
	// on the service to deregister it.
	if rule.NodeWrite(subj.Node, &authzContext) == acl.Allow {
		return nil
	}

	// This order must match the code in applyDeregister() in
	// fsm/commands_oss.go since it also evaluates things in this order,
	// and will ignore fields based on this precedence. This lets us also
	// ignore them from an ACL perspective.
	if subj.ServiceID != "" {
		if ns == nil {
			return fmt.Errorf("Unknown service '%s'", subj.ServiceID)
		}

		ns.FillAuthzContext(&authzContext)

		if rule.ServiceWrite(ns.Service, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	} else if subj.CheckID != "" {
		if nc == nil {
			return fmt.Errorf("Unknown check '%s'", subj.CheckID)
		}

		nc.FillAuthzContext(&authzContext)

		if nc.ServiceID != "" {
			if rule.ServiceWrite(nc.ServiceName, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		} else {
			if rule.NodeWrite(subj.Node, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}
		}
	} else {
		// Since NodeWrite is not given - otherwise the earlier check
		// would've returned already - we can deny here.
		return acl.ErrPermissionDenied
	}

	return nil
}

// vetNodeTxnOp applies the given ACL policy to a node transaction operation.
func vetNodeTxnOp(op *structs.TxnNodeOp, rule acl.Authorizer) error {
	// Fast path if ACLs are not enabled.
	if rule == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	op.FillAuthzContext(&authzContext)

	if rule != nil && rule.NodeWrite(op.Node.Node, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	return nil
}

// vetServiceTxnOp applies the given ACL policy to a service transaction operation.
func vetServiceTxnOp(op *structs.TxnServiceOp, rule acl.Authorizer) error {
	// Fast path if ACLs are not enabled.
	if rule == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	op.FillAuthzContext(&authzContext)

	if rule.ServiceWrite(op.Service.Service, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	return nil
}

// vetCheckTxnOp applies the given ACL policy to a check transaction operation.
func vetCheckTxnOp(op *structs.TxnCheckOp, rule acl.Authorizer) error {
	// Fast path if ACLs are not enabled.
	if rule == nil {
		return nil
	}

	var authzContext acl.AuthorizerContext
	op.FillAuthzContext(&authzContext)

	if op.Check.ServiceID == "" {
		// Node-level check.
		if rule.NodeWrite(op.Check.Node, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	} else {
		// Service-level check.
		if rule.ServiceWrite(op.Check.ServiceName, &authzContext) != acl.Allow {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

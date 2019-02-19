package consul

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sentinel"
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
)

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

type ACLResolverDelegate interface {
	ACLsEnabled() bool
	ACLDatacenter(legacy bool) string
	// UseLegacyACLs
	UseLegacyACLs() bool
	ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error)
	ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error)
	RPC(method string, args interface{}, reply interface{}) error
}

type remoteACLLegacyResult struct {
	authorizer acl.Authorizer
	err        error
}

type remoteACLIdentityResult struct {
	identity structs.ACLIdentity
	err      error
}

type remoteACLPolicyResult struct {
	policy *structs.ACLPolicy
	err    error
}

type policyTokenError struct {
	Err   error
	token string
}

func (e policyTokenError) Error() string {
	return e.Err.Error()
}

// ACLResolverConfig holds all the configuration necessary to create an ACLResolver
type ACLResolverConfig struct {
	Config *Config
	Logger *log.Logger

	// CacheConfig is a pass through configuration for ACL cache limits
	CacheConfig *structs.ACLCachesConfig

	// Delegate that implements some helper functionality that is server/client specific
	Delegate ACLResolverDelegate

	// AutoDisable indicates that RPC responses should be checked and if they indicate ACLs are disabled
	// remotely then disable them locally as well. This is particularly useful for the client agent
	// so that it can detect when the servers have gotten ACLs enabled.
	AutoDisable bool

	Sentinel sentinel.Evaluator
}

// ACLResolver is the type to handle all your token and policy resolution needs.
//
// Supports:
//   - Resolving tokens locally via the ACLResolverDelegate
//   - Resolving policies locally via the ACLResolverDelegate
//   - Resolving legacy tokens remotely via a ACL.GetPolicy RPC
//   - Resolving tokens remotely via an ACL.TokenRead RPC
//   - Resolving policies remotely via an ACL.PolicyResolve RPC
//
// Remote Resolution:
//   Remote resolution can be done syncrhonously or asynchronously depending
//   on the ACLDownPolicy in the Config passed to the resolver.
//
//   When the down policy is set to async-cache and we have already cached values
//   then go routines will be spawned to perform the RPCs in the background
//   and then will udpate the cache with either the positive or negative result.
//
//   When the down policy is set to extend-cache or the token/policy is not already
//   cached then the same go routines are spawned to do the RPCs in the background.
//   However in this mode channels are created to receive the results of the RPC
//   and are registered with the resolver. Those channels are immediately read/blocked
//   upon.
//
type ACLResolver struct {
	config *Config
	logger *log.Logger

	delegate ACLResolverDelegate
	sentinel sentinel.Evaluator

	cache                     *structs.ACLCaches
	asyncIdentityResults      map[string][]chan (*remoteACLIdentityResult)
	asyncIdentityResultsMutex sync.RWMutex
	asyncPolicyResults        map[string][]chan (*remoteACLPolicyResult)
	asyncPolicyResultsMutex   sync.RWMutex
	asyncLegacyResults        map[string][]chan (*remoteACLLegacyResult)
	asyncLegacyMutex          sync.RWMutex

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
		config.Logger = log.New(os.Stderr, "", log.LstdFlags)
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
		config:               config.Config,
		logger:               config.Logger,
		delegate:             config.Delegate,
		sentinel:             config.Sentinel,
		cache:                cache,
		asyncIdentityResults: make(map[string][]chan (*remoteACLIdentityResult)),
		asyncPolicyResults:   make(map[string][]chan (*remoteACLPolicyResult)),
		asyncLegacyResults:   make(map[string][]chan (*remoteACLLegacyResult)),
		autoDisable:          config.AutoDisable,
		down:                 down,
	}, nil
}

// fireAsyncLegacyResult is used to notify any watchers that legacy resolution of a token is complete
func (r *ACLResolver) fireAsyncLegacyResult(token string, authorizer acl.Authorizer, ttl time.Duration, err error) {
	// cache the result: positive or negative
	r.cache.PutAuthorizerWithTTL(token, authorizer, ttl)

	// get the list of channels to send the result to
	r.asyncLegacyMutex.Lock()
	channels := r.asyncLegacyResults[token]
	delete(r.asyncLegacyResults, token)
	r.asyncLegacyMutex.Unlock()

	// notify all watchers of the RPC results
	result := &remoteACLLegacyResult{authorizer, err}
	for _, cx := range channels {
		// only chans that are being blocked on will be in the list of channels so this cannot block
		cx <- result
		close(cx)
	}
}

func (r *ACLResolver) resolveTokenLegacyAsync(token string, cached *structs.AuthorizerCacheEntry) {
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
			r.fireAsyncLegacyResult(token, cached.Authorizer, cacheTTL, acl.ErrInvalidParent)
			return
		}

		var policies []*acl.Policy
		policy := reply.Policy
		if policy != nil {
			policies = append(policies, policy.ConvertFromLegacy())
		}

		authorizer, err := acl.NewPolicyAuthorizer(parent, policies, r.sentinel)
		r.fireAsyncLegacyResult(token, authorizer, reply.TTL, err)
		return
	}

	if acl.IsErrNotFound(err) {
		// Make sure to remove from the cache if it was deleted
		r.fireAsyncLegacyResult(token, nil, cacheTTL, acl.ErrNotFound)
		return
	}

	// some other RPC error
	switch r.config.ACLDownPolicy {
	case "allow":
		r.fireAsyncLegacyResult(token, acl.AllowAll(), cacheTTL, nil)
		return
	case "async-cache", "extend-cache":
		if cached != nil {
			r.fireAsyncLegacyResult(token, cached.Authorizer, cacheTTL, nil)
			return
		}
		fallthrough
	default:
		r.fireAsyncLegacyResult(token, acl.DenyAll(), cacheTTL, nil)
		return
	}
}

func (r *ACLResolver) resolveTokenLegacy(token string) (acl.Authorizer, error) {
	defer metrics.MeasureSince([]string{"acl", "resolveTokenLegacy"}, time.Now())

	// Attempt to resolve locally first (local results are not cached)
	// This is only useful for servers where either legacy replication is being
	// done or the server is within the primary datacenter.
	if done, identity, err := r.delegate.ResolveIdentityFromToken(token); done {
		if err == nil && identity != nil {
			policies, err := r.resolvePoliciesForIdentity(identity)
			if err != nil {
				return nil, err
			}

			return policies.Compile(acl.RootAuthorizer(r.config.ACLDefaultPolicy), r.cache, r.sentinel)
		}

		return nil, err
	}

	// Look in the cache prior to making a RPC request
	entry := r.cache.GetAuthorizer(token)

	if entry != nil && entry.Age() <= minTTL(entry.TTL, r.config.ACLTokenTTL) {
		metrics.IncrCounter([]string{"acl", "token", "cache_hit"}, 1)
		if entry.Authorizer != nil {
			return entry.Authorizer, nil
		}
		return nil, acl.ErrNotFound
	}

	metrics.IncrCounter([]string{"acl", "token", "cache_miss"}, 1)

	// Resolve the token in the background and wait on the result if we must
	var waitChan chan *remoteACLLegacyResult
	waitForResult := entry == nil || r.config.ACLDownPolicy != "async-cache"

	r.asyncLegacyMutex.Lock()

	// check if resolution for this token is already happening
	waiters, ok := r.asyncLegacyResults[token]
	if !ok || waiters == nil {
		// initialize the slice of waiters if not already done
		waiters = make([]chan *remoteACLLegacyResult, 0)
	}
	if waitForResult {
		// create the waitChan only if we are going to block waiting
		// for the response and then append it to the list of waiters
		// Because we will block (not select or discard this chan) we
		// do not need to create it as buffered
		waitChan = make(chan *remoteACLLegacyResult)
		r.asyncLegacyResults[token] = append(waiters, waitChan)
	}
	r.asyncLegacyMutex.Unlock()

	if !ok {
		// start the async RPC if it wasn't already ongoing
		go r.resolveTokenLegacyAsync(token, entry)
	}

	if !waitForResult {
		// waitForResult being false requires the cacheEntry to not be nil
		if entry.Authorizer != nil {
			return entry.Authorizer, nil
		}
		return nil, acl.ErrNotFound
	}

	// block waiting for the async RPC to finish.
	res := <-waitChan
	return res.authorizer, res.err
}

// fireAsyncTokenResult is used to notify all waiters that the results of a token resolution is complete
func (r *ACLResolver) fireAsyncTokenResult(token string, identity structs.ACLIdentity, err error) {
	// cache the result: positive or negative
	r.cache.PutIdentity(token, identity)

	// get the list of channels to send the result to
	r.asyncIdentityResultsMutex.Lock()
	channels := r.asyncIdentityResults[token]
	delete(r.asyncIdentityResults, token)
	r.asyncIdentityResultsMutex.Unlock()

	// notify all watchers of the RPC results
	result := &remoteACLIdentityResult{identity, err}
	for _, cx := range channels {
		// cannot block because all wait chans will have another goroutine blocked on the read
		cx <- result
		close(cx)
	}
}

func (r *ACLResolver) resolveIdentityFromTokenAsync(token string, cached *structs.IdentityCacheEntry) {
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
			r.fireAsyncTokenResult(token, nil, acl.ErrNotFound)
		} else {
			r.fireAsyncTokenResult(token, resp.Token, nil)
		}
		return
	}

	if acl.IsErrNotFound(err) {
		// Make sure to remove from the cache if it was deleted
		r.fireAsyncTokenResult(token, nil, acl.ErrNotFound)
		return
	}

	// some other RPC error
	if cached != nil && (r.config.ACLDownPolicy == "extend-cache" || r.config.ACLDownPolicy == "async-cache") {
		// extend the cache
		r.fireAsyncTokenResult(token, cached.Identity, nil)
	}

	r.fireAsyncTokenResult(token, nil, err)
	return
}

func (r *ACLResolver) resolveIdentityFromToken(token string) (structs.ACLIdentity, error) {
	// Attempt to resolve locally first (local results are not cached)
	if done, identity, err := r.delegate.ResolveIdentityFromToken(token); done {
		return identity, err
	}

	// Check the cache before making any RPC requests
	cacheEntry := r.cache.GetIdentity(token)
	if cacheEntry != nil && cacheEntry.Age() <= r.config.ACLTokenTTL {
		metrics.IncrCounter([]string{"acl", "token", "cache_hit"}, 1)
		return cacheEntry.Identity, nil
	}

	metrics.IncrCounter([]string{"acl", "token", "cache_miss"}, 1)

	// Background a RPC request and wait on it if we must
	var waitChan chan *remoteACLIdentityResult

	waitForResult := cacheEntry == nil || r.config.ACLDownPolicy != "async-cache"

	r.asyncIdentityResultsMutex.Lock()
	// check if resolution of this token is already ongoing
	waiters, ok := r.asyncIdentityResults[token]
	if !ok || waiters == nil {
		// only initialize the slice of waiters if need be (when this token resolution isn't ongoing)
		waiters = make([]chan *remoteACLIdentityResult, 0)
	}
	if waitForResult {
		// create the waitChan only if we are going to block waiting
		// for the response and then append it to the list of waiters
		// Because we will block (not select or discard this chan) we
		// do not need to create it as buffered
		waitChan = make(chan *remoteACLIdentityResult)
		r.asyncIdentityResults[token] = append(waiters, waitChan)
	}
	r.asyncIdentityResultsMutex.Unlock()

	if !ok {
		// only start the RPC if one isn't in flight
		go r.resolveIdentityFromTokenAsync(token, cacheEntry)
	}

	if !waitForResult {
		// waitForResult being false requires the cacheEntry to not be nil
		return cacheEntry.Identity, nil
	}

	// block on the read here, this is why we don't need chan buffering
	res := <-waitChan

	if res.err != nil && !acl.IsErrNotFound(res.err) {
		return res.identity, ACLRemoteError{Err: res.err}
	}
	return res.identity, res.err
}

// fireAsyncPolicyResult is used to notify all waiters that policy resolution is complete.
func (r *ACLResolver) fireAsyncPolicyResult(policyID string, policy *structs.ACLPolicy, err error, updateCache bool) {
	if updateCache {
		// cache the result: positive or negative
		r.cache.PutPolicy(policyID, policy)
	}

	// get the list of channels to send the result to
	r.asyncPolicyResultsMutex.Lock()
	channels := r.asyncPolicyResults[policyID]
	delete(r.asyncPolicyResults, policyID)
	r.asyncPolicyResultsMutex.Unlock()

	// notify all watchers of the RPC results
	result := &remoteACLPolicyResult{policy, err}
	for _, cx := range channels {
		// not closing the channel as there could be more events to be fired.
		cx <- result
	}
}

func (r *ACLResolver) resolvePoliciesAsyncForIdentity(identity structs.ACLIdentity, policyIDs []string, cached map[string]*structs.PolicyCacheEntry) {
	req := structs.ACLPolicyBatchGetRequest{
		Datacenter: r.delegate.ACLDatacenter(false),
		PolicyIDs:  policyIDs,
		QueryOptions: structs.QueryOptions{
			Token:      identity.SecretToken(),
			AllowStale: true,
		},
	}

	found := make(map[string]struct{})
	var resp structs.ACLPolicyBatchResponse
	err := r.delegate.RPC("ACL.PolicyResolve", &req, &resp)
	if err == nil {
		for _, policy := range resp.Policies {
			r.fireAsyncPolicyResult(policy.ID, policy, nil, true)
			found[policy.ID] = struct{}{}
		}

		for _, policyID := range policyIDs {
			if _, ok := found[policyID]; !ok {
				r.fireAsyncPolicyResult(policyID, nil, acl.ErrNotFound, true)
			}
		}
		return
	}

	if acl.IsErrNotFound(err) {
		// make sure to indicate that this identity is no longer valid within
		// the cache
		//
		// Note - This must be done before firing the results or else it would
		// be possible for waiters to get woken up an get the cached identity
		// again
		r.cache.PutIdentity(identity.SecretToken(), nil)
		for _, policyID := range policyIDs {
			// Do not touch the cache. Getting a top level ACL not found error
			// only indicates that the secret token used in the request
			// no longer exists
			r.fireAsyncPolicyResult(policyID, nil, &policyTokenError{acl.ErrNotFound, identity.SecretToken()}, false)
		}
		return
	}

	if acl.IsErrPermissionDenied(err) {
		// invalidate our ID cache so that identity resolution will take place
		// again in the future
		//
		// Note - This must be done before firing the results or else it would
		// be possible for waiters to get woken up and get the cached identity
		// again
		r.cache.RemoveIdentity(identity.SecretToken())

		for _, policyID := range policyIDs {
			// Do not remove from the cache for permission denied
			// what this does indicate is that our view of the token is out of date
			r.fireAsyncPolicyResult(policyID, nil, &policyTokenError{acl.ErrPermissionDenied, identity.SecretToken()}, false)
		}
		return
	}

	// other RPC error - use cache if available

	extendCache := r.config.ACLDownPolicy == "extend-cache" || r.config.ACLDownPolicy == "async-cache"
	for _, policyID := range policyIDs {
		if entry, ok := cached[policyID]; extendCache && ok {
			r.fireAsyncPolicyResult(policyID, entry.Policy, nil, true)
		} else {
			r.fireAsyncPolicyResult(policyID, nil, ACLRemoteError{Err: err}, true)
		}
	}
	return
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
	if len(policyIDs) == 0 {
		policy := identity.EmbeddedPolicy()
		if policy != nil {
			return []*structs.ACLPolicy{policy}, nil
		}

		// In this case the default policy will be all that is in effect.
		return nil, nil
	}

	// For the new ACLs policy replication is mandatory for correct operation on servers. Therefore
	// we only attempt to resolve policies locally
	policies := make([]*structs.ACLPolicy, 0, len(policyIDs))

	// Get all associated policies
	var missing []string
	var expired []*structs.ACLPolicy
	expCacheMap := make(map[string]*structs.PolicyCacheEntry)

	for _, policyID := range policyIDs {
		if done, policy, err := r.delegate.ResolvePolicyFromID(policyID); done {
			if err != nil && !acl.IsErrNotFound(err) {
				return nil, err
			}

			if policy != nil {
				policies = append(policies, policy)
			} else {
				r.logger.Printf("[WARN] acl: policy %q not found for identity %q", policyID, identity.ID())
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
			// this happens when we cache a negative response for the policies existence
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
		return r.filterPoliciesByScope(policies), nil
	}

	fetchIDs := missing
	for _, policy := range expired {
		fetchIDs = append(fetchIDs, policy.ID)
	}

	// Background a RPC request and wait on it if we must
	var waitChan chan *remoteACLPolicyResult
	waitForResult := len(missing) > 0 || r.config.ACLDownPolicy != "async-cache"
	if waitForResult {
		// buffered because there are going to be multiple go routines that send data to this chan
		waitChan = make(chan *remoteACLPolicyResult, len(fetchIDs))
	}

	var newAsyncFetchIds []string
	r.asyncPolicyResultsMutex.Lock()
	for _, policyID := range fetchIDs {
		clients, ok := r.asyncPolicyResults[policyID]
		if !ok || clients == nil {
			clients = make([]chan *remoteACLPolicyResult, 0)
		}
		if waitForResult {
			r.asyncPolicyResults[policyID] = append(clients, waitChan)
		}

		if !ok {
			newAsyncFetchIds = append(newAsyncFetchIds, policyID)
		}
	}
	r.asyncPolicyResultsMutex.Unlock()

	if len(newAsyncFetchIds) > 0 {
		// only start the RPC if one isn't in flight
		go r.resolvePoliciesAsyncForIdentity(identity, newAsyncFetchIds, expCacheMap)
	}

	if !waitForResult {
		// waitForResult being false requires that all the policies were cached already
		policies = append(policies, expired...)
		return r.filterPoliciesByScope(policies), nil
	}

	for i := 0; i < len(fetchIDs); i++ {
		res := <-waitChan

		if res.err != nil {
			if _, ok := res.err.(*policyTokenError); ok {
				// always return token errors
				return nil, res.err
			} else if !acl.IsErrNotFound(res.err) {
				// ignore regular not found errors for policies
				return nil, res.err
			}
		}

		// we probably could handle a special case where we
		// get a permission denied error due to another requests
		// issues and spawn the go routine to resolve it ourselves.
		// however this should be exceedingly rare and in this case
		// we can just kick the can down the road and retry the whole
		// token/policy resolution. All the remaining good bits that
		// we need will already be cached anyways.

		if res.policy != nil {
			policies = append(policies, res.policy)
		}
	}

	return r.filterPoliciesByScope(policies), nil
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

		if tokenErr, ok := err.(*policyTokenError); ok {
			if acl.IsErrNotFound(err) && tokenErr.token == identity.SecretToken() {
				// token was deleted while resolving policies
				return nil, nil, acl.ErrNotFound
			}

			// other types of policyTokenErrors should cause retrying the whole token
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

	r.logger.Printf("[DEBUG] acl: ACLs disabled on upstream servers, will check again after %s", r.config.ACLDisabledTTL)
	r.disabledLock.Lock()
	r.disabled = time.Now().Add(r.config.ACLDisabledTTL)
	r.disabledLock.Unlock()

	return err
}

func (r *ACLResolver) ResolveToken(token string) (acl.Authorizer, error) {
	if !r.ACLsEnabled() {
		return nil, nil
	}

	if acl.RootAuthorizer(token) != nil {
		return nil, acl.ErrRootDenied
	}

	// handle the anonymous token
	if token == "" {
		token = anonymousToken
	}

	if r.delegate.UseLegacyACLs() {
		authorizer, err := r.resolveTokenLegacy(token)
		return authorizer, r.disableACLsWhenUpstreamDisabled(err)
	}

	defer metrics.MeasureSince([]string{"acl", "ResolveToken"}, time.Now())

	policies, err := r.resolveTokenToPolicies(token)
	if err != nil {
		r.disableACLsWhenUpstreamDisabled(err)
		if IsACLRemoteError(err) {
			r.logger.Printf("[ERR] consul.acl: %v", err)
			return r.down, nil
		}

		return nil, err
	}

	// Build the Authorizer
	authorizer, err := policies.Compile(acl.RootAuthorizer(r.config.ACLDefaultPolicy), r.cache, r.sentinel)
	return authorizer, err

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

	return policies.Merge(r.cache, r.sentinel)
}

// aclFilter is used to filter results from our state store based on ACL rules
// configured for the provided token.
type aclFilter struct {
	authorizer      acl.Authorizer
	logger          *log.Logger
	enforceVersion8 bool
}

// newACLFilter constructs a new aclFilter.
func newACLFilter(authorizer acl.Authorizer, logger *log.Logger, enforceVersion8 bool) *aclFilter {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &aclFilter{
		authorizer:      authorizer,
		logger:          logger,
		enforceVersion8: enforceVersion8,
	}
}

// allowNode is used to determine if a node is accessible for an ACL.
func (f *aclFilter) allowNode(node string) bool {
	if !f.enforceVersion8 {
		return true
	}
	return f.authorizer.NodeRead(node)
}

// allowService is used to determine if a service is accessible for an ACL.
func (f *aclFilter) allowService(service string) bool {
	if service == "" {
		return true
	}

	if !f.enforceVersion8 && service == structs.ConsulServiceID {
		return true
	}
	return f.authorizer.ServiceRead(service)
}

// allowSession is used to determine if a session for a node is accessible for
// an ACL.
func (f *aclFilter) allowSession(node string) bool {
	if !f.enforceVersion8 {
		return true
	}
	return f.authorizer.SessionRead(node)
}

// filterHealthChecks is used to filter a set of health checks down based on
// the configured ACL rules for a token.
func (f *aclFilter) filterHealthChecks(checks *structs.HealthChecks) {
	hc := *checks
	for i := 0; i < len(hc); i++ {
		check := hc[i]
		if f.allowNode(check.Node) && f.allowService(check.ServiceName) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping check %q from result due to ACLs", check.CheckID)
		hc = append(hc[:i], hc[i+1:]...)
		i--
	}
	*checks = hc
}

// filterServices is used to filter a set of services based on ACLs.
func (f *aclFilter) filterServices(services structs.Services) {
	for svc := range services {
		if f.allowService(svc) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping service %q from result due to ACLs", svc)
		delete(services, svc)
	}
}

// filterServiceNodes is used to filter a set of nodes for a given service
// based on the configured ACL rules.
func (f *aclFilter) filterServiceNodes(nodes *structs.ServiceNodes) {
	sn := *nodes
	for i := 0; i < len(sn); i++ {
		node := sn[i]
		if f.allowNode(node.Node) && f.allowService(node.ServiceName) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping node %q from result due to ACLs", node.Node)
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

	if !f.allowNode((*services).Node.Node) {
		*services = nil
		return
	}

	for svc := range (*services).Services {
		if f.allowService(svc) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping service %q from result due to ACLs", svc)
		delete((*services).Services, svc)
	}
}

// filterCheckServiceNodes is used to filter nodes based on ACL rules.
func (f *aclFilter) filterCheckServiceNodes(nodes *structs.CheckServiceNodes) {
	csn := *nodes
	for i := 0; i < len(csn); i++ {
		node := csn[i]
		if f.allowNode(node.Node.Node) && f.allowService(node.Service.Service) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping node %q from result due to ACLs", node.Node.Node)
		csn = append(csn[:i], csn[i+1:]...)
		i--
	}
	*nodes = csn
}

// filterSessions is used to filter a set of sessions based on ACLs.
func (f *aclFilter) filterSessions(sessions *structs.Sessions) {
	s := *sessions
	for i := 0; i < len(s); i++ {
		session := s[i]
		if f.allowSession(session.Node) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping session %q from result due to ACLs", session.ID)
		s = append(s[:i], s[i+1:]...)
		i--
	}
	*sessions = s
}

// filterCoordinates is used to filter nodes in a coordinate dump based on ACL
// rules.
func (f *aclFilter) filterCoordinates(coords *structs.Coordinates) {
	c := *coords
	for i := 0; i < len(c); i++ {
		node := c[i].Node
		if f.allowNode(node) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping node %q from result due to ACLs", node)
		c = append(c[:i], c[i+1:]...)
		i--
	}
	*coords = c
}

// filterIntentions is used to filter intentions based on ACL rules.
// We prune entries the user doesn't have access to, and we redact any tokens
// if the user doesn't have a management token.
func (f *aclFilter) filterIntentions(ixns *structs.Intentions) {
	// Management tokens can see everything with no filtering.
	if f.authorizer.ACLRead() {
		return
	}

	// Otherwise, we need to see what the token has access to.
	ret := make(structs.Intentions, 0, len(*ixns))
	for _, ixn := range *ixns {
		// If no prefix ACL applies to this then filter it, since
		// we know at this point the user doesn't have a management
		// token, otherwise see what the policy says.
		prefix, ok := ixn.GetACLPrefix()
		if !ok || !f.authorizer.IntentionRead(prefix) {
			f.logger.Printf("[DEBUG] consul: dropping intention %q from result due to ACLs", ixn.ID)
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
	for i := 0; i < len(nd); i++ {
		info := nd[i]

		// Filter nodes
		if node := info.Node; !f.allowNode(node) {
			f.logger.Printf("[DEBUG] consul: dropping node %q from result due to ACLs", node)
			nd = append(nd[:i], nd[i+1:]...)
			i--
			continue
		}

		// Filter services
		for j := 0; j < len(info.Services); j++ {
			svc := info.Services[j].Service
			if f.allowService(svc) {
				continue
			}
			f.logger.Printf("[DEBUG] consul: dropping service %q from result due to ACLs", svc)
			info.Services = append(info.Services[:j], info.Services[j+1:]...)
			j--
		}

		// Filter checks
		for j := 0; j < len(info.Checks); j++ {
			chk := info.Checks[j]
			if f.allowService(chk.ServiceName) {
				continue
			}
			f.logger.Printf("[DEBUG] consul: dropping check %q from result due to ACLs", chk.CheckID)
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
	for i := 0; i < len(n); i++ {
		node := n[i].Node
		if f.allowNode(node) {
			continue
		}
		f.logger.Printf("[DEBUG] consul: dropping node %q from result due to ACLs", node)
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
	if f.authorizer.ACLWrite() {
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
	// Management tokens can see everything with no filtering.
	if f.authorizer.ACLWrite() {
		return
	}

	// Otherwise, we need to see what the token has access to.
	ret := make(structs.PreparedQueries, 0, len(*queries))
	for _, query := range *queries {
		// If no prefix ACL applies to this query then filter it, since
		// we know at this point the user doesn't have a management
		// token, otherwise see what the policy says.
		prefix, ok := query.GetACLPrefix()
		if !ok || !f.authorizer.PreparedQueryRead(prefix) {
			f.logger.Printf("[DEBUG] consul: dropping prepared query %q from result due to ACLs", query.ID)
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

func (f *aclFilter) redactTokenSecret(token **structs.ACLToken) {
	if token == nil || *token == nil || f == nil || f.authorizer.ACLWrite() {
		return
	}
	clone := *(*token)
	clone.SecretID = redactedToken
	*token = &clone
}

func (f *aclFilter) redactTokenSecrets(tokens *structs.ACLTokens) {
	ret := make(structs.ACLTokens, 0, len(*tokens))
	for _, token := range *tokens {
		final := token
		f.redactTokenSecret(&final)
		ret = append(ret, final)
	}
	*tokens = ret
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

	case *structs.IndexedServiceNodes:
		filt.filterServiceNodes(&v.ServiceNodes)

	case *structs.IndexedServices:
		filt.filterServices(v.Services)

	case *structs.IndexedSessions:
		filt.filterSessions(&v.Sessions)

	case *structs.IndexedPreparedQueries:
		filt.filterPreparedQueries(&v.Queries)

	case **structs.PreparedQuery:
		filt.redactPreparedQueryTokens(v)

	case *structs.ACLTokens:
		filt.redactTokenSecrets(v)

	case **structs.ACLToken:
		filt.redactTokenSecret(v)

	default:
		panic(fmt.Errorf("Unhandled type passed to ACL filter: %#v", subj))
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

	// This gets called potentially from a few spots so we save it and
	// return the structure we made if we have it.
	var memo map[string]interface{}
	scope := func() map[string]interface{} {
		if memo != nil {
			return memo
		}

		node := &api.Node{
			ID:              string(subj.ID),
			Node:            subj.Node,
			Address:         subj.Address,
			Datacenter:      subj.Datacenter,
			TaggedAddresses: subj.TaggedAddresses,
			Meta:            subj.NodeMeta,
		}

		var service *api.AgentService
		if subj.Service != nil {
			service = &api.AgentService{
				ID:                subj.Service.ID,
				Service:           subj.Service.Service,
				Tags:              subj.Service.Tags,
				Meta:              subj.Service.Meta,
				Address:           subj.Service.Address,
				Port:              subj.Service.Port,
				EnableTagOverride: subj.Service.EnableTagOverride,
			}
		}

		memo = sentinel.ScopeCatalogUpsert(node, service)
		return memo
	}

	// Vet the node info. This allows service updates to re-post the required
	// node info for each request without having to have node "write"
	// privileges.
	needsNode := ns == nil || subj.ChangesNode(ns.Node)

	if needsNode && !rule.NodeWrite(subj.Node, scope) {
		return acl.ErrPermissionDenied
	}

	// Vet the service change. This includes making sure they can register
	// the given service, and that we can write to any existing service that
	// is being modified by id (if any).
	if subj.Service != nil {
		if !rule.ServiceWrite(subj.Service.Service, scope) {
			return acl.ErrPermissionDenied
		}

		if ns != nil {
			other, ok := ns.Services[subj.Service.ID]

			// This is effectively a delete, so we DO NOT apply the
			// sentinel scope to the service we are overwriting, just
			// the regular ACL policy.
			if ok && !rule.ServiceWrite(other.Service, nil) {
				return acl.ErrPermissionDenied
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
			if !rule.NodeWrite(subj.Node, scope) {
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
		if !rule.ServiceWrite(other.Service, nil) {
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

	// This order must match the code in applyRegister() in fsm.go since it
	// also evaluates things in this order, and will ignore fields based on
	// this precedence. This lets us also ignore them from an ACL perspective.
	if subj.ServiceID != "" {
		if ns == nil {
			return fmt.Errorf("Unknown service '%s'", subj.ServiceID)
		}
		if !rule.ServiceWrite(ns.Service, nil) {
			return acl.ErrPermissionDenied
		}
	} else if subj.CheckID != "" {
		if nc == nil {
			return fmt.Errorf("Unknown check '%s'", subj.CheckID)
		}
		if nc.ServiceID != "" {
			if !rule.ServiceWrite(nc.ServiceName, nil) {
				return acl.ErrPermissionDenied
			}
		} else {
			if !rule.NodeWrite(subj.Node, nil) {
				return acl.ErrPermissionDenied
			}
		}
	} else {
		if !rule.NodeWrite(subj.Node, nil) {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

// vetNodeTxnOp applies the given ACL policy to a node transaction operation.
func vetNodeTxnOp(op *structs.TxnNodeOp, rule acl.Authorizer) error {
	// Fast path if ACLs are not enabled.
	if rule == nil {
		return nil
	}

	node := op.Node

	n := &api.Node{
		Node:            node.Node,
		ID:              string(node.ID),
		Address:         node.Address,
		Datacenter:      node.Datacenter,
		TaggedAddresses: node.TaggedAddresses,
		Meta:            node.Meta,
	}

	// Sentinel doesn't apply to deletes, only creates/updates, so we don't need the scopeFn.
	var scope func() map[string]interface{}
	if op.Verb != api.NodeDelete && op.Verb != api.NodeDeleteCAS {
		scope = func() map[string]interface{} {
			return sentinel.ScopeCatalogUpsert(n, nil)
		}
	}

	if rule != nil && !rule.NodeWrite(node.Node, scope) {
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

	service := op.Service

	n := &api.Node{Node: op.Node}
	svc := &api.AgentService{
		ID:                service.ID,
		Service:           service.Service,
		Tags:              service.Tags,
		Meta:              service.Meta,
		Address:           service.Address,
		Port:              service.Port,
		EnableTagOverride: service.EnableTagOverride,
	}
	var scope func() map[string]interface{}
	if op.Verb != api.ServiceDelete && op.Verb != api.ServiceDeleteCAS {
		scope = func() map[string]interface{} {
			return sentinel.ScopeCatalogUpsert(n, svc)
		}
	}
	if !rule.ServiceWrite(service.Service, scope) {
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

	n := &api.Node{Node: op.Check.Node}
	svc := &api.AgentService{
		ID:      op.Check.ServiceID,
		Service: op.Check.ServiceID,
		Tags:    op.Check.ServiceTags,
	}
	var scope func() map[string]interface{}
	if op.Check.ServiceID == "" {
		// Node-level check.
		if op.Verb == api.CheckDelete || op.Verb == api.CheckDeleteCAS {
			scope = func() map[string]interface{} {
				return sentinel.ScopeCatalogUpsert(n, svc)
			}
		}
		if !rule.NodeWrite(op.Check.Node, scope) {
			return acl.ErrPermissionDenied
		}
	} else {
		// Service-level check.
		if op.Verb == api.CheckDelete || op.Verb == api.CheckDeleteCAS {
			scope = func() map[string]interface{} {
				return sentinel.ScopeCatalogUpsert(n, svc)
			}
		}
		if !rule.ServiceWrite(op.Check.ServiceName, scope) {
			return acl.ErrPermissionDenied
		}
	}

	return nil
}

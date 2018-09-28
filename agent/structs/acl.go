package structs

import (
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/sentinel"
	"github.com/hashicorp/golang-lru"
	"github.com/mitchellh/hashstructure"
)

// ACLOp is used in RPCs to encode ACL operations.
type ACLOp string

type ACLTokenIDType string

const (
	ACLTokenSecret   ACLTokenIDType = "secret"
	ACLTokenAccessor ACLTokenIDType = "accessor"
)

type ACLPolicyIDType string

const (
	ACLPolicyName ACLPolicyIDType = "name"
	ACLPolicyID   ACLPolicyIDType = "id"
)

const (
	// All policy ids with the first 120 bits set to all zeroes are
	// reserved for builtin policies. Policy creation will ensure we
	// dont accidentally create them when autogenerating uuids.

	// This policy gives unlimited access to everything. Users
	// may rename if desired but cannot delete or modify the rules
	ACLPolicyGlobalManagementID = "00000000-0000-0000-0000-000000000001"
	ACLPolicyGlobalManagement   = `
acl = "write"
agent_prefix "" {
	policy = "write"
}
event_prefix "" {
	policy = "write"
}
key_prefix "" {
	policy = "write"
}
keyring = "write"
node_prefix "" {
	policy = "write"
}
operator = "write"
query_prefix "" {
	policy = "write"
}
service_prefix "" {
	policy = "write"
	intention = "write"
}
session_prefix "" {
	policy = "write"
}`

	// This is the policy ID for anonymous access. This is configurable by the
	ACLTokenAnonymousID = "00000000-0000-0000-0000-000000000002"
)

func ACLIDReserved(id string) bool {
	return strings.HasPrefix(id, "00000000-0000-0000-0000-0000000000")
}

const (
	// ACLSet creates or updates a token.
	ACLSet ACLOp = "set"

	// ACLDelete deletes a token.
	ACLDelete ACLOp = "delete"
)

// ACLBootstrapNotAllowedErr is returned once we know that a bootstrap can no
// longer be done since the cluster was bootstrapped
var ACLBootstrapNotAllowedErr = errors.New("ACL bootstrap no longer allowed")

type ACLIdentity interface {
	// ID returns a string that can be used for logging and telemetry. This should not
	// contain any secret data used for authentication
	ID() string
	SecretToken() string
	PolicyIDs() []string
	EmbeddedPolicy() *ACLPolicy
}

type ACLTokenPolicyLink struct {
	ID   string
	Name string `hash:"ignore"`
}

type ACLToken struct {
	// This is the UUID used for tracking and management purposes
	AccessorID string

	// This is the UUID used as the api token by clients
	SecretID string

	// Human readable string to display for the token (Optional)
	Description string

	// List of policy links - nil/empty for legacy tokens
	// Note this is the list of IDs and not the names. Prior to token creation
	// the list of policy names gets validated and the policy IDs get stored herein
	Policies []ACLTokenPolicyLink `hash:"set"`

	// Type is the V1 Token Type
	// DEPRECATED (ACL-Legacy-Compat) - remove once we no longer support v1 ACL compat
	// Even though we are going to auto upgrade management tokens we still
	// want to be able to have the old APIs operate on the upgraded management tokens
	// so this field is being kept to identify legacy tokens even after an auto-upgrade
	Type string `json:",omitempty"`

	// Rules is the V1 acl rules associated with
	// DEPRECATED (ACL-Legacy-Compat) - remove once we no longer support v1 ACL compat
	Rules string `json:",omitempty"`

	// Whether this token is DC local. This means that it will not be synced
	// to the ACL datacenter and replicated to others.
	Local bool

	// Hash of the contents of the token
	//
	// This is needed mainly for replication purposes. When replicating from
	// one DC to another keeping the content Hash will allow us to avoid
	// unnecessary calls to the authoritative DC
	Hash uint64 `hash:"ignore"`

	// Embedded Raft Metadata
	RaftIndex `hash:"ignore"`
}

func (t *ACLToken) ID() string {
	return t.AccessorID
}

func (t *ACLToken) SecretToken() string {
	return t.SecretID
}

func (t *ACLToken) PolicyIDs() []string {
	if len(t.Policies) == 0 && t.Type == ACLTokenTypeManagement {
		return []string{
			ACLPolicyGlobalManagementID,
		}
	}
	var ids []string
	for _, link := range t.Policies {
		ids = append(ids, link.ID)
	}
	return ids
}

func (t *ACLToken) EmbeddedPolicy() *ACLPolicy {
	// DEPRECATED (ACL-Legacy-Compat)
	//
	// For legacy tokens with embedded rules this provides a way to map those
	// rules to an ACLPolicy. This function can just return nil once legacy
	// acl compatibility is no longer needed
	if t.Rules == "" {
		return nil
	}

	hasher := fnv.New128a()
	ruleID := fmt.Sprintf("%x", hasher.Sum([]byte(t.Rules)))

	policy := &ACLPolicy{
		ID:     ruleID,
		Name:   fmt.Sprintf("legacy-policy-%s", ruleID),
		Rules:  t.Rules,
		Syntax: acl.SyntaxLegacy,
	}

	policy.SetHash(true)
	return policy
}

func (t *ACLToken) IsManagement() bool {
	return t.Type == ACLTokenTypeManagement && len(t.Policies) == 0
}

func (t *ACLToken) SetHash(force bool) uint64 {
	if force || t.Hash == 0 {
		t.Hash, _ = hashstructure.Hash(t, nil)
	}
	return t.Hash
}

// ACLTokens is a slice of ACLTokens.
type ACLTokens []*ACLToken

// IsSame checks if one ACL is the same as another, without looking
// at the Raft information (that's why we didn't call it IsEqual). This is
// useful for seeing if an update would be idempotent for all the functional
// parts of the structure.
func (token *ACLToken) IsSame(other *ACLToken) bool {
	if token.AccessorID != other.AccessorID ||
		token.SecretID != other.SecretID ||
		token.Description != other.Description ||
		token.Local != other.Local ||
		len(token.Policies) != len(other.Policies) ||
		// DEPRECATED (ACL-Legacy-Compat) - remove these 2 checks when v1 acl support is removed
		token.Type != other.Type ||
		token.Rules != other.Rules {
		return false
	}

	for idx, policy := range token.Policies {
		if policy != other.Policies[idx] {
			return false
		}
	}

	return true
}

type ACLTokenWriteRequest struct {
	Datacenter string   // The DC to perform the request within
	Op         ACLOp    // The operation to perform (supports bootstrap, set and delete)
	ACLToken   ACLToken // Token to manipulate - I really dislike this name but to satis
	WriteRequest
}

func (r *ACLTokenWriteRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLTokenBootstrapRequest struct {
	Datacenter string   // The DC to perform the request within
	ACLToken   ACLToken // Token to use for bootstrapping
	ResetIndex uint64   // Reset index
	WriteRequest
}

func (r *ACLTokenBootstrapRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLTokenReadRequest struct {
	Datacenter string         // The DC to perform the request within
	ID         string         // id used for the token lookup
	IDType     ACLTokenIDType // The Type of ID used to lookup the token
	QueryOptions
}

func (r *ACLTokenReadRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenResponse returns a single Token + metadata
type ACLTokenResponse struct {
	Token *ACLToken
	QueryMeta
}

// ACLTokensResponse returns multiple Tokens associated with the same metadata
type ACLTokensResponse struct {
	Tokens []*ACLToken
	QueryMeta
}

type ACLPolicy struct {
	// This is the internal UUID associated with the policy
	ID string `hash:"ignore"`

	// Unique name to reference the policy by.
	//   - Valid Characters: [a-zA-Z0-9-]
	//   - Valid Lengths: 1 - 128
	Name string

	// Human readable description (Optional)
	Description string

	// The rule set (using the updated rule syntax)
	Rules string

	// DEPRECATED (ACL-Legacy-Compat) - This is only needed while we support the legacy ACLS
	Syntax acl.SyntaxVersion `json:"-"`

	// Datacenters that the policy is valid within.
	//   - No wildcards allowed
	//   - If empty then the policy is valid within all datacenters
	Datacenters []string `json:",omitempty" hash:"set"`

	// Hash of the contents of the policy
	// This does not take into account the ID (which is immutable)
	// nor the raft metadata.
	//
	// This is needed mainly for replication purposes. When replicating from
	// one DC to another keeping the content Hash will allow us to avoid
	// unnecessary calls to the authoritative DC
	Hash uint64 `hash:"ignore"`

	// Embedded Raft Metadata
	RaftIndex `hash:"ignore"`
}

type ACLPolicies []*ACLPolicy

func (policy *ACLPolicy) IsSame(other *ACLPolicy) bool {
	if policy.ID != other.ID ||
		policy.Name != other.Name ||
		policy.Description != other.Description ||
		policy.Rules != other.Rules ||
		len(policy.Datacenters) != len(other.Datacenters) {
		return false
	}

	for idx, dc := range policy.Datacenters {
		if dc != other.Datacenters[idx] {
			return false
		}
	}

	return true
}

func (p *ACLPolicy) SetHash(force bool) uint64 {
	if force || p.Hash == 0 {
		p.Hash, _ = hashstructure.Hash(p, nil)
	}
	return p.Hash
}

type ACLPolicyWriteRequest struct {
	Datacenter string    // The DC to perform the request within
	Op         ACLOp     // Supports either "set" or "delete"
	Policy     ACLPolicy // The policy to manipulate
	WriteRequest
}

func (r *ACLPolicyWriteRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLPolicyReadRequest struct {
	Datacenter string          // The DC to perform the request within
	ID         string          // id used for the policy lookup
	IDType     ACLPolicyIDType // The type of id used to lookup the token
	QueryOptions
}

func (r *ACLPolicyReadRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLPolicyResolveRequest struct {
	Datacenter string
	IDs        []string
	QueryOptions
}

func (r *ACLPolicyResolveRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyResponse returns a single policy + metadata
type ACLPolicyResponse struct {
	Policy *ACLPolicy
	QueryMeta
}

type ACLPolicyMultiResponse struct {
	Policies []*ACLPolicy
	QueryMeta
}

// ACLReplicationStatus provides information about the health of the ACL
// replication system.
type ACLReplicationStatus struct {
	Enabled          bool
	Running          bool
	SourceDatacenter string
	ReplicatedIndex  uint64
	LastSuccess      time.Time
	LastError        time.Time
}

type ACLCachesConfig struct {
	Identities     int
	Policies       int
	ParsedPolicies int
	Authorizers    int
}

type ACLCaches struct {
	identities     *lru.TwoQueueCache // identity id -> structs.ACLIdentity
	parsedPolicies *lru.TwoQueueCache // policy content hash -> acl.Policy
	policies       *lru.TwoQueueCache // policy ID -> ACLPolicy
	authorizers    *lru.TwoQueueCache // token secret -> acl.Authorizer

	legacy *lru.TwoQueueCache // po
}

type IdentityCacheEntry struct {
	Identity  ACLIdentity
	CacheTime time.Time
}

func (e *IdentityCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

type ParsedPolicyCacheEntry struct {
	Policy    *acl.Policy
	CacheTime time.Time
}

func (e *ParsedPolicyCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

type PolicyCacheEntry struct {
	Policy    *ACLPolicy
	CacheTime time.Time
}

func (e *PolicyCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

type AuthorizerCacheEntry struct {
	Authorizer acl.Authorizer
	CacheTime  time.Time
}

func (e *AuthorizerCacheEntry) Age() time.Duration {
	return time.Since(e.CacheTime)
}

func NewACLCaches(config *ACLCachesConfig) (*ACLCaches, error) {
	cache := &ACLCaches{}

	if config != nil && config.Identities > 0 {
		identCache, err := lru.New2Q(config.Identities)
		if err != nil {
			return nil, err
		}

		cache.identities = identCache
	}

	if config != nil && config.Policies > 0 {
		policyCache, err := lru.New2Q(config.Policies)
		if err != nil {
			return nil, err
		}

		cache.policies = policyCache
	}

	if config != nil && config.ParsedPolicies > 0 {
		parsedCache, err := lru.New2Q(config.ParsedPolicies)
		if err != nil {
			return nil, err
		}

		cache.parsedPolicies = parsedCache
	}

	if config != nil && config.Authorizers > 0 {
		authCache, err := lru.New2Q(config.Authorizers)
		if err != nil {
			return nil, err
		}

		cache.authorizers = authCache
	}

	return cache, nil
}

// GetIdentity fetches an identity from the cache and returns it
func (c *ACLCaches) GetIdentity(id string) *IdentityCacheEntry {
	if c == nil || c.identities == nil {
		return nil
	}

	if raw, ok := c.identities.Get(id); ok {
		return raw.(*IdentityCacheEntry)
	}

	return nil
}

// GetPolicy fetches a policy from the cache and returns it
func (c *ACLCaches) GetPolicy(policyID string) *PolicyCacheEntry {
	if c == nil || c.policies == nil {
		return nil
	}

	if raw, ok := c.policies.Get(policyID); ok {
		return raw.(*PolicyCacheEntry)
	}

	return nil
}

// GetPolicy fetches a policy from the cache and returns it
func (c *ACLCaches) GetParsedPolicy(id uint64) *ParsedPolicyCacheEntry {
	if c == nil || c.parsedPolicies == nil {
		return nil
	}

	if raw, ok := c.parsedPolicies.Get(id); ok {
		return raw.(*ParsedPolicyCacheEntry)
	}

	return nil
}

// GetAuthorizer fetches a acl from the cache and returns it
func (c *ACLCaches) GetAuthorizer(id uint64) *AuthorizerCacheEntry {
	if c == nil || c.authorizers == nil {
		return nil
	}

	if raw, ok := c.authorizers.Get(id); ok {
		return raw.(*AuthorizerCacheEntry)
	}

	return nil
}

// PutIdentity adds a new identity to the cache
func (c *ACLCaches) PutIdentity(id string, ident ACLIdentity) {
	if c == nil || c.identities == nil {
		return
	}

	c.identities.Add(id, &IdentityCacheEntry{Identity: ident, CacheTime: time.Now()})
}

func (c *ACLCaches) PutPolicy(policyId string, policy *ACLPolicy) {
	if c == nil || c.policies == nil {
		return
	}

	c.policies.Add(policyId, &PolicyCacheEntry{Policy: policy, CacheTime: time.Now()})
}

func (c *ACLCaches) PutParsedPolicy(id uint64, policy *acl.Policy) {
	if c == nil || c.parsedPolicies == nil {
		return
	}

	c.parsedPolicies.Add(id, &ParsedPolicyCacheEntry{Policy: policy, CacheTime: time.Now()})
}

func (c *ACLCaches) PutAuthorizer(id uint64, authorizer acl.Authorizer) {
	if c == nil || c.authorizers == nil {
		return
	}

	c.authorizers.Add(id, &AuthorizerCacheEntry{Authorizer: authorizer, CacheTime: time.Now()})
}

func (c *ACLCaches) RemoveIdentity(id string) {
	if c != nil && c.identities != nil {
		c.identities.Remove(id)
	}
}

func (c *ACLCaches) RemovePolicy(policyID string) {
	if c != nil && c.policies != nil {
		c.policies.Remove(policyID)
	}
}

func (c *ACLCaches) Purge() {
	if c != nil {
		if c.identities != nil {
			c.identities.Purge()
		}
		if c.policies != nil {
			c.policies.Purge()
		}
		if c.parsedPolicies != nil {
			c.parsedPolicies.Purge()
		}
		if c.authorizers != nil {
			c.authorizers.Purge()
		}
	}
}

func (policies ACLPolicies) HashKey() uint64 {
	var key uint64

	// Not just using hashstructure.Hash on the policies slice because it
	// will perform an ordered hash. Instead the xor is what hashstructure
	// would do if the policies slice was in a struct and the struct field
	// were tagged like `hash:"set"`
	//
	// Also if we have computed the hash of the policies previously we
	// will not recompute the hash.
	for _, policy := range policies {
		policy.SetHash(false)
		key = key ^ policy.Hash
	}
	return key
}

func (policies ACLPolicies) resolveWithCache(cache *ACLCaches, sentinel sentinel.Evaluator) ([]*acl.Policy, error) {
	// Parse the policies
	parsed := make([]*acl.Policy, 0, len(policies))
	for _, policy := range policies {
		policy.SetHash(false)
		cachedPolicy := cache.GetParsedPolicy(policy.Hash)
		if cachedPolicy != nil {
			// policies are content hashed so no need to check the age
			parsed = append(parsed, cachedPolicy.Policy)
			continue
		}

		p, err := acl.NewPolicyFromSource(policy.ID, policy.ModifyIndex, policy.Rules, policy.Syntax, sentinel)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %v", policy.Name, err)
		}

		cache.PutParsedPolicy(policy.Hash, p)
		parsed = append(parsed, p)
	}

	return parsed, nil
}

func (policies ACLPolicies) Compile(parent acl.Authorizer, cache *ACLCaches, sentinel sentinel.Evaluator) (acl.Authorizer, error) {
	// Determine the cache key
	cacheKey := policies.HashKey()
	entry := cache.GetAuthorizer(cacheKey)
	if entry != nil {
		// the hash key takes into account the policy contents. There is no reason to expire this cache or check its age.
		return entry.Authorizer, nil
	}

	parsed, err := policies.resolveWithCache(cache, sentinel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the ACL policies: %v", err)
	}

	// Create the ACL object
	authorizer, err := acl.NewPolicyAuthorizer(parent, parsed, sentinel)
	if err != nil {
		return nil, fmt.Errorf("failed to construct ACL Authorizer: %v", err)
	}

	// Update the cache
	cache.PutAuthorizer(cacheKey, authorizer)
	return authorizer, nil
}

func (policies ACLPolicies) Merge(cache *ACLCaches, sentinel sentinel.Evaluator) (*acl.Policy, error) {
	parsed, err := policies.resolveWithCache(cache, sentinel)
	if err != nil {
		return nil, err
	}

	return acl.MergePolicies(parsed), nil
}

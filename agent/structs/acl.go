package structs

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/sentinel"
	"github.com/mitchellh/hashstructure"
)

type ACLMode string

const (
	// ACLs are disabled by configuration
	ACLModeDisabled ACLMode = "0"
	// ACLs are enabled
	ACLModeEnabled ACLMode = "1"
	// DEPRECATED (ACL-Legacy-Compat) - only needed while legacy ACLs are supported
	// ACLs are enabled and using legacy ACLs
	ACLModeLegacy ACLMode = "2"
	// DEPRECATED (ACL-Legacy-Compat) - only needed while legacy ACLs are supported
	// ACLs are assumed enabled but not being advertised
	ACLModeUnknown ACLMode = "3"
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
	Type string `json:"-"`

	// Rules is the V1 acl rules associated with
	// DEPRECATED (ACL-Legacy-Compat) - remove once we no longer support v1 ACL compat
	Rules string `json:",omitempty"`

	// Whether this token is DC local. This means that it will not be synced
	// to the ACL datacenter and replicated to others.
	Local bool

	// The time when this token was created
	CreateTime time.Time `json:",omitempty" hash:"ignore"`

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

func (t *ACLToken) EstimateSize() int {
	// 33 = 16 (RaftIndex) + 8 (Hash) + 8 (CreateTime) + 1 (Local)
	size := 33 + len(t.AccessorID) + len(t.SecretID) + len(t.Description) + len(t.Type) + len(t.Rules)
	for _, link := range t.Policies {
		size += len(link.ID) + len(link.Name)
	}
	return size
}

// ACLTokens is a slice of ACLTokens.
type ACLTokens []*ACLToken

type ACLTokenListStub struct {
	AccessorID  string
	Description string
	Policies    []ACLTokenPolicyLink
	Local       bool
	CreateTime  time.Time `json:",omitempty"`
	Hash        uint64
	CreateIndex uint64
	ModifyIndex uint64
	Legacy      bool `json:",omitempty"`
}

type ACLTokenListStubs []*ACLTokenListStub

func (token *ACLToken) Stub() *ACLTokenListStub {
	return &ACLTokenListStub{
		AccessorID:  token.AccessorID,
		Description: token.Description,
		Policies:    token.Policies,
		Local:       token.Local,
		CreateTime:  token.CreateTime,
		Hash:        token.Hash,
		CreateIndex: token.CreateIndex,
		ModifyIndex: token.ModifyIndex,
		Legacy:      token.Rules != "",
	}
}

func (tokens ACLTokens) Sort() {
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].AccessorID < tokens[j].AccessorID
	})
}

func (tokens ACLTokenListStubs) Sort() {
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].AccessorID < tokens[j].AccessorID
	})
}

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

type ACLPolicyListStub struct {
	ID          string
	Name        string
	Description string
	Datacenters []string
	Hash        uint64
	CreateIndex uint64
	ModifyIndex uint64
}

func (p *ACLPolicy) Stub() *ACLPolicyListStub {
	return &ACLPolicyListStub{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Datacenters: p.Datacenters,
		Hash:        p.Hash,
		CreateIndex: p.CreateIndex,
		ModifyIndex: p.ModifyIndex,
	}
}

type ACLPolicies []*ACLPolicy
type ACLPolicyListStubs []*ACLPolicyListStub

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

/*
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
}*/

func (p *ACLPolicy) EstimateSize() int {
	// This is just an estimate. There is other data structure overhead
	// pointers etc that this does not account for.

	// 64 = 36 (uuid) + 16 (RaftIndex) + 8 (Hash) + 4 (Syntax)
	size := 64 + len(p.Name) + len(p.Description) + len(p.Rules)
	for _, dc := range p.Datacenters {
		size += len(dc)
	}

	return size
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

func (policies ACLPolicies) Sort() {
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].ID < policies[j].ID
	})
}

func (policies ACLPolicyListStubs) Sort() {
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].ID < policies[j].ID
	})
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

type ACLReplicationType string

const (
	ACLReplicateLegacy   ACLReplicationType = "legacy"
	ACLReplicatePolicies ACLReplicationType = "policies"
	ACLReplicateTokens   ACLReplicationType = "tokens"
)

// ACLReplicationStatus provides information about the health of the ACL
// replication system.
type ACLReplicationStatus struct {
	Enabled              bool
	Running              bool
	SourceDatacenter     string
	ReplicationType      ACLReplicationType
	ReplicatedIndex      uint64
	ReplicatedTokenIndex uint64
	LastSuccess          time.Time
	LastError            time.Time
}

// ACLTokenUpsertRequest is used for token creation and update operations
// at the RPC layer
type ACLTokenUpsertRequest struct {
	ACLToken   ACLToken // Token to manipulate - I really dislike this name but "Token" is taken in the WriteRequest
	Datacenter string   // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLTokenUpsertRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenReadRequest is used for token read operations at the RPC layer
type ACLTokenReadRequest struct {
	TokenID     string         // id used for the token lookup
	TokenIDType ACLTokenIDType // The Type of ID used to lookup the token
	Datacenter  string         // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLTokenReadRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenDeleteRequest is used for token deletion operations at the RPC layer
type ACLTokenDeleteRequest struct {
	TokenID    string // ID of the token to delete
	Datacenter string // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLTokenDeleteRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenListRequest is used for token listing operations at the RPC layer
type ACLTokenListRequest struct {
	IncludeLocal  bool   // Whether local tokens should be included
	IncludeGlobal bool   // Whether global tokens should be included
	Policy        string // Policy filter
	Datacenter    string // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLTokenListRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenListResponse is used to return the secret data free stubs
// of the tokens
type ACLTokenListResponse struct {
	Tokens []*ACLTokenListStub
	QueryMeta
}

// ACLTokenBatchReadRequest is used for reading multiple tokens, this is
// different from the the token list request in that only tokens with the
// the requested ids are returned
type ACLTokenBatchReadRequest struct {
	AccessorIDs []string // List of accessor ids to fetch
	Datacenter  string   // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLTokenBatchReadRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenBatchUpsertRequest is used only at the Raft layer
// for batching multiple token creation/update operations
//
// This is particularly useful during token replication and during
// automatic legacy token upgrades.
type ACLTokenBatchUpsertRequest struct {
	Tokens      ACLTokens
	AllowCreate bool
}

// ACLTokenBatchDeleteRequest is used only at the Raft layer
// for batching multiple token deletions.
//
// This is particularly useful during token replication when
// multiple tokens need to be removed from the local DCs state.
type ACLTokenBatchDeleteRequest struct {
	TokenIDs []string // Tokens to delete
}

// ACLTokenBootstrapRequest is used only at the Raft layer
// for ACL bootstrapping
//
// The RPC layer will use a generic DCSpecificRequest to indicate
// that bootstrapping must be performed but the actual token
// and the resetIndex will be generated by that RPC endpoint
type ACLTokenBootstrapRequest struct {
	Token      ACLToken // Token to use for bootstrapping
	ResetIndex uint64   // Reset index
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

// ACLPolicyUpsertRequest is used at the RPC layer for creation and update requests
type ACLPolicyUpsertRequest struct {
	Policy     ACLPolicy // The policy to upsert
	Datacenter string    // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLPolicyUpsertRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyDeleteRequest is used at the RPC layer deletion requests
type ACLPolicyDeleteRequest struct {
	PolicyID   string // The id of the policy to delete
	Datacenter string // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLPolicyDeleteRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyReadRequest is used at the RPC layer to perform policy read operations
type ACLPolicyReadRequest struct {
	PolicyID   string // id used for the policy lookup
	Datacenter string // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLPolicyReadRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyListRequest is used at the RPC layer to request a listing of policies
type ACLPolicyListRequest struct {
	DCScope    string
	Datacenter string // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLPolicyListRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLPolicyListResponse struct {
	Policies ACLPolicyListStubs
	QueryMeta
}

// ACLPolicyBatchReadRequest is used at the RPC layer to request a subset of
// the policies associated with the token used for retrieval
type ACLPolicyBatchReadRequest struct {
	PolicyIDs  []string // List of policy ids to fetch
	Datacenter string   // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLPolicyBatchReadRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyResponse returns a single policy + metadata
type ACLPolicyResponse struct {
	Policy *ACLPolicy
	QueryMeta
}

type ACLPoliciesResponse struct {
	Policies []*ACLPolicy
	QueryMeta
}

// ACLPolicyBatchUpsertRequest is used at the Raft layer for batching
// multiple policy creations and updates
//
// This is particularly useful during replication
type ACLPolicyBatchUpsertRequest struct {
	Policies ACLPolicies
}

// ACLPolicyBatchDeleteRequest is used at the Raft layer for batching
// multiple policy deletions
//
// This is particularly useful during replication
type ACLPolicyBatchDeleteRequest struct {
	PolicyIDs []string
}

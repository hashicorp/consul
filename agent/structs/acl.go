package structs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/sentinel"
	"golang.org/x/crypto/blake2b"
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
	// may rename if desired but cannot delete or modify the rules.
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
	intentions = "write"
}
session_prefix "" {
	policy = "write"
}`

	// This is the policy ID for anonymous access. This is configurable by the
	// user.
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

// ACLBootstrapInvalidResetIndexErr is returned when bootstrap is requested with a non-zero
// reset index but the index doesn't match the bootstrap index
var ACLBootstrapInvalidResetIndexErr = errors.New("Invalid ACL bootstrap reset index")

type ACLIdentity interface {
	// ID returns a string that can be used for logging and telemetry. This should not
	// contain any secret data used for authentication
	ID() string
	SecretToken() string
	PolicyIDs() []string
	EmbeddedPolicy() *ACLPolicy
	IsExpired(asOf time.Time) bool
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
	Policies []ACLTokenPolicyLink

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

	// ExpirationTime represents the point after which a token should be
	// considered revoked and is eligible for destruction. The zero value
	// represents NO expiration.
	ExpirationTime time.Time `json:",omitempty"`

	// ExpirationTTL is a convenience field for helping set ExpirationTime to a
	// value of CreateTime+ExpirationTTL. This can only be set during
	// TokenCreate and is cleared and used to initialize the ExpirationTime
	// field before being persisted to the state store or raft log.
	//
	// This is a string version of a time.Duration like "2m".
	ExpirationTTL time.Duration `json:",omitempty"`

	// The time when this token was created
	CreateTime time.Time `json:",omitempty"`

	// Hash of the contents of the token
	//
	// This is needed mainly for replication purposes. When replicating from
	// one DC to another keeping the content Hash will allow us to avoid
	// unnecessary calls to the authoritative DC
	Hash []byte

	// Embedded Raft Metadata
	RaftIndex
}

func (t *ACLToken) Clone() *ACLToken {
	t2 := *t
	t2.Policies = nil

	if len(t.Policies) > 0 {
		t2.Policies = make([]ACLTokenPolicyLink, len(t.Policies))
		copy(t2.Policies, t.Policies)
	}
	return &t2
}

func (t *ACLToken) ID() string {
	return t.AccessorID
}

func (t *ACLToken) SecretToken() string {
	return t.SecretID
}

func (t *ACLToken) PolicyIDs() []string {
	var ids []string
	for _, link := range t.Policies {
		ids = append(ids, link.ID)
	}
	return ids
}

func (t *ACLToken) IsExpired(asOf time.Time) bool {
	if asOf.IsZero() || t.ExpirationTime.IsZero() {
		return false
	}
	return t.ExpirationTime.Before(asOf)
}

func (t *ACLToken) UsesNonLegacyFields() bool {
	return len(t.Policies) > 0 ||
		t.Type == "" ||
		!t.ExpirationTime.IsZero() ||
		t.ExpirationTTL != 0
}

func (t *ACLToken) EmbeddedPolicy() *ACLPolicy {
	// DEPRECATED (ACL-Legacy-Compat)
	//
	// For legacy tokens with embedded rules this provides a way to map those
	// rules to an ACLPolicy. This function can just return nil once legacy
	// acl compatibility is no longer needed.
	//
	// Additionally for management tokens we must embed the policy rules
	// as well
	policy := &ACLPolicy{}
	if t.Type == ACLTokenTypeManagement {
		hasher := fnv.New128a()
		policy.ID = fmt.Sprintf("%x", hasher.Sum([]byte(ACLPolicyGlobalManagement)))
		policy.Name = "legacy-management"
		policy.Rules = ACLPolicyGlobalManagement
		policy.Syntax = acl.SyntaxCurrent
	} else if t.Rules != "" || t.Type == ACLTokenTypeClient {
		hasher := fnv.New128a()
		policy.ID = fmt.Sprintf("%x", hasher.Sum([]byte(t.Rules)))
		policy.Name = fmt.Sprintf("legacy-policy-%s", policy.ID)
		policy.Rules = t.Rules
		policy.Syntax = acl.SyntaxLegacy
	} else {
		return nil
	}

	policy.SetHash(true)
	return policy
}

func (t *ACLToken) SetHash(force bool) []byte {
	if force || t.Hash == nil {
		// Initialize a 256bit Blake2 hash (32 bytes)
		hash, err := blake2b.New256(nil)
		if err != nil {
			panic(err)
		}

		// Any non-immutable "content" fields should be involved with the
		// overall hash. The IDs are immutable which is why they aren't here.
		// The raft indices are metadata similar to the hash which is why they
		// aren't incorporated. CreateTime is similarly immutable
		//
		// The Hash is really only used for replication to determine if a token
		// has changed and should be updated locally.

		// Write all the user set fields
		hash.Write([]byte(t.Description))
		hash.Write([]byte(t.Type))
		hash.Write([]byte(t.Rules))

		if t.Local {
			hash.Write([]byte("local"))
		} else {
			hash.Write([]byte("global"))
		}

		for _, link := range t.Policies {
			hash.Write([]byte(link.ID))
		}

		// Finalize the hash
		hashVal := hash.Sum(nil)

		// Set and return the hash
		t.Hash = hashVal
	}
	return t.Hash
}

func (t *ACLToken) EstimateSize() int {
	// 41 = 16 (RaftIndex) + 8 (Hash) + 8 (ExpirationTime) + 8 (CreateTime) + 1 (Local)
	size := 41 + len(t.AccessorID) + len(t.SecretID) + len(t.Description) + len(t.Type) + len(t.Rules)
	for _, link := range t.Policies {
		size += len(link.ID) + len(link.Name)
	}
	return size
}

// ACLTokens is a slice of ACLTokens.
type ACLTokens []*ACLToken

type ACLTokenListStub struct {
	AccessorID     string
	Description    string
	Policies       []ACLTokenPolicyLink
	Local          bool
	ExpirationTime time.Time `json:",omitempty"`
	CreateTime     time.Time `json:",omitempty"`
	Hash           []byte
	CreateIndex    uint64
	ModifyIndex    uint64
	Legacy         bool `json:",omitempty"`
}

type ACLTokenListStubs []*ACLTokenListStub

func (token *ACLToken) Stub() *ACLTokenListStub {
	return &ACLTokenListStub{
		AccessorID:     token.AccessorID,
		Description:    token.Description,
		Policies:       token.Policies,
		Local:          token.Local,
		ExpirationTime: token.ExpirationTime,
		CreateTime:     token.CreateTime,
		Hash:           token.Hash,
		CreateIndex:    token.CreateIndex,
		ModifyIndex:    token.ModifyIndex,
		Legacy:         token.Rules != "",
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

type ACLPolicy struct {
	// This is the internal UUID associated with the policy
	ID string

	// Unique name to reference the policy by.
	//   - Valid Characters: [a-zA-Z0-9-]
	//   - Valid Lengths: 1 - 128
	Name string

	// Human readable description (Optional)
	Description string

	// The rule set (using the updated rule syntax)
	Rules string

	// DEPRECATED (ACL-Legacy-Compat) - This is only needed while we support the legacy ACLs
	Syntax acl.SyntaxVersion `json:"-"`

	// Datacenters that the policy is valid within.
	//   - No wildcards allowed
	//   - If empty then the policy is valid within all datacenters
	Datacenters []string `json:",omitempty"`

	// Hash of the contents of the policy
	// This does not take into account the ID (which is immutable)
	// nor the raft metadata.
	//
	// This is needed mainly for replication purposes. When replicating from
	// one DC to another keeping the content Hash will allow us to avoid
	// unnecessary calls to the authoritative DC
	Hash []byte

	// Embedded Raft Metadata
	RaftIndex `hash:"ignore"`
}

func (p *ACLPolicy) Clone() *ACLPolicy {
	p2 := *p
	p2.Datacenters = nil
	if len(p.Datacenters) > 0 {
		p2.Datacenters = make([]string, len(p.Datacenters))
		copy(p2.Datacenters, p.Datacenters)
	}
	return &p2
}

type ACLPolicyListStub struct {
	ID          string
	Name        string
	Description string
	Datacenters []string
	Hash        []byte
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

func (p *ACLPolicy) SetHash(force bool) []byte {
	if force || p.Hash == nil {
		// Initialize a 256bit Blake2 hash (32 bytes)
		hash, err := blake2b.New256(nil)
		if err != nil {
			panic(err)
		}

		// Any non-immutable "content" fields should be involved with the
		// overall hash. The ID is immutable which is why it isn't here.  The
		// raft indices are metadata similar to the hash which is why they
		// aren't incorporated. CreateTime is similarly immutable
		//
		// The Hash is really only used for replication to determine if a policy
		// has changed and should be updated locally.

		// Write all the user set fields
		hash.Write([]byte(p.Name))
		hash.Write([]byte(p.Description))
		hash.Write([]byte(p.Rules))
		for _, dc := range p.Datacenters {
			hash.Write([]byte(dc))
		}

		// Finalize the hash
		hashVal := hash.Sum(nil)

		// Set and return the hash
		p.Hash = hashVal
	}
	return p.Hash
}

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

// HashKey returns a consistent hash for a set of policies.
func (policies ACLPolicies) HashKey() string {
	cacheKeyHash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	for _, policy := range policies {
		cacheKeyHash.Write([]byte(policy.ID))
		// including the modify index prevents a policy set from being
		// cached if one of the policies has changed
		binary.Write(cacheKeyHash, binary.BigEndian, policy.ModifyIndex)
	}
	return fmt.Sprintf("%x", cacheKeyHash.Sum(nil))
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
		cacheKey := fmt.Sprintf("%x", policy.Hash)
		cachedPolicy := cache.GetParsedPolicy(cacheKey)
		if cachedPolicy != nil {
			// policies are content hashed so no need to check the age
			parsed = append(parsed, cachedPolicy.Policy)
			continue
		}

		p, err := acl.NewPolicyFromSource(policy.ID, policy.ModifyIndex, policy.Rules, policy.Syntax, sentinel)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %v", policy.Name, err)
		}

		cache.PutParsedPolicy(cacheKey, p)
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

// ACLTokenSetRequest is used for token creation and update operations
// at the RPC layer
type ACLTokenSetRequest struct {
	ACLToken   ACLToken // Token to manipulate - I really dislike this name but "Token" is taken in the WriteRequest
	Datacenter string   // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLTokenSetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenGetRequest is used for token read operations at the RPC layer
type ACLTokenGetRequest struct {
	TokenID     string         // id used for the token lookup
	TokenIDType ACLTokenIDType // The Type of ID used to lookup the token
	Datacenter  string         // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLTokenGetRequest) RequestDatacenter() string {
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
	Tokens ACLTokenListStubs
	QueryMeta
}

// ACLTokenBatchGetRequest is used for reading multiple tokens, this is
// different from the the token list request in that only tokens with the
// the requested ids are returned
type ACLTokenBatchGetRequest struct {
	AccessorIDs []string // List of accessor ids to fetch
	Datacenter  string   // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLTokenBatchGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenBatchSetRequest is used only at the Raft layer
// for batching multiple token creation/update operations
//
// This is particularly useful during token replication and during
// automatic legacy token upgrades.
type ACLTokenBatchSetRequest struct {
	Tokens ACLTokens
	CAS    bool
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
	Token    *ACLToken
	Redacted bool // whether the token's secret was redacted
	QueryMeta
}

// ACLTokenBatchResponse returns multiple Tokens associated with the same metadata
type ACLTokenBatchResponse struct {
	Tokens   []*ACLToken
	Redacted bool // whether the token secrets were redacted.
	QueryMeta
}

// ACLPolicySetRequest is used at the RPC layer for creation and update requests
type ACLPolicySetRequest struct {
	Policy     ACLPolicy // The policy to upsert
	Datacenter string    // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLPolicySetRequest) RequestDatacenter() string {
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

// ACLPolicyGetRequest is used at the RPC layer to perform policy read operations
type ACLPolicyGetRequest struct {
	PolicyID   string // id used for the policy lookup
	Datacenter string // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLPolicyGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyListRequest is used at the RPC layer to request a listing of policies
type ACLPolicyListRequest struct {
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

// ACLPolicyBatchGetRequest is used at the RPC layer to request a subset of
// the policies associated with the token used for retrieval
type ACLPolicyBatchGetRequest struct {
	PolicyIDs  []string // List of policy ids to fetch
	Datacenter string   // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLPolicyBatchGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyResponse returns a single policy + metadata
type ACLPolicyResponse struct {
	Policy *ACLPolicy
	QueryMeta
}

type ACLPolicyBatchResponse struct {
	Policies []*ACLPolicy
	QueryMeta
}

// ACLPolicyBatchSetRequest is used at the Raft layer for batching
// multiple policy creations and updates
//
// This is particularly useful during replication
type ACLPolicyBatchSetRequest struct {
	Policies ACLPolicies
}

// ACLPolicyBatchDeleteRequest is used at the Raft layer for batching
// multiple policy deletions
//
// This is particularly useful during replication
type ACLPolicyBatchDeleteRequest struct {
	PolicyIDs []string
}

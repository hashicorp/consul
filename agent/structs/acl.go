// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/lib/stringslice"

	"golang.org/x/crypto/blake2b"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/lib"
)

type ACLMode string

const (
	// ACLModeDisabled indicates the ACL system is disabled
	ACLModeDisabled ACLMode = "0"
	// ACLModeEnabled indicates the ACL system is enabled
	ACLModeEnabled ACLMode = "1"
)

type ACLTokenIDType string

const (
	ACLTokenSecret   ACLTokenIDType = "secret"
	ACLTokenAccessor ACLTokenIDType = "accessor"
)

const (
	// All policy ids with the first 120 bits set to all zeroes are
	// reserved for builtin policies. Policy creation will ensure we
	// dont accidentally create them when autogenerating uuids.

	// This policy gives unlimited access to everything. Users
	// may rename if desired but cannot delete or modify the rules.
	ACLPolicyGlobalManagementID    = "00000000-0000-0000-0000-000000000001"
	ACLPolicyGlobalManagementName  = "global-management"
	ACLPolicyGlobalManagementDesc  = "Builtin Policy that grants unlimited access"
	ACLPolicyGlobalManagementRules = `
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
mesh = "write"
peering = "write"
query_prefix "" {
	policy = "write"
}
service_prefix "" {
	policy = "write"
	intentions = "write"
}
session_prefix "" {
	policy = "write"
}` + EnterpriseACLPolicyGlobalManagement

	ACLPolicyGlobalReadOnlyID    = "00000000-0000-0000-0000-000000000002"
	ACLPolicyGlobalReadOnlyName  = "builtin/global-read-only"
	ACLPolicyGlobalReadOnlyDesc  = "Builtin Policy that grants unlimited read-only access to all components"
	ACLPolicyGlobalReadOnlyRules = `
acl = "read"
agent_prefix "" {
	policy = "read"
}
event_prefix "" {
	policy = "read"
}
key_prefix "" {
	policy = "read"
}
keyring = "read"
node_prefix "" {
	policy = "read"
}
operator = "read"
mesh = "read"
peering = "read"
query_prefix "" {
	policy = "read"
}
service_prefix "" {
	policy = "read"
	intentions = "read"
}
session_prefix "" {
	policy = "read"
}` + EnterpriseACLPolicyGlobalReadOnly

	ACLReservedIDPrefix = "00000000-0000-0000-0000-0000000000"
)

var ACLBuiltinPolicies = map[string]ACLPolicy{
	ACLPolicyGlobalManagementID: {
		ID:          ACLPolicyGlobalManagementID,
		Name:        ACLPolicyGlobalManagementName,
		Description: ACLPolicyGlobalManagementDesc,
		Rules:       ACLPolicyGlobalManagementRules,
	},
	ACLPolicyGlobalReadOnlyID: {
		ID:          ACLPolicyGlobalReadOnlyID,
		Name:        ACLPolicyGlobalReadOnlyName,
		Description: ACLPolicyGlobalReadOnlyDesc,
		Rules:       ACLPolicyGlobalReadOnlyRules,
	},
}

func ACLIDReserved(id string) bool {
	return strings.HasPrefix(id, ACLReservedIDPrefix)
}

// ACLBootstrapNotAllowedErr is returned once we know that a bootstrap can no
// longer be done since the cluster was bootstrapped
var ACLBootstrapNotAllowedErr = errors.New("ACL bootstrap no longer allowed")

// ACLBootstrapInvalidResetIndexErr is returned when bootstrap is requested with a non-zero
// reset index but the index doesn't match the bootstrap index
var ACLBootstrapInvalidResetIndexErr = errors.New("Invalid ACL bootstrap reset index")

type ACLIdentity interface {
	// ID returns the accessor ID, a string that can be used for logging and
	// telemetry. It is not the secret ID used for authentication.
	ID() string
	SecretToken() string
	PolicyIDs() []string
	RoleIDs() []string
	ServiceIdentityList() []*ACLServiceIdentity
	NodeIdentityList() []*ACLNodeIdentity
	IsExpired(asOf time.Time) bool
	IsLocal() bool
	EnterpriseMetadata() *acl.EnterpriseMeta
}

type ACLTokenPolicyLink struct {
	ID   string
	Name string `hash:"ignore"`
}

type ACLTokenRoleLink struct {
	ID   string
	Name string `hash:"ignore"`
}

// ACLServiceIdentity represents a high-level grant of all necessary privileges
// to assume the identity of the named Service in the Catalog and within
// Connect.
type ACLServiceIdentity struct {
	ServiceName string

	// Datacenters that the synthetic policy will be valid within.
	//   - No wildcards allowed
	//   - If empty then the synthetic policy is valid within all datacenters
	//
	// Only valid for global tokens. It is an error to specify this for local tokens.
	Datacenters []string `json:",omitempty"`
}

func (s *ACLServiceIdentity) Clone() *ACLServiceIdentity {
	s2 := *s
	s2.Datacenters = stringslice.CloneStringSlice(s.Datacenters)
	return &s2
}

func (s *ACLServiceIdentity) AddToHash(h hash.Hash) {
	h.Write([]byte(s.ServiceName))
	for _, dc := range s.Datacenters {
		h.Write([]byte(dc))
	}
}

func (s *ACLServiceIdentity) EstimateSize() int {
	size := len(s.ServiceName)
	for _, dc := range s.Datacenters {
		size += len(dc)
	}
	return size
}

func (s *ACLServiceIdentity) SyntheticPolicy(entMeta *acl.EnterpriseMeta) *ACLPolicy {
	// Given that we validate this string name before persisting, we do not
	// have to escape it before doing the following interpolation.
	rules := aclServiceIdentityRules(s.ServiceName, entMeta)

	hasher := fnv.New128a()
	hashID := fmt.Sprintf("%x", hasher.Sum([]byte(rules)))

	policy := &ACLPolicy{}
	policy.ID = hashID
	policy.Name = fmt.Sprintf("synthetic-policy-%s", hashID)
	sn := NewServiceName(s.ServiceName, entMeta)
	policy.Description = fmt.Sprintf("synthetic policy for service identity %q", sn.String())
	policy.Rules = rules
	policy.Datacenters = s.Datacenters
	policy.EnterpriseMeta.Merge(entMeta)
	policy.SetHash(true)
	return policy
}

type ACLServiceIdentities []*ACLServiceIdentity

// Deduplicate returns a new list of service identities without duplicates.
// Identities with the same ServiceName but different datacenters will be
// merged into a single identity with all datacenters.
func (ids ACLServiceIdentities) Deduplicate() ACLServiceIdentities {
	unique := make(map[string]*ACLServiceIdentity)

	for _, id := range ids {
		entry, ok := unique[id.ServiceName]
		if ok {
			dcs := stringslice.CloneStringSlice(id.Datacenters)
			sort.Strings(dcs)
			entry.Datacenters = stringslice.MergeSorted(dcs, entry.Datacenters)
		} else {
			entry = id.Clone()
			sort.Strings(entry.Datacenters)
			unique[id.ServiceName] = entry
		}
	}

	results := make(ACLServiceIdentities, 0, len(unique))
	for _, id := range unique {
		results = append(results, id)
	}
	return results
}

// ACLNodeIdentity represents a high-level grant of all privileges
// necessary to assume the identity of that node and manage it.
type ACLNodeIdentity struct {
	// NodeName identities the Node that this identity authorizes access to
	NodeName string

	// Datacenter is required and specifies the datacenter of the node.
	Datacenter string
}

func (s *ACLNodeIdentity) Clone() *ACLNodeIdentity {
	s2 := *s
	return &s2
}

func (s *ACLNodeIdentity) AddToHash(h hash.Hash) {
	h.Write([]byte(s.NodeName))
	h.Write([]byte(s.Datacenter))
}

func (s *ACLNodeIdentity) EstimateSize() int {
	return len(s.NodeName) + len(s.Datacenter)
}

func (s *ACLNodeIdentity) SyntheticPolicy(entMeta *acl.EnterpriseMeta) *ACLPolicy {
	// Given that we validate this string name before persisting, we do not
	// have to escape it before doing the following interpolation.
	rules := aclNodeIdentityRules(s.NodeName, entMeta)

	hasher := fnv.New128a()
	hashID := fmt.Sprintf("%x", hasher.Sum([]byte(rules)))

	policy := &ACLPolicy{}
	policy.ID = hashID
	policy.Name = fmt.Sprintf("synthetic-policy-%s", hashID)
	policy.Description = fmt.Sprintf("synthetic policy for node identity %q", s.NodeName)
	policy.Rules = rules
	policy.Datacenters = []string{s.Datacenter}
	policy.EnterpriseMeta.Merge(entMeta)
	policy.SetHash(true)
	return policy
}

type ACLNodeIdentities []*ACLNodeIdentity

// Deduplicate returns a new list of node identities without duplicates.
func (ids ACLNodeIdentities) Deduplicate() ACLNodeIdentities {
	type mapKey struct {
		nodeName, datacenter string
	}
	seen := make(map[mapKey]struct{})

	var results ACLNodeIdentities
	for _, id := range ids {
		key := mapKey{id.NodeName, id.Datacenter}
		if _, ok := seen[key]; ok {
			continue
		}
		results = append(results, id.Clone())
		seen[key] = struct{}{}
	}
	return results
}

type ACLToken struct {
	// This is the UUID used for tracking and management purposes
	AccessorID string

	// This is the UUID used as the api token by clients
	SecretID string

	// Human readable string to display for the token (Optional)
	Description string

	// List of policy links - nil/empty for legacy tokens or if service identities are in use.
	// Note this is the list of IDs and not the names. Prior to token creation
	// the list of policy names gets validated and the policy IDs get stored herein
	Policies []ACLTokenPolicyLink `json:",omitempty"`

	// List of role links. Note this is the list of IDs and not the names.
	// Prior to token creation the list of role names gets validated and the
	// role IDs get stored herein
	Roles []ACLTokenRoleLink `json:",omitempty"`

	// List of services to generate synthetic policies for.
	ServiceIdentities ACLServiceIdentities `json:",omitempty"`

	// The node identities that this token should be allowed to manage.
	NodeIdentities ACLNodeIdentities `json:",omitempty"`

	// Whether this token is DC local. This means that it will not be synced
	// to the ACL datacenter and replicated to others.
	Local bool

	// AuthMethod is the name of the auth method used to create this token.
	AuthMethod string `json:",omitempty"`

	// ACLAuthMethodEnterpriseMeta is the EnterpriseMeta for the AuthMethod that this token was created from
	ACLAuthMethodEnterpriseMeta

	// ExpirationTime represents the point after which a token should be
	// considered revoked and is eligible for destruction. The zero value
	// represents NO expiration.
	//
	// This is a pointer value so that the zero value is omitted properly
	// during json serialization. time.Time does not respect json omitempty
	// directives unfortunately.
	ExpirationTime *time.Time `json:",omitempty"`

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

	// Embedded Enterprise Metadata
	acl.EnterpriseMeta `mapstructure:",squash"`

	// Embedded Raft Metadata
	RaftIndex
}

func (t *ACLToken) UnmarshalJSON(data []byte) (err error) {
	type Alias ACLToken
	aux := &struct {
		ExpirationTTL interface{}
		Hash          string
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if aux.ExpirationTTL != nil {
		switch v := aux.ExpirationTTL.(type) {
		case string:
			if t.ExpirationTTL, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.ExpirationTTL = time.Duration(v)
		}

	}
	if aux.Hash != "" {
		t.Hash = []byte(aux.Hash)
	}
	return nil
}

func (t *ACLToken) Clone() *ACLToken {
	t2 := *t
	t2.Policies = nil
	t2.Roles = nil
	t2.ServiceIdentities = nil
	t2.NodeIdentities = nil

	if len(t.Policies) > 0 {
		t2.Policies = make([]ACLTokenPolicyLink, len(t.Policies))
		copy(t2.Policies, t.Policies)
	}
	if len(t.Roles) > 0 {
		t2.Roles = make([]ACLTokenRoleLink, len(t.Roles))
		copy(t2.Roles, t.Roles)
	}
	if len(t.ServiceIdentities) > 0 {
		t2.ServiceIdentities = make([]*ACLServiceIdentity, len(t.ServiceIdentities))
		for i, s := range t.ServiceIdentities {
			t2.ServiceIdentities[i] = s.Clone()
		}
	}
	if len(t.NodeIdentities) > 0 {
		t2.NodeIdentities = make([]*ACLNodeIdentity, len(t.NodeIdentities))
		for i, n := range t.NodeIdentities {
			t2.NodeIdentities[i] = n.Clone()
		}
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
	if len(t.Policies) == 0 {
		return nil
	}

	ids := make([]string, 0, len(t.Policies))
	for _, link := range t.Policies {
		ids = append(ids, link.ID)
	}
	return ids
}

func (t *ACLToken) RoleIDs() []string {
	if len(t.Roles) == 0 {
		return nil
	}

	ids := make([]string, 0, len(t.Roles))
	for _, link := range t.Roles {
		ids = append(ids, link.ID)
	}
	return ids
}

func (t *ACLToken) ServiceIdentityList() []*ACLServiceIdentity {
	if len(t.ServiceIdentities) == 0 {
		return nil
	}

	out := make([]*ACLServiceIdentity, 0, len(t.ServiceIdentities))
	for _, s := range t.ServiceIdentities {
		out = append(out, s.Clone())
	}
	return out
}

func (t *ACLToken) IsExpired(asOf time.Time) bool {
	if asOf.IsZero() || !t.HasExpirationTime() {
		return false
	}
	return t.ExpirationTime.Before(asOf)
}

func (t *ACLToken) IsLocal() bool {
	return t.Local
}

func (t *ACLToken) HasExpirationTime() bool {
	return t.ExpirationTime != nil && !t.ExpirationTime.IsZero()
}

func (t *ACLToken) EnterpriseMetadata() *acl.EnterpriseMeta {
	return &t.EnterpriseMeta
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

		if t.Local {
			hash.Write([]byte("local"))
		} else {
			hash.Write([]byte("global"))
		}

		for _, link := range t.Policies {
			hash.Write([]byte(link.ID))
		}

		for _, link := range t.Roles {
			hash.Write([]byte(link.ID))
		}

		for _, srvid := range t.ServiceIdentities {
			srvid.AddToHash(hash)
		}

		for _, nodeID := range t.NodeIdentities {
			nodeID.AddToHash(hash)
		}

		t.EnterpriseMeta.AddToHash(hash, false)

		// Finalize the hash
		hashVal := hash.Sum(nil)

		// Set and return the hash
		t.Hash = hashVal
	}
	return t.Hash
}

func (t *ACLToken) EstimateSize() int {
	// 41 = 16 (RaftIndex) + 8 (Hash) + 8 (ExpirationTime) + 8 (CreateTime) + 1 (Local)
	size := 41 + len(t.AccessorID) + len(t.SecretID) + len(t.Description) + len(t.AuthMethod)
	for _, link := range t.Policies {
		size += len(link.ID) + len(link.Name)
	}
	for _, link := range t.Roles {
		size += len(link.ID) + len(link.Name)
	}
	for _, srvid := range t.ServiceIdentities {
		size += srvid.EstimateSize()
	}
	for _, nodeID := range t.NodeIdentities {
		size += nodeID.EstimateSize()
	}
	return size + t.EnterpriseMeta.EstimateSize()
}

// ACLTokens is a slice of ACLTokens.
type ACLTokens []*ACLToken

type ACLTokenListStub struct {
	AccessorID        string
	SecretID          string
	Description       string
	Policies          []ACLTokenPolicyLink `json:",omitempty"`
	Roles             []ACLTokenRoleLink   `json:",omitempty"`
	ServiceIdentities ACLServiceIdentities `json:",omitempty"`
	NodeIdentities    ACLNodeIdentities    `json:",omitempty"`
	Local             bool
	AuthMethod        string     `json:",omitempty"`
	ExpirationTime    *time.Time `json:",omitempty"`
	CreateTime        time.Time  `json:",omitempty"`
	Hash              []byte
	CreateIndex       uint64
	ModifyIndex       uint64
	acl.EnterpriseMeta
	ACLAuthMethodEnterpriseMeta
}

type ACLTokenListStubs []*ACLTokenListStub

func (token *ACLToken) Stub() *ACLTokenListStub {
	return &ACLTokenListStub{
		AccessorID:                  token.AccessorID,
		SecretID:                    token.SecretID,
		Description:                 token.Description,
		Policies:                    token.Policies,
		Roles:                       token.Roles,
		ServiceIdentities:           token.ServiceIdentities,
		NodeIdentities:              token.NodeIdentities,
		Local:                       token.Local,
		AuthMethod:                  token.AuthMethod,
		ExpirationTime:              token.ExpirationTime,
		CreateTime:                  token.CreateTime,
		Hash:                        token.Hash,
		CreateIndex:                 token.CreateIndex,
		ModifyIndex:                 token.ModifyIndex,
		EnterpriseMeta:              token.EnterpriseMeta,
		ACLAuthMethodEnterpriseMeta: token.ACLAuthMethodEnterpriseMeta,
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

	// Embedded Enterprise ACL Metadata
	acl.EnterpriseMeta `mapstructure:",squash"`

	// Embedded Raft Metadata
	RaftIndex `hash:"ignore"`
}

func (t *ACLPolicy) UnmarshalJSON(data []byte) error {
	type Alias ACLPolicy
	aux := &struct {
		Hash string
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if aux.Hash != "" {
		t.Hash = []byte(aux.Hash)
	}
	return nil
}

func (p *ACLPolicy) Clone() *ACLPolicy {
	p2 := *p
	p2.Datacenters = stringslice.CloneStringSlice(p.Datacenters)
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
	acl.EnterpriseMeta
}

func (p *ACLPolicy) Stub() *ACLPolicyListStub {
	return &ACLPolicyListStub{
		ID:             p.ID,
		Name:           p.Name,
		Description:    p.Description,
		Datacenters:    p.Datacenters,
		Hash:           p.Hash,
		CreateIndex:    p.CreateIndex,
		ModifyIndex:    p.ModifyIndex,
		EnterpriseMeta: p.EnterpriseMeta,
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

		p.EnterpriseMeta.AddToHash(hash, false)

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

	return size + p.EnterpriseMeta.EstimateSize()
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

func (policies ACLPolicies) resolveWithCache(cache *ACLCaches, entConf *acl.Config) ([]*acl.Policy, error) {
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

		p, err := acl.NewPolicyFromSource(policy.Rules, entConf, policy.EnterprisePolicyMeta())
		if err != nil {
			return nil, fmt.Errorf("failed to parse %q: %v", policy.Name, err)
		}

		cache.PutParsedPolicy(cacheKey, p)
		parsed = append(parsed, p)
	}

	return parsed, nil
}

func (policies ACLPolicies) Compile(cache *ACLCaches, entConf *acl.Config) (acl.Authorizer, error) {
	// Determine the cache key
	cacheKey := policies.HashKey()
	entry := cache.GetAuthorizer(cacheKey)
	if entry != nil {
		// the hash key takes into account the policy contents. There is no reason to expire this cache or check its age.
		return entry.Authorizer, nil
	}

	parsed, err := policies.resolveWithCache(cache, entConf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the ACL policies: %v", err)
	}

	// Create the ACL object
	authorizer, err := acl.NewPolicyAuthorizer(parsed, entConf)
	if err != nil {
		return nil, fmt.Errorf("failed to construct ACL Authorizer: %v", err)
	}

	// Update the cache
	cache.PutAuthorizer(cacheKey, authorizer)
	return authorizer, nil
}

type ACLRoles []*ACLRole

// HashKey returns a consistent hash for a set of roles.
func (roles ACLRoles) HashKey() string {
	cacheKeyHash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	for _, role := range roles {
		cacheKeyHash.Write([]byte(role.ID))
		// including the modify index prevents a role set from being
		// cached if one of the roles has changed
		binary.Write(cacheKeyHash, binary.BigEndian, role.ModifyIndex)
	}
	return fmt.Sprintf("%x", cacheKeyHash.Sum(nil))
}

func (roles ACLRoles) Sort() {
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].ID < roles[j].ID
	})
}

type ACLRolePolicyLink struct {
	ID   string
	Name string `hash:"ignore"`
}

type ACLRole struct {
	// ID is the internal UUID associated with the role
	ID string

	// Name is the unique name to reference the role by.
	Name string

	// Description is a human readable description (Optional)
	Description string

	// List of policy links.
	// Note this is the list of IDs and not the names. Prior to role creation
	// the list of policy names gets validated and the policy IDs get stored herein
	Policies []ACLRolePolicyLink `json:",omitempty"`

	// List of services to generate synthetic policies for.
	ServiceIdentities ACLServiceIdentities `json:",omitempty"`

	// List of nodes to generate synthetic policies for.
	NodeIdentities ACLNodeIdentities `json:",omitempty"`

	// Hash of the contents of the role
	// This does not take into account the ID (which is immutable)
	// nor the raft metadata.
	//
	// This is needed mainly for replication purposes. When replicating from
	// one DC to another keeping the content Hash will allow us to avoid
	// unnecessary calls to the authoritative DC
	Hash []byte

	// Embedded Enterprise ACL metadata
	acl.EnterpriseMeta `mapstructure:",squash"`

	// Embedded Raft Metadata
	RaftIndex `hash:"ignore"`
}

func (t *ACLRole) UnmarshalJSON(data []byte) error {
	type Alias ACLRole
	aux := &struct {
		Hash string
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if aux.Hash != "" {
		t.Hash = []byte(aux.Hash)
	}
	return nil
}

func (r *ACLRole) Clone() *ACLRole {
	r2 := *r
	r2.Policies = nil
	r2.ServiceIdentities = nil
	r2.NodeIdentities = nil

	if len(r.Policies) > 0 {
		r2.Policies = make([]ACLRolePolicyLink, len(r.Policies))
		copy(r2.Policies, r.Policies)
	}
	if len(r.ServiceIdentities) > 0 {
		r2.ServiceIdentities = make([]*ACLServiceIdentity, len(r.ServiceIdentities))
		for i, s := range r.ServiceIdentities {
			r2.ServiceIdentities[i] = s.Clone()
		}
	}
	if len(r.NodeIdentities) > 0 {
		r2.NodeIdentities = make([]*ACLNodeIdentity, len(r.NodeIdentities))
		for i, n := range r.NodeIdentities {
			r2.NodeIdentities[i] = n.Clone()
		}
	}
	return &r2
}

func (r *ACLRole) SetHash(force bool) []byte {
	if force || r.Hash == nil {
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
		// The Hash is really only used for replication to determine if a role
		// has changed and should be updated locally.

		// Write all the user set fields
		hash.Write([]byte(r.Name))
		hash.Write([]byte(r.Description))
		for _, link := range r.Policies {
			hash.Write([]byte(link.ID))
		}
		for _, srvid := range r.ServiceIdentities {
			srvid.AddToHash(hash)
		}
		for _, nodeID := range r.NodeIdentities {
			nodeID.AddToHash(hash)
		}

		r.EnterpriseMeta.AddToHash(hash, false)

		// Finalize the hash
		hashVal := hash.Sum(nil)

		// Set and return the hash
		r.Hash = hashVal
	}
	return r.Hash
}

func (r *ACLRole) EstimateSize() int {
	// This is just an estimate. There is other data structure overhead
	// pointers etc that this does not account for.

	// 60 = 36 (uuid) + 16 (RaftIndex) + 8 (Hash)
	size := 60 + len(r.Name) + len(r.Description)
	for _, link := range r.Policies {
		size += len(link.ID) + len(link.Name)
	}
	for _, srvid := range r.ServiceIdentities {
		size += srvid.EstimateSize()
	}
	for _, nodeID := range r.NodeIdentities {
		size += nodeID.EstimateSize()
	}

	return size + r.EnterpriseMeta.EstimateSize()
}

const (
	// BindingRuleBindTypeService is the binding rule bind type that
	// assigns a Service Identity to the token that is created using the value
	// of the computed BindName as the ServiceName like:
	//
	// &ACLToken{
	//   ...other fields...
	//   ServiceIdentities: []*ACLServiceIdentity{
	//     &ACLServiceIdentity{
	//       ServiceName: "<computed BindName>",
	//     },
	//   },
	// }
	BindingRuleBindTypeService = "service"

	// BindingRuleBindTypeRole is the binding rule bind type that only allows
	// the binding rule to function if a role with the given name (BindName)
	// exists at login-time. If it does the token that is created is directly
	// linked to that role like:
	//
	// &ACLToken{
	//   ...other fields...
	//   Roles: []ACLTokenRoleLink{
	//     { Name: "<computed BindName>" }
	//   }
	// }
	//
	// If it does not exist at login-time the rule is ignored.
	BindingRuleBindTypeRole = "role"

	// BindingRuleBindTypeNode is the binding rule bind type that assigns
	// a Node Identity to the token that is created using the value of
	// the computed BindName as the NodeName like:
	//
	// &ACLToken{
	//   ...other fields...
	//   NodeIdentities: []*ACLNodeIdentity{
	//     &ACLNodeIdentity{
	//       NodeName: "<computed BindName>",
	//       Datacenter: "<local datacenter of the binding rule>"
	//     }
	//   }
	// }
	BindingRuleBindTypeNode = "node"
)

type ACLBindingRule struct {
	// ID is the internal UUID associated with the binding rule
	ID string

	// Description is a human readable description (Optional)
	Description string

	// AuthMethod is the name of the auth method for which this rule applies.
	AuthMethod string

	// Selector is an expression that matches against verified identity
	// attributes returned from the auth method during login.
	Selector string

	// BindType adjusts how this binding rule is applied at login time.  The
	// valid values are:
	//
	//  - BindingRuleBindTypeService = "service"
	//  - BindingRuleBindTypeRole    = "role"
	BindType string

	// BindName is the target of the binding. Can be lightly templated using
	// HIL ${foo} syntax from available field names. How it is used depends
	// upon the BindType.
	BindName string

	// Embedded Enterprise ACL metadata
	acl.EnterpriseMeta `mapstructure:",squash"`

	// Embedded Raft Metadata
	RaftIndex `hash:"ignore"`
}

func (r *ACLBindingRule) Clone() *ACLBindingRule {
	r2 := *r
	return &r2
}

type ACLBindingRules []*ACLBindingRule

func (rules ACLBindingRules) Sort() {
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].ID < rules[j].ID
	})
}

// Note: this is a subset of ACLAuthMethod's fields
type ACLAuthMethodListStub struct {
	Name          string
	Type          string
	DisplayName   string        `json:",omitempty"`
	Description   string        `json:",omitempty"`
	MaxTokenTTL   time.Duration `json:",omitempty"`
	TokenLocality string        `json:",omitempty"`
	CreateIndex   uint64
	ModifyIndex   uint64
	acl.EnterpriseMeta
}

func (p *ACLAuthMethod) Stub() *ACLAuthMethodListStub {
	return &ACLAuthMethodListStub{
		Name:           p.Name,
		Type:           p.Type,
		DisplayName:    p.DisplayName,
		Description:    p.Description,
		MaxTokenTTL:    p.MaxTokenTTL,
		TokenLocality:  p.TokenLocality,
		CreateIndex:    p.CreateIndex,
		ModifyIndex:    p.ModifyIndex,
		EnterpriseMeta: p.EnterpriseMeta,
	}
}

// This is nearly identical to the ACLAuthMethod MarshalJSON
// Unmarshaling is not implemented because the API is read only
func (m *ACLAuthMethodListStub) MarshalJSON() ([]byte, error) {
	type Alias ACLAuthMethodListStub
	exported := &struct {
		MaxTokenTTL string `json:",omitempty"`
		*Alias
	}{
		MaxTokenTTL: m.MaxTokenTTL.String(),
		Alias:       (*Alias)(m),
	}
	if m.MaxTokenTTL == 0 {
		exported.MaxTokenTTL = ""
	}

	data, err := json.Marshal(exported)

	return data, err
}

type ACLAuthMethods []*ACLAuthMethod
type ACLAuthMethodListStubs []*ACLAuthMethodListStub

func (methods ACLAuthMethods) Sort() {
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})
}

func (methods ACLAuthMethodListStubs) Sort() {
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})
}

type ACLAuthMethod struct {
	// Name is a unique identifier for this specific auth method.
	//
	// Immutable once set and only settable during create.
	Name string

	// Type is the type of the auth method this is.
	//
	// Immutable once set and only settable during create.
	Type string

	// DisplayName is an optional name to use instead of the Name field when
	// displaying information about this auth method in any kind of user
	// interface.
	DisplayName string `json:",omitempty"`

	// Description is just an optional bunch of explanatory text.
	Description string `json:",omitempty"`

	// MaxTokenTTL this is the maximum life of a token created by this method.
	MaxTokenTTL time.Duration `json:",omitempty"`

	// TokenLocality defines the kind of token that this auth method produces.
	// This can be either 'local' or 'global'. If empty 'local' is assumed.
	TokenLocality string `json:",omitempty"`

	// Configuration is arbitrary configuration for the auth method. This
	// should only contain primitive values and containers (such as lists and
	// maps).
	Config map[string]interface{}

	// Embedded Enterprise ACL Meta
	acl.EnterpriseMeta `mapstructure:",squash"`

	ACLAuthMethodEnterpriseFields `mapstructure:",squash"`

	// Embedded Raft Metadata
	RaftIndex `hash:"ignore"`
}

func (m *ACLAuthMethod) MarshalJSON() ([]byte, error) {
	type Alias ACLAuthMethod
	exported := &struct {
		MaxTokenTTL string `json:",omitempty"`
		*Alias
	}{
		MaxTokenTTL: m.MaxTokenTTL.String(),
		Alias:       (*Alias)(m),
	}
	if m.MaxTokenTTL == 0 {
		exported.MaxTokenTTL = ""
	}

	return json.Marshal(exported)
}

func (m *ACLAuthMethod) UnmarshalJSON(data []byte) (err error) {
	type Alias ACLAuthMethod
	aux := &struct {
		MaxTokenTTL interface{}
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err = lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	if aux.MaxTokenTTL != nil {
		switch v := aux.MaxTokenTTL.(type) {
		case string:
			if m.MaxTokenTTL, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			m.MaxTokenTTL = time.Duration(v)
		}
	}

	return nil
}

type ACLReplicationType string

const (
	ACLReplicatePolicies ACLReplicationType = "policies"
	ACLReplicateRoles    ACLReplicationType = "roles"
	ACLReplicateTokens   ACLReplicationType = "tokens"
)

func (t ACLReplicationType) SingularNoun() string {
	switch t {
	case ACLReplicatePolicies:
		return "policy"
	case ACLReplicateRoles:
		return "role"
	case ACLReplicateTokens:
		return "token"
	default:
		return "<UNKNOWN>"
	}
}

// ACLReplicationStatus provides information about the health of the ACL
// replication system.
type ACLReplicationStatus struct {
	Enabled              bool
	Running              bool
	SourceDatacenter     string
	ReplicationType      ACLReplicationType
	ReplicatedIndex      uint64
	ReplicatedRoleIndex  uint64
	ReplicatedTokenIndex uint64
	LastSuccess          time.Time
	LastError            time.Time
	LastErrorMessage     string
}

// ACLTokenSetRequest is used for token creation and update operations
// at the RPC layer
type ACLTokenSetRequest struct {
	ACLToken   ACLToken // Token to manipulate - I really dislike this name but "Token" is taken in the WriteRequest
	Create     bool     // Used to explicitly mark this request as a creation
	Datacenter string   // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLTokenSetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenGetRequest is used for token read operations at the RPC layer
type ACLTokenGetRequest struct {
	TokenID     string         // Accessor ID used for the token lookup
	TokenIDType ACLTokenIDType // The Type of ID used to lookup the token
	Expanded    bool
	Datacenter  string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLTokenGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenDeleteRequest is used for token deletion operations at the RPC layer
type ACLTokenDeleteRequest struct {
	TokenID    string // Accessor ID of the token to delete
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
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
	Role          string // Role filter
	AuthMethod    string // Auth Method filter
	Datacenter    string // The datacenter to perform the request within
	ACLAuthMethodEnterpriseMeta
	acl.EnterpriseMeta
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
	Tokens               ACLTokens
	CAS                  bool
	AllowMissingLinks    bool
	ProhibitUnprivileged bool
	FromReplication      bool
}

// ACLTokenBatchDeleteRequest is used only at the Raft layer
// for batching multiple token deletions.
//
// This is particularly useful during token replication when
// multiple tokens need to be removed from the local DCs state.
type ACLTokenBatchDeleteRequest struct {
	TokenIDs []string // Tokens to delete
}

type ACLInitialTokenBootstrapRequest struct {
	BootstrapSecret string
	Datacenter      string
	QueryOptions
}

func (r *ACLInitialTokenBootstrapRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLTokenBootstrapRequest is used only at the Raft layer
// for ACL bootstrapping
//
// The RPC layer will use ACLInitialTokenBootstrapRequest to indicate
// that bootstrapping must be performed but the actual token
// and the resetIndex will be generated by that RPC endpoint
type ACLTokenBootstrapRequest struct {
	Token      ACLToken // Token to use for bootstrapping
	ResetIndex uint64   // Reset index
}

// ACLTokenResponse returns a single Token + metadata
type ACLTokenResponse struct {
	Token            *ACLToken
	Redacted         bool // whether the token's secret was redacted
	SourceDatacenter string

	ExpandedTokenInfo
	QueryMeta
}

type ExpandedTokenInfo struct {
	ExpandedPolicies []*ACLPolicy
	ExpandedRoles    []*ACLRole

	NamespaceDefaultPolicyIDs []string
	NamespaceDefaultRoleIDs   []string

	AgentACLDefaultPolicy string
	AgentACLDownPolicy    string
	ResolvedByAgent       string
}

type ACLTokenExpanded struct {
	*ACLToken
	ExpandedTokenInfo
}

// ACLTokenBatchResponse returns multiple Tokens associated with the same metadata
type ACLTokenBatchResponse struct {
	Tokens   []*ACLToken
	Redacted bool // whether the token secrets were redacted.
	Removed  bool // whether any tokens were completely removed
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
	acl.EnterpriseMeta
	WriteRequest
}

func (r *ACLPolicyDeleteRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyGetRequest is used at the RPC layer to perform policy read operations
type ACLPolicyGetRequest struct {
	PolicyID   string // id used for the policy lookup (one of PolicyID or PolicyName is allowed)
	PolicyName string // name used for the policy lookup (one of PolicyID or PolicyName is allowed)
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLPolicyGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLPolicyListRequest is used at the RPC layer to request a listing of policies
type ACLPolicyListRequest struct {
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
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

// ACLRoleSetRequest is used at the RPC layer for creation and update requests
type ACLRoleSetRequest struct {
	Role       ACLRole // The role to upsert
	Datacenter string  // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLRoleSetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLRoleDeleteRequest is used at the RPC layer deletion requests
type ACLRoleDeleteRequest struct {
	RoleID     string // id of the role to delete
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
	WriteRequest
}

func (r *ACLRoleDeleteRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLRoleGetRequest is used at the RPC layer to perform role read operations
type ACLRoleGetRequest struct {
	RoleID     string // id used for the role lookup (one of RoleID or RoleName is allowed)
	RoleName   string // name used for the role lookup (one of RoleID or RoleName is allowed)
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLRoleGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLRoleListRequest is used at the RPC layer to request a listing of roles
type ACLRoleListRequest struct {
	Policy     string // Policy filter
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLRoleListRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLRoleListResponse struct {
	Roles ACLRoles
	QueryMeta
}

// ACLRoleBatchGetRequest is used at the RPC layer to request a subset of
// the roles associated with the token used for retrieval
type ACLRoleBatchGetRequest struct {
	RoleIDs    []string // List of role ids to fetch
	Datacenter string   // The datacenter to perform the request within
	QueryOptions
}

func (r *ACLRoleBatchGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLRoleResponse returns a single role + metadata
type ACLRoleResponse struct {
	Role *ACLRole
	QueryMeta
}

type ACLRoleBatchResponse struct {
	Roles []*ACLRole
	QueryMeta
}

// ACLRoleBatchSetRequest is used at the Raft layer for batching
// multiple role creations and updates
//
// This is particularly useful during replication
type ACLRoleBatchSetRequest struct {
	Roles             ACLRoles
	AllowMissingLinks bool
}

// ACLRoleBatchDeleteRequest is used at the Raft layer for batching
// multiple role deletions
//
// This is particularly useful during replication
type ACLRoleBatchDeleteRequest struct {
	RoleIDs []string
}

// ACLBindingRuleSetRequest is used at the RPC layer for creation and update requests
type ACLBindingRuleSetRequest struct {
	BindingRule ACLBindingRule // The rule to upsert
	Datacenter  string         // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLBindingRuleSetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLBindingRuleDeleteRequest is used at the RPC layer deletion requests
type ACLBindingRuleDeleteRequest struct {
	BindingRuleID string // id of the rule to delete
	Datacenter    string // The datacenter to perform the request within
	acl.EnterpriseMeta
	WriteRequest
}

func (r *ACLBindingRuleDeleteRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLBindingRuleGetRequest is used at the RPC layer to perform rule read operations
type ACLBindingRuleGetRequest struct {
	BindingRuleID string // id used for the rule lookup
	Datacenter    string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLBindingRuleGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLBindingRuleListRequest is used at the RPC layer to request a listing of rules
type ACLBindingRuleListRequest struct {
	AuthMethod string // optional filter
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLBindingRuleListRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLBindingRuleListResponse struct {
	BindingRules ACLBindingRules
	QueryMeta
}

// ACLBindingRuleResponse returns a single binding + metadata
type ACLBindingRuleResponse struct {
	BindingRule *ACLBindingRule
	QueryMeta
}

// ACLBindingRuleBatchSetRequest is used at the Raft layer for batching
// multiple rule creations and updates
type ACLBindingRuleBatchSetRequest struct {
	BindingRules ACLBindingRules
}

// ACLBindingRuleBatchDeleteRequest is used at the Raft layer for batching
// multiple rule deletions
type ACLBindingRuleBatchDeleteRequest struct {
	BindingRuleIDs []string
}

// ACLAuthMethodSetRequest is used at the RPC layer for creation and update requests
type ACLAuthMethodSetRequest struct {
	AuthMethod ACLAuthMethod // The auth method to upsert
	Datacenter string        // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLAuthMethodSetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLAuthMethodDeleteRequest is used at the RPC layer deletion requests
type ACLAuthMethodDeleteRequest struct {
	AuthMethodName string // name of the auth method to delete
	Datacenter     string // The datacenter to perform the request within
	acl.EnterpriseMeta
	WriteRequest
}

func (r *ACLAuthMethodDeleteRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLAuthMethodGetRequest is used at the RPC layer to perform rule read operations
type ACLAuthMethodGetRequest struct {
	AuthMethodName string // name used for the auth method lookup
	Datacenter     string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLAuthMethodGetRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ACLAuthMethodListRequest is used at the RPC layer to request a listing of auth methods
type ACLAuthMethodListRequest struct {
	Datacenter string // The datacenter to perform the request within
	acl.EnterpriseMeta
	QueryOptions
}

func (r *ACLAuthMethodListRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLAuthMethodListResponse struct {
	AuthMethods ACLAuthMethodListStubs
	QueryMeta
}

// ACLAuthMethodResponse returns a single auth method + metadata
type ACLAuthMethodResponse struct {
	AuthMethod *ACLAuthMethod
	QueryMeta
}

// ACLAuthMethodBatchSetRequest is used at the Raft layer for batching
// multiple auth method creations and updates
type ACLAuthMethodBatchSetRequest struct {
	AuthMethods ACLAuthMethods
}

// ACLAuthMethodBatchDeleteRequest is used at the Raft layer for batching
// multiple auth method deletions
type ACLAuthMethodBatchDeleteRequest struct {
	AuthMethodNames []string
	// While it may seem odd that AuthMethodNames is associated with a single
	// EnterpriseMeta, it is okay as this struct is only ever used to
	// delete a single entry. This is because AuthMethods unlike tokens, policies
	// and roles are not replicated between datacenters and therefore never
	// batch applied.
	acl.EnterpriseMeta
}

type ACLLoginParams struct {
	AuthMethod  string
	BearerToken string
	Meta        map[string]string `json:",omitempty"`
	acl.EnterpriseMeta
}

type ACLLoginRequest struct {
	Auth       *ACLLoginParams
	Datacenter string // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLLoginRequest) RequestDatacenter() string {
	return r.Datacenter
}

type ACLLogoutRequest struct {
	Datacenter string // The datacenter to perform the request within
	WriteRequest
}

func (r *ACLLogoutRequest) RequestDatacenter() string {
	return r.Datacenter
}

type RemoteACLAuthorizationRequest struct {
	Datacenter string
	Requests   []ACLAuthorizationRequest
	QueryOptions
}

type ACLAuthorizationRequest struct {
	Resource acl.Resource
	Segment  string `json:",omitempty"`
	Access   string
	acl.EnterpriseMeta
}

type ACLAuthorizationResponse struct {
	ACLAuthorizationRequest
	Allow bool
}

func (r *RemoteACLAuthorizationRequest) RequestDatacenter() string {
	return r.Datacenter
}

func CreateACLAuthorizationResponses(authz acl.Authorizer, requests []ACLAuthorizationRequest) ([]ACLAuthorizationResponse, error) {
	responses := make([]ACLAuthorizationResponse, len(requests))
	var ctx acl.AuthorizerContext

	for idx, req := range requests {
		req.FillAuthzContext(&ctx)
		decision, err := acl.Enforce(authz, req.Resource, req.Segment, req.Access, &ctx)
		if err != nil {
			return nil, err
		}

		responses[idx].ACLAuthorizationRequest = req
		responses[idx].Allow = decision == acl.Allow
	}

	return responses, nil
}

type AgentRecoveryTokenIdentity struct {
	agent    string
	secretID string
}

func NewAgentRecoveryTokenIdentity(agent string, secretID string) *AgentRecoveryTokenIdentity {
	return &AgentRecoveryTokenIdentity{
		agent:    agent,
		secretID: secretID,
	}
}

func (id *AgentRecoveryTokenIdentity) ID() string {
	return fmt.Sprintf("agent-recovery:%s", id.agent)
}

func (id *AgentRecoveryTokenIdentity) SecretToken() string {
	return id.secretID
}

func (id *AgentRecoveryTokenIdentity) PolicyIDs() []string {
	return nil
}

func (id *AgentRecoveryTokenIdentity) RoleIDs() []string {
	return nil
}

func (id *AgentRecoveryTokenIdentity) ServiceIdentityList() []*ACLServiceIdentity {
	return nil
}

func (id *AgentRecoveryTokenIdentity) NodeIdentityList() []*ACLNodeIdentity {
	return nil
}

func (id *AgentRecoveryTokenIdentity) IsExpired(asOf time.Time) bool {
	return false
}

func (id *AgentRecoveryTokenIdentity) IsLocal() bool {
	return true
}

func (id *AgentRecoveryTokenIdentity) EnterpriseMetadata() *acl.EnterpriseMeta {
	return nil
}

const ServerManagementTokenAccessorID = "server-management-token"

type ACLServerIdentity struct {
	secretID string
}

func NewACLServerIdentity(secretID string) *ACLServerIdentity {
	return &ACLServerIdentity{
		secretID: secretID,
	}
}

func (i *ACLServerIdentity) ID() string {
	return ServerManagementTokenAccessorID
}

func (i *ACLServerIdentity) SecretToken() string {
	return i.secretID
}

func (i *ACLServerIdentity) PolicyIDs() []string {
	return nil
}

func (i *ACLServerIdentity) RoleIDs() []string {
	return nil
}

func (i *ACLServerIdentity) ServiceIdentityList() []*ACLServiceIdentity {
	return nil
}

func (i *ACLServerIdentity) NodeIdentityList() []*ACLNodeIdentity {
	return nil
}

func (i *ACLServerIdentity) IsExpired(asOf time.Time) bool {
	return false
}

func (i *ACLServerIdentity) IsLocal() bool {
	return true
}

func (i *ACLServerIdentity) EnterpriseMetadata() *acl.EnterpriseMeta {
	return acl.DefaultEnterpriseMeta()
}

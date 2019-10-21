package consul

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testACLPolicy = `
key "" {
	policy = "deny"
}
key "foo/" {
	policy = "write"
}
`

var testACLPolicyNew = `
key_prefix "" {
	policy = "deny"
}
key_prefix "foo/" {
	policy = "write"
}
`

type asyncResolutionResult struct {
	authz acl.Authorizer
	err   error
}

func verifyAuthorizerChain(t *testing.T, expected acl.Authorizer, actual acl.Authorizer) {
	expectedChainAuthz, ok := expected.(*acl.ChainedAuthorizer)
	require.True(t, ok, "expected Authorizer is not a ChainedAuthorizer")
	actualChainAuthz, ok := actual.(*acl.ChainedAuthorizer)
	require.True(t, ok, "actual Authorizer is not a ChainedAuthorizer")

	expectedChain := expectedChainAuthz.AuthorizerChain()
	actualChain := actualChainAuthz.AuthorizerChain()

	require.Equal(t, len(expectedChain), len(actualChain), "ChainedAuthorizers have different length chains")
	for idx, expectedAuthz := range expectedChain {
		actualAuthz := actualChain[idx]

		// pointer equality - because we want to verify authorizer reuse
		require.True(t, expectedAuthz == actualAuthz, "Authorizer pointers are not equal")
	}
}

func resolveTokenAsync(r *ACLResolver, token string, ch chan *asyncResolutionResult) {
	authz, err := r.ResolveToken(token)
	ch <- &asyncResolutionResult{authz: authz, err: err}
}

func testIdentityForToken(token string) (bool, structs.ACLIdentity, error) {
	switch token {
	case "missing-policy":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "not-found",
				},
				structs.ACLTokenPolicyLink{
					ID: "acl-ro",
				},
			},
		}, nil
	case "missing-role":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Roles: []structs.ACLTokenRoleLink{
				structs.ACLTokenRoleLink{
					ID: "not-found",
				},
				structs.ACLTokenRoleLink{
					ID: "acl-ro",
				},
			},
		}, nil
	case "missing-policy-on-role":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Roles: []structs.ACLTokenRoleLink{
				structs.ACLTokenRoleLink{
					ID: "missing-policy",
				},
			},
		}, nil
	case "legacy-management":
		return true, &structs.ACLToken{
			AccessorID: "d109a033-99d1-47e2-a711-d6593373a973",
			SecretID:   "415cd1e1-1493-4fb4-827d-d762ed9cfe7c",
			Type:       structs.ACLTokenTypeManagement,
		}, nil
	case "legacy-client":
		return true, &structs.ACLToken{
			AccessorID: "b7375838-b104-4a25-b457-329d939bf257",
			SecretID:   "03f49328-c23c-4b26-92a2-3b898332400d",
			Type:       structs.ACLTokenTypeClient,
			Rules:      `service "" { policy = "read" }`,
		}, nil
	case "found":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "node-wr",
				},
				structs.ACLTokenPolicyLink{
					ID: "dc2-key-wr",
				},
			},
		}, nil
	case "found-role":
		// This should be permission-wise identical to "found", except it
		// gets it's policies indirectly by way of a Role.
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Roles: []structs.ACLTokenRoleLink{
				structs.ACLTokenRoleLink{
					ID: "found",
				},
			},
		}, nil
	case "found-policy-and-role":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "node-wr",
				},
				structs.ACLTokenPolicyLink{
					ID: "dc2-key-wr",
				},
			},
			Roles: []structs.ACLTokenRoleLink{
				structs.ACLTokenRoleLink{
					ID: "service-ro",
				},
			},
		}, nil
	case "found-synthetic-policy-1":
		return true, &structs.ACLToken{
			AccessorID: "f6c5a5fb-4da4-422b-9abf-2c942813fc71",
			SecretID:   "55cb7d69-2bea-42c3-a68f-2a1443d2abbc",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				&structs.ACLServiceIdentity{
					ServiceName: "service1",
				},
			},
		}, nil
	case "found-synthetic-policy-2":
		return true, &structs.ACLToken{
			AccessorID: "7c87dfad-be37-446e-8305-299585677cb5",
			SecretID:   "dfca9676-ac80-453a-837b-4c0cf923473c",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				&structs.ACLServiceIdentity{
					ServiceName: "service2",
				},
			},
		}, nil
	case "acl-ro":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "acl-ro",
				},
			},
		}, nil
	case "acl-wr":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "acl-wr",
				},
			},
		}, nil
	case "racey-unmodified":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "node-wr",
				},
				structs.ACLTokenPolicyLink{
					ID: "acl-wr",
				},
			},
		}, nil
	case "racey-modified":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "node-wr",
				},
			},
		}, nil
	case "concurrent-resolve":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "node-wr",
				},
				structs.ACLTokenPolicyLink{
					ID: "acl-wr",
				},
			},
		}, nil
	case anonymousToken:
		return true, &structs.ACLToken{
			AccessorID: "00000000-0000-0000-0000-000000000002",
			SecretID:   anonymousToken,
			Policies: []structs.ACLTokenPolicyLink{
				structs.ACLTokenPolicyLink{
					ID: "node-wr",
				},
			},
		}, nil
	default:
		return testIdentityForTokenEnterprise(token)
	}
}

func testPolicyForID(policyID string) (bool, *structs.ACLPolicy, error) {
	switch policyID {
	case "acl-ro":
		return true, &structs.ACLPolicy{
			ID:          "acl-ro",
			Name:        "acl-ro",
			Description: "acl-ro",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "acl-wr":
		return true, &structs.ACLPolicy{
			ID:          "acl-wr",
			Name:        "acl-wr",
			Description: "acl-wr",
			Rules:       `acl = "write"`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "service-ro":
		return true, &structs.ACLPolicy{
			ID:          "service-ro",
			Name:        "service-ro",
			Description: "service-ro",
			Rules:       `service_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "service-wr":
		return true, &structs.ACLPolicy{
			ID:          "service-wr",
			Name:        "service-wr",
			Description: "service-wr",
			Rules:       `service_prefix "" { policy = "write" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "node-wr":
		return true, &structs.ACLPolicy{
			ID:          "node-wr",
			Name:        "node-wr",
			Description: "node-wr",
			Rules:       `node_prefix "" { policy = "write"}`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc1"},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "dc2-key-wr":
		return true, &structs.ACLPolicy{
			ID:          "dc2-key-wr",
			Name:        "dc2-key-wr",
			Description: "dc2-key-wr",
			Rules:       `key_prefix "" { policy = "write"}`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc2"},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	default:
		return testPolicyForIDEnterprise(policyID)
	}
}

func testRoleForID(roleID string) (bool, *structs.ACLRole, error) {
	switch roleID {
	case "service-ro":
		return true, &structs.ACLRole{
			ID:          "service-ro",
			Name:        "service-ro",
			Description: "service-ro",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "service-ro",
				},
			},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "service-wr":
		return true, &structs.ACLRole{
			ID:          "service-wr",
			Name:        "service-wr",
			Description: "service-wr",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "service-wr",
				},
			},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "missing-policy":
		return true, &structs.ACLRole{
			ID:          "missing-policy",
			Name:        "missing-policy",
			Description: "missing-policy",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "not-found",
				},
				structs.ACLRolePolicyLink{
					ID: "acl-ro",
				},
			},
			RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}, nil
	case "found":
		return true, &structs.ACLRole{
			ID:          "found",
			Name:        "found",
			Description: "found",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "node-wr",
				},
				structs.ACLRolePolicyLink{
					ID: "dc2-key-wr",
				},
			},
		}, nil
	case "acl-ro":
		return true, &structs.ACLRole{
			ID:          "acl-ro",
			Name:        "acl-ro",
			Description: "acl-ro",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "acl-ro",
				},
			},
		}, nil
	case "acl-wr":
		return true, &structs.ACLRole{
			ID:          "acl-rw",
			Name:        "acl-rw",
			Description: "acl-rw",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "acl-wr",
				},
			},
		}, nil
	case "racey-unmodified":
		return true, &structs.ACLRole{
			ID:          "racey-unmodified",
			Name:        "racey-unmodified",
			Description: "racey-unmodified",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "node-wr",
				},
				structs.ACLRolePolicyLink{
					ID: "acl-wr",
				},
			},
		}, nil
	case "racey-modified":
		return true, &structs.ACLRole{
			ID:          "racey-modified",
			Name:        "racey-modified",
			Description: "racey-modified",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "node-wr",
				},
			},
		}, nil
	case "concurrent-resolve-1":
		return true, &structs.ACLRole{
			ID:          "concurrent-resolve-1",
			Name:        "concurrent-resolve-1",
			Description: "concurrent-resolve-1",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "node-wr",
				},
				structs.ACLRolePolicyLink{
					ID: "acl-wr",
				},
			},
		}, nil
	case "concurrent-resolve-2":
		return true, &structs.ACLRole{
			ID:          "concurrent-resolve-2",
			Name:        "concurrent-resolve-2",
			Description: "concurrent-resolve-2",
			Policies: []structs.ACLRolePolicyLink{
				structs.ACLRolePolicyLink{
					ID: "node-wr",
				},
				structs.ACLRolePolicyLink{
					ID: "acl-wr",
				},
			},
		}, nil
	default:
		return testRoleForIDEnterprise(roleID)
	}
}

// ACLResolverTestDelegate is used to test
// the ACLResolver without running Agents
type ACLResolverTestDelegate struct {
	enabled         bool
	datacenter      string
	legacy          bool
	localTokens     bool
	localPolicies   bool
	localRoles      bool
	getPolicyFn     func(*structs.ACLPolicyResolveLegacyRequest, *structs.ACLPolicyResolveLegacyResponse) error
	tokenReadFn     func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error
	policyResolveFn func(*structs.ACLPolicyBatchGetRequest, *structs.ACLPolicyBatchResponse) error
	roleResolveFn   func(*structs.ACLRoleBatchGetRequest, *structs.ACLRoleBatchResponse) error

	// state for the optional default resolver function defaultTokenReadFn
	tokenCached bool
	// state for the optional default resolver function defaultPolicyResolveFn
	policyCached bool
	// state for the optional default resolver function defaultRoleResolveFn
	roleCached bool

	EnterpriseACLResolverTestDelegate
}

func (d *ACLResolverTestDelegate) Reset() {
	d.tokenCached = false
	d.policyCached = false
	d.roleCached = false
}

var errRPC = fmt.Errorf("Induced RPC Error")

func (d *ACLResolverTestDelegate) defaultTokenReadFn(errAfterCached error) func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error {
	return func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
		if !d.tokenCached {
			err := d.plainTokenReadFn(args, reply)
			d.tokenCached = true
			return err
		}
		return errAfterCached
	}
}

func (d *ACLResolverTestDelegate) plainTokenReadFn(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
	_, token, err := testIdentityForToken(args.TokenID)
	if token != nil {
		reply.Token = token.(*structs.ACLToken)
	}
	return err
}

func (d *ACLResolverTestDelegate) defaultPolicyResolveFn(errAfterCached error) func(*structs.ACLPolicyBatchGetRequest, *structs.ACLPolicyBatchResponse) error {
	return func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
		if !d.policyCached {
			err := d.plainPolicyResolveFn(args, reply)
			d.policyCached = true
			return err
		}

		return errAfterCached
	}
}

func (d *ACLResolverTestDelegate) plainPolicyResolveFn(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
	// TODO: if we were being super correct about it, we'd verify the token first
	// TODO: and possibly return a not-found or permission-denied here

	for _, policyID := range args.PolicyIDs {
		_, policy, _ := testPolicyForID(policyID)
		if policy != nil {
			reply.Policies = append(reply.Policies, policy)
		}
	}

	return nil
}

func (d *ACLResolverTestDelegate) defaultRoleResolveFn(errAfterCached error) func(*structs.ACLRoleBatchGetRequest, *structs.ACLRoleBatchResponse) error {
	return func(args *structs.ACLRoleBatchGetRequest, reply *structs.ACLRoleBatchResponse) error {
		if !d.roleCached {
			err := d.plainRoleResolveFn(args, reply)
			d.roleCached = true
			return err
		}

		return errAfterCached
	}
}

// plainRoleResolveFn tries to follow the normal logic of ACL.RoleResolve using
// the test fixtures.
func (d *ACLResolverTestDelegate) plainRoleResolveFn(args *structs.ACLRoleBatchGetRequest, reply *structs.ACLRoleBatchResponse) error {
	// TODO: if we were being super correct about it, we'd verify the token first
	// TODO: and possibly return a not-found or permission-denied here

	for _, roleID := range args.RoleIDs {
		_, role, _ := testRoleForID(roleID)
		if role != nil {
			reply.Roles = append(reply.Roles, role)
		}
	}

	return nil
}

func (d *ACLResolverTestDelegate) ACLsEnabled() bool {
	return d.enabled
}

func (d *ACLResolverTestDelegate) ACLDatacenter(legacy bool) string {
	return d.datacenter
}

func (d *ACLResolverTestDelegate) UseLegacyACLs() bool {
	return d.legacy
}

func (d *ACLResolverTestDelegate) ResolveIdentityFromToken(token string) (bool, structs.ACLIdentity, error) {
	if !d.localTokens {
		return false, nil, nil
	}

	return testIdentityForToken(token)
}

func (d *ACLResolverTestDelegate) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	if !d.localPolicies {
		return false, nil, nil
	}

	return testPolicyForID(policyID)
}

func (d *ACLResolverTestDelegate) ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error) {
	if !d.localRoles {
		return false, nil, nil
	}

	return testRoleForID(roleID)
}

func (d *ACLResolverTestDelegate) RPC(method string, args interface{}, reply interface{}) error {
	switch method {
	case "ACL.GetPolicy":
		if d.getPolicyFn != nil {
			return d.getPolicyFn(args.(*structs.ACLPolicyResolveLegacyRequest), reply.(*structs.ACLPolicyResolveLegacyResponse))
		}
		panic("Bad Test Implementation: should provide a getPolicyFn to the ACLResolverTestDelegate")
	case "ACL.TokenRead":
		if d.tokenReadFn != nil {
			return d.tokenReadFn(args.(*structs.ACLTokenGetRequest), reply.(*structs.ACLTokenResponse))
		}
		panic("Bad Test Implementation: should provide a tokenReadFn to the ACLResolverTestDelegate")
	case "ACL.PolicyResolve":
		if d.policyResolveFn != nil {
			return d.policyResolveFn(args.(*structs.ACLPolicyBatchGetRequest), reply.(*structs.ACLPolicyBatchResponse))
		}
		panic("Bad Test Implementation: should provide a policyResolveFn to the ACLResolverTestDelegate")
	case "ACL.RoleResolve":
		if d.roleResolveFn != nil {
			return d.roleResolveFn(args.(*structs.ACLRoleBatchGetRequest), reply.(*structs.ACLRoleBatchResponse))
		}
		panic("Bad Test Implementation: should provide a roleResolveFn to the ACLResolverTestDelegate")
	}
	if handled, err := d.EnterpriseACLResolverTestDelegate.RPC(method, args, reply); handled {
		return err
	}
	panic("Bad Test Implementation: Was the ACLResolver updated to use new RPC methods")
}

func newTestACLResolver(t *testing.T, delegate ACLResolverDelegate, cb func(*ACLResolverConfig)) *ACLResolver {
	config := DefaultConfig()
	config.ACLDefaultPolicy = "deny"
	config.ACLDownPolicy = "extend-cache"
	rconf := &ACLResolverConfig{
		Config: config,
		Logger: testutil.LoggerWithName(t, t.Name()),
		CacheConfig: &structs.ACLCachesConfig{
			Identities:     4,
			Policies:       4,
			ParsedPolicies: 4,
			Authorizers:    4,
			Roles:          4,
		},
		AutoDisable: true,
		Delegate:    delegate,
	}

	if cb != nil {
		cb(rconf)
	}

	resolver, err := NewACLResolver(rconf)
	require.NoError(t, err)
	return resolver
}

func TestACLResolver_Disabled(t *testing.T) {
	t.Parallel()

	delegate := &ACLResolverTestDelegate{
		enabled:    false,
		datacenter: "dc1",
		legacy:     false,
	}

	r := newTestACLResolver(t, delegate, nil)

	authz, err := r.ResolveToken("does not exist")
	require.Nil(t, authz)
	require.Nil(t, err)
}

func TestACLResolver_ResolveRootACL(t *testing.T) {
	t.Parallel()
	delegate := &ACLResolverTestDelegate{
		enabled:    true,
		datacenter: "dc1",
		legacy:     false,
	}
	r := newTestACLResolver(t, delegate, nil)

	t.Run("Allow", func(t *testing.T) {
		authz, err := r.ResolveToken("allow")
		require.Nil(t, authz)
		require.Error(t, err)
		require.True(t, acl.IsErrRootDenied(err))
	})

	t.Run("Deny", func(t *testing.T) {
		authz, err := r.ResolveToken("deny")
		require.Nil(t, authz)
		require.Error(t, err)
		require.True(t, acl.IsErrRootDenied(err))
	})

	t.Run("Manage", func(t *testing.T) {
		authz, err := r.ResolveToken("manage")
		require.Nil(t, authz)
		require.Error(t, err)
		require.True(t, acl.IsErrRootDenied(err))
	})
}

func TestACLResolver_DownPolicy(t *testing.T) {
	t.Parallel()

	requireIdentityCached := func(t *testing.T, r *ACLResolver, id string, present bool, msg string) {
		t.Helper()

		cacheVal := r.cache.GetIdentity(id)
		require.NotNil(t, cacheVal)
		if present {
			require.NotNil(t, cacheVal.Identity, msg)
		} else {
			require.Nil(t, cacheVal.Identity, msg)
		}
	}
	requirePolicyCached := func(t *testing.T, r *ACLResolver, policyID string, present bool, msg string) {
		t.Helper()

		cacheVal := r.cache.GetPolicy(policyID)
		require.NotNil(t, cacheVal)
		if present {
			require.NotNil(t, cacheVal.Policy, msg)
		} else {
			require.Nil(t, cacheVal.Policy, msg)
		}
	}

	t.Run("Deny", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			localRoles:    true,
			tokenReadFn: func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error {
				return errRPC
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "deny"
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, authz, acl.DenyAll())

		requireIdentityCached(t, r, tokenSecretCacheID("foo"), false, "not present")
	})

	t.Run("Allow", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			localRoles:    true,
			tokenReadFn: func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error {
				return errRPC
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "allow"
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, authz, acl.AllowAll())

		requireIdentityCached(t, r, tokenSecretCacheID("foo"), false, "not present")
	})

	t.Run("Expired-Policy", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			localRoles:    false,
		}
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "deny"
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		// policy cache expired - so we will fail to resolve that policy and use the default policy only
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		require.NotEqual(t, authz, authz2)
		require.Equal(t, acl.Deny, authz2.NodeWrite("foo", nil))

		requirePolicyCached(t, r, "node-wr", false, "expired")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", false, "expired") // from "found" token
	})

	t.Run("Expired-Role", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			localRoles:    false,
		}
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(errRPC)
		delegate.roleResolveFn = delegate.defaultRoleResolveFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "deny"
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		// role cache expired - so we will fail to resolve that role and use the default policy only
		authz2, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		require.False(t, authz == authz2)
		require.Equal(t, acl.Deny, authz2.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache-Policy", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			localRoles:    true,
		}
		delegate.tokenReadFn = delegate.defaultTokenReadFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requireIdentityCached(t, r, tokenSecretCacheID("found"), true, "cached")

		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache-Role", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			localRoles:    true,
		}
		delegate.tokenReadFn = delegate.defaultTokenReadFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requireIdentityCached(t, r, tokenSecretCacheID("found-role"), true, "still cached")

		authz2, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache-Expired-Policy", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			localRoles:    false,
		}
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		// Will just use the policy cache
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requirePolicyCached(t, r, "node-wr", true, "still cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "still cached") // from "found" token
	})

	t.Run("Extend-Cache-Expired-Role", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			localRoles:    false,
		}
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(errRPC)
		delegate.roleResolveFn = delegate.defaultRoleResolveFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		// Will just use the policy cache
		authz2, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		verifyAuthorizerChain(t, authz, authz2)

		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	t.Run("Async-Cache-Expired-Policy", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			localRoles:    false,
		}
		// We don't need to return acl.ErrNotFound here but we could. The ACLResolver will search for any
		// policies not in the response and emit an ACL not found for any not-found within the result set.
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(nil)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "async-cache"
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		// The identity should have been cached so this should still be valid
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		// the go routine spawned will eventually return with a authz that doesn't have the policy
		retry.Run(t, func(t *retry.R) {
			authz3, err := r.ResolveToken("found")
			assert.NoError(t, err)
			assert.NotNil(t, authz3)
			assert.Equal(t, acl.Deny, authz3.NodeWrite("foo", nil))
		})

		requirePolicyCached(t, r, "node-wr", false, "no longer cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", false, "no longer cached") // from "found" token
	})

	t.Run("Async-Cache-Expired-Role", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			localRoles:    false,
		}
		// We don't need to return acl.ErrNotFound here but we could. The ACLResolver will search for any
		// policies not in the response and emit an ACL not found for any not-found within the result set.
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(nil)
		delegate.roleResolveFn = delegate.defaultRoleResolveFn(nil)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "async-cache"
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		// The identity should have been cached so this should still be valid
		authz2, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		// the go routine spawned will eventually return with a authz that doesn't have the policy
		retry.Run(t, func(t *retry.R) {
			authz3, err := r.ResolveToken("found-role")
			assert.NoError(t, err)
			assert.NotNil(t, authz3)
			assert.Equal(t, acl.Deny, authz3.NodeWrite("foo", nil))
		})
	})

	t.Run("Extend-Cache-Client-Policy", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
			localRoles:    false,
		}
		delegate.tokenReadFn = delegate.defaultTokenReadFn(errRPC)
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache-Client-Role", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
			localRoles:    false,
		}
		delegate.tokenReadFn = delegate.defaultTokenReadFn(errRPC)
		delegate.policyResolveFn = delegate.defaultPolicyResolveFn(errRPC)
		delegate.roleResolveFn = delegate.defaultRoleResolveFn(errRPC)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
			config.Config.ACLPolicyTTL = 0
			config.Config.ACLRoleTTL = 0
		})

		authz, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requirePolicyCached(t, r, "node-wr", true, "still cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "still cached") // from "found" token

		authz2, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))
	})

	t.Run("Async-Cache", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			localRoles:    true,
		}
		delegate.tokenReadFn = delegate.defaultTokenReadFn(acl.ErrNotFound)

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "async-cache"
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		requireIdentityCached(t, r, tokenSecretCacheID("found"), true, "cached")

		// The identity should have been cached so this should still be valid
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))

		// the go routine spawned will eventually return and this will be a not found error
		retry.Run(t, func(t *retry.R) {
			authz3, err := r.ResolveToken("found")
			assert.Error(t, err)
			assert.True(t, acl.IsErrNotFound(err))
			assert.Nil(t, authz3)
		})

		requireIdentityCached(t, r, tokenSecretCacheID("found"), false, "no longer cached")
	})

	t.Run("PolicyResolve-TokenNotFound", func(t *testing.T) {
		t.Parallel()

		_, rawToken, _ := testIdentityForToken("found")
		foundToken := rawToken.(*structs.ACLToken)
		secretID := foundToken.SecretID

		tokenResolved := false
		policyResolved := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
			tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				if !tokenResolved {
					reply.Token = foundToken
					tokenResolved = true
					return nil
				}

				return fmt.Errorf("Not Supposed to be Invoked again")
			},
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				if !policyResolved {
					for _, policyID := range args.PolicyIDs {
						_, policy, _ := testPolicyForID(policyID)
						if policy != nil {
							reply.Policies = append(reply.Policies, policy)
						}
					}
					policyResolved = true
					return nil
				}
				return acl.ErrNotFound // test condition
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
			config.Config.ACLPolicyTTL = 0
		})

		// Prime the standard caches.
		authz, err := r.ResolveToken(secretID)
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		// Verify that the caches are setup properly.
		requireIdentityCached(t, r, tokenSecretCacheID(secretID), true, "cached")
		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		// Nuke 1 policy from the cache so that we force a policy resolve
		// during token resolve.
		r.cache.RemovePolicy("dc2-key-wr")

		_, err = r.ResolveToken(secretID)
		require.True(t, acl.IsErrNotFound(err))

		requireIdentityCached(t, r, tokenSecretCacheID(secretID), false, "identity not found cached")
		requirePolicyCached(t, r, "node-wr", true, "still cached")
		require.Nil(t, r.cache.GetPolicy("dc2-key-wr"), "not stored at all")
	})

	t.Run("PolicyResolve-PermissionDenied", func(t *testing.T) {
		t.Parallel()

		_, rawToken, _ := testIdentityForToken("found")
		foundToken := rawToken.(*structs.ACLToken)
		secretID := foundToken.SecretID

		policyResolved := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
			tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				// no limit
				reply.Token = foundToken
				return nil
			},
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				if !policyResolved {
					for _, policyID := range args.PolicyIDs {
						_, policy, _ := testPolicyForID(policyID)
						if policy != nil {
							reply.Policies = append(reply.Policies, policy)
						}
					}
					policyResolved = true
					return nil
				}
				return acl.ErrPermissionDenied // test condition
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
			config.Config.ACLPolicyTTL = 0
		})

		// Prime the standard caches.
		authz, err := r.ResolveToken(secretID)
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))

		// Verify that the caches are setup properly.
		requireIdentityCached(t, r, tokenSecretCacheID(secretID), true, "cached")
		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		// Nuke 1 policy from the cache so that we force a policy resolve
		// during token resolve.
		r.cache.RemovePolicy("dc2-key-wr")

		_, err = r.ResolveToken(secretID)
		require.True(t, acl.IsErrPermissionDenied(err))

		require.Nil(t, r.cache.GetIdentity(tokenSecretCacheID(secretID)), "identity not stored at all")
		requirePolicyCached(t, r, "node-wr", true, "still cached")
		require.Nil(t, r.cache.GetPolicy("dc2-key-wr"), "not stored at all")
	})
}

func TestACLResolver_DatacenterScoping(t *testing.T) {
	t.Parallel()
	t.Run("dc1", func(t *testing.T) {
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: true,
			localRoles:    true,
			// No need to provide any of the RPC callbacks
		}
		r := newTestACLResolver(t, delegate, nil)

		authz, err := r.ResolveToken("found")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Deny, authz.KeyWrite("foo", nil))
	})

	t.Run("dc2", func(t *testing.T) {
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc2",
			legacy:        false,
			localTokens:   true,
			localPolicies: true,
			localRoles:    true,
			// No need to provide any of the RPC callbacks
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.Datacenter = "dc2"
		})

		authz, err := r.ResolveToken("found")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.KeyWrite("foo", nil))
	})
}

// TODO(rb): replicate this sort of test but for roles
func TestACLResolver_Client(t *testing.T) {
	t.Parallel()

	t.Run("Racey-Token-Mod-Policy-Resolve", func(t *testing.T) {
		t.Parallel()
		var tokenReads int32
		var policyResolves int32
		modified := false
		deleted := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
			tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				atomic.AddInt32(&tokenReads, 1)
				if deleted {
					return acl.ErrNotFound
				} else if modified {
					_, token, _ := testIdentityForToken("racey-modified")
					reply.Token = token.(*structs.ACLToken)
				} else {
					_, token, _ := testIdentityForToken("racey-unmodified")
					reply.Token = token.(*structs.ACLToken)
				}
				return nil
			},
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				atomic.AddInt32(&policyResolves, 1)
				if deleted {
					return acl.ErrNotFound
				} else if !modified {
					modified = true
					return acl.ErrPermissionDenied
				} else {
					deleted = true
					for _, policyID := range args.PolicyIDs {
						_, policy, _ := testPolicyForID(policyID)
						if policy != nil {
							reply.Policies = append(reply.Policies, policy)
						}
					}

					modified = true
					return nil
				}
			},
		}

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLTokenTTL = 600 * time.Second
			config.Config.ACLPolicyTTL = 30 * time.Millisecond
			config.Config.ACLRoleTTL = 30 * time.Millisecond
			config.Config.ACLDownPolicy = "extend-cache"
		})

		// resolves the token
		// gets a permission denied resolving the policies - token updated
		// invalidates the token
		// refetches the token
		// fetches the policies from the modified token
		// creates the authorizers
		//
		// Must use the token secret here in order for the cached identity
		// to be removed properly. Many other tests just resolve some other
		// random name and it wont matter but this one cannot.
		authz, err := r.ResolveToken("a1a54629-5050-4d17-8a4e-560d2423f835")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.True(t, modified)
		require.True(t, deleted)
		require.Equal(t, int32(2), tokenReads)
		require.Equal(t, int32(2), policyResolves)

		// sleep long enough for the policy cache to expire
		time.Sleep(50 * time.Millisecond)

		// this round the identity will be resolved from the cache
		// then the policy will be resolved but resolution will return ACL not found
		// resolution will stop with the not found error (even though we still have the
		// policies within the cache)
		authz, err = r.ResolveToken("a1a54629-5050-4d17-8a4e-560d2423f835")
		require.EqualError(t, err, acl.ErrNotFound.Error())
		require.Nil(t, authz)

		require.True(t, modified)
		require.True(t, deleted)
		require.Equal(t, tokenReads, int32(2))
		require.Equal(t, policyResolves, int32(3))
	})

	t.Run("Concurrent-Token-Resolve", func(t *testing.T) {
		t.Parallel()

		var tokenReads int32
		var policyResolves int32
		readyCh := make(chan struct{})

		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
			tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				atomic.AddInt32(&tokenReads, 1)

				switch args.TokenID {
				case "a1a54629-5050-4d17-8a4e-560d2423f835":
					_, token, _ := testIdentityForToken("concurrent-resolve")
					reply.Token = token.(*structs.ACLToken)
				default:
					return acl.ErrNotFound
				}

				select {
				case <-readyCh:
				}
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				atomic.AddInt32(&policyResolves, 1)

				for _, policyID := range args.PolicyIDs {
					_, policy, _ := testPolicyForID(policyID)
					if policy != nil {
						reply.Policies = append(reply.Policies, policy)
					}
				}
				return nil
			},
		}

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			// effectively disable caching - so the only way we end up with 1 token read is if they were
			// being resolved concurrently
			config.Config.ACLTokenTTL = 0 * time.Second
			config.Config.ACLPolicyTTL = 30 * time.Millisecond
			config.Config.ACLRoleTTL = 30 * time.Millisecond
			config.Config.ACLDownPolicy = "extend-cache"
		})

		ch1 := make(chan *asyncResolutionResult)
		ch2 := make(chan *asyncResolutionResult)
		go resolveTokenAsync(r, "a1a54629-5050-4d17-8a4e-560d2423f835", ch1)
		go resolveTokenAsync(r, "a1a54629-5050-4d17-8a4e-560d2423f835", ch2)
		close(readyCh)

		res1 := <-ch1
		res2 := <-ch2
		require.NoError(t, res1.err)
		require.NoError(t, res2.err)
		require.Equal(t, res1.authz, res2.authz)
		require.Equal(t, int32(1), tokenReads)
		require.Equal(t, int32(1), policyResolves)
	})
}

func TestACLResolver_Client_TokensPoliciesAndRoles(t *testing.T) {
	t.Parallel()
	delegate := &ACLResolverTestDelegate{
		enabled:       true,
		datacenter:    "dc1",
		legacy:        false,
		localTokens:   false,
		localPolicies: false,
		localRoles:    false,
	}
	delegate.tokenReadFn = delegate.plainTokenReadFn
	delegate.policyResolveFn = delegate.plainPolicyResolveFn
	delegate.roleResolveFn = delegate.plainRoleResolveFn

	testACLResolver_variousTokens(t, delegate)
}

func TestACLResolver_LocalTokensPoliciesAndRoles(t *testing.T) {
	t.Parallel()
	delegate := &ACLResolverTestDelegate{
		enabled:       true,
		datacenter:    "dc1",
		legacy:        false,
		localTokens:   true,
		localPolicies: true,
		localRoles:    true,
		// No need to provide any of the RPC callbacks
	}

	testACLResolver_variousTokens(t, delegate)
}

func TestACLResolver_LocalPoliciesAndRoles(t *testing.T) {
	t.Parallel()

	delegate := &ACLResolverTestDelegate{
		enabled:       true,
		datacenter:    "dc1",
		legacy:        false,
		localTokens:   false,
		localPolicies: true,
		localRoles:    true,
	}
	delegate.tokenReadFn = delegate.plainTokenReadFn

	testACLResolver_variousTokens(t, delegate)
}

func testACLResolver_variousTokens(t *testing.T, delegate *ACLResolverTestDelegate) {
	t.Helper()
	r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
		config.Config.ACLTokenTTL = 600 * time.Second
		config.Config.ACLPolicyTTL = 30 * time.Millisecond
		config.Config.ACLRoleTTL = 30 * time.Millisecond
		config.Config.ACLDownPolicy = "extend-cache"
	})
	reset := func() {
		// prevent subtest bleedover
		r.cache.Purge()
		delegate.Reset()
	}

	runTwiceAndReset := func(name string, f func(t *testing.T)) {
		t.Helper()
		defer reset() // reset the stateful resolve AND blow away the cache

		t.Run(name+" (no-cache)", f)
		delegate.Reset() // allow the stateful resolve functions to reset
		t.Run(name+" (cached)", f)
	}

	runTwiceAndReset("Missing Identity", func(t *testing.T) {
		authz, err := r.ResolveToken("doesn't exist")
		require.Nil(t, authz)
		require.Error(t, err)
		require.True(t, acl.IsErrNotFound(err))
	})

	runTwiceAndReset("Missing Policy", func(t *testing.T) {
		authz, err := r.ResolveToken("missing-policy")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Missing Role", func(t *testing.T) {
		authz, err := r.ResolveToken("missing-role")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Missing Policy on Role", func(t *testing.T) {
		authz, err := r.ResolveToken("missing-policy-on-role")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Policy", func(t *testing.T) {
		authz, err := r.ResolveToken("found")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Role", func(t *testing.T) {
		authz, err := r.ResolveToken("found-role")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Policy and Role", func(t *testing.T) {
		authz, err := r.ResolveToken("found-policy-and-role")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("bar", nil))
	})

	runTwiceAndReset("Synthetic Policies Independently Cache", func(t *testing.T) {
		// We resolve both of these tokens in the same cache session
		// to verify that the keys for caching synthetic policies don't bleed
		// over between each other.
		{
			authz, err := r.ResolveToken("found-synthetic-policy-1")
			require.NotNil(t, authz)
			require.NoError(t, err)
			// spot check some random perms
			require.Equal(t, acl.Deny, authz.ACLRead(nil))
			require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
			// ensure we didn't bleed over to the other synthetic policy
			require.Equal(t, acl.Deny, authz.ServiceWrite("service2", nil))
			// check our own synthetic policy
			require.Equal(t, acl.Allow, authz.ServiceWrite("service1", nil))
			require.Equal(t, acl.Allow, authz.ServiceRead("literally-anything", nil))
			require.Equal(t, acl.Allow, authz.NodeRead("any-node", nil))
		}
		{
			authz, err := r.ResolveToken("found-synthetic-policy-2")
			require.NotNil(t, authz)
			require.NoError(t, err)
			// spot check some random perms
			require.Equal(t, acl.Deny, authz.ACLRead(nil))
			require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
			// ensure we didn't bleed over to the other synthetic policy
			require.Equal(t, acl.Deny, authz.ServiceWrite("service1", nil))
			// check our own synthetic policy
			require.Equal(t, acl.Allow, authz.ServiceWrite("service2", nil))
			require.Equal(t, acl.Allow, authz.ServiceRead("literally-anything", nil))
			require.Equal(t, acl.Allow, authz.NodeRead("any-node", nil))
		}
	})

	runTwiceAndReset("Anonymous", func(t *testing.T) {
		authz, err := r.ResolveToken("")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("legacy-management", func(t *testing.T) {
		authz, err := r.ResolveToken("legacy-management")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Allow, authz.ACLWrite(nil))
		require.Equal(t, acl.Allow, authz.KeyRead("foo", nil))
	})

	runTwiceAndReset("legacy-client", func(t *testing.T) {
		authz, err := r.ResolveToken("legacy-client")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.OperatorRead(nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("foo", nil))
	})
}

func TestACLResolver_Legacy(t *testing.T) {
	t.Parallel()

	t.Run("Cached", func(t *testing.T) {
		t.Parallel()
		cached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        true,
			localTokens:   false,
			localPolicies: false,
			getPolicyFn: func(args *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 30
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								&acl.NodeRule{
									Name:   "foo",
									Policy: acl.PolicyWrite,
								},
							},
						},
					}
					cached = true
					return nil
				}
				return errRPC
			},
		}
		r := newTestACLResolver(t, delegate, nil)

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))
	})

	t.Run("Cache-Expiry-Extend", func(t *testing.T) {
		t.Parallel()
		cached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        true,
			localTokens:   false,
			localPolicies: false,
			getPolicyFn: func(args *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								&acl.NodeRule{
									Name:   "foo",
									Policy: acl.PolicyWrite,
								},
							},
						},
					}
					cached = true
					return nil
				}
				return errRPC
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))
	})

	t.Run("Cache-Expiry-Allow", func(t *testing.T) {
		t.Parallel()
		cached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        true,
			localTokens:   false,
			localPolicies: false,
			getPolicyFn: func(args *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								&acl.NodeRule{
									Name:   "foo",
									Policy: acl.PolicyWrite,
								},
							},
						},
					}
					cached = true
					return nil
				}
				return errRPC
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "allow"
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("fo", nil))
	})

	t.Run("Cache-Expiry-Deny", func(t *testing.T) {
		t.Parallel()
		cached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        true,
			localTokens:   false,
			localPolicies: false,
			getPolicyFn: func(args *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								&acl.NodeRule{
									Name:   "foo",
									Policy: acl.PolicyWrite,
								},
							},
						},
					}
					cached = true
					return nil
				}
				return errRPC
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "deny"
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))
	})

	t.Run("Cache-Expiry-Async-Cache", func(t *testing.T) {
		t.Parallel()
		cached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        true,
			localTokens:   false,
			localPolicies: false,
			getPolicyFn: func(args *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								&acl.NodeRule{
									Name:   "foo",
									Policy: acl.PolicyWrite,
								},
							},
						},
					}
					cached = true
					return nil
				}
				return acl.ErrNotFound
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "async-cache"
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo/bar", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("fo", nil))

		// delivered from the cache
		authz2, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz == authz2)

		// the go routine spawned will eventually return and this will be a not found error
		retry.Run(t, func(t *retry.R) {
			authz3, err := r.ResolveToken("foo")
			assert.Error(t, err)
			assert.True(t, acl.IsErrNotFound(err))
			assert.Nil(t, authz3)
		})
	})
}

/*

func TestACL_Replication(t *testing.T) {
	t.Parallel()
	aclExtendPolicies := []string{"extend-cache", "async-cache"} //"async-cache"

	for _, aclDownPolicy := range aclExtendPolicies {
		dir1, s1 := testServerWithConfig(t, func(c *Config) {
			c.ACLDatacenter = "dc1"
			c.ACLMasterToken = "root"
		})
		defer os.RemoveAll(dir1)
		defer s1.Shutdown()
		client := rpcClient(t, s1)
		defer client.Close()

		dir2, s2 := testServerWithConfig(t, func(c *Config) {
			c.Datacenter = "dc2"
			c.ACLDatacenter = "dc1"
			c.ACLDefaultPolicy = "deny"
			c.ACLDownPolicy = aclDownPolicy
			c.ACLTokenReplication = true
			c.ACLReplicationRate = 100
			c.ACLReplicationBurst = 100
			c.ACLReplicationApplyLimit = 1000000
		})
		s2.tokens.UpdateReplicationToken("root")
		defer os.RemoveAll(dir2)
		defer s2.Shutdown()

		dir3, s3 := testServerWithConfig(t, func(c *Config) {
			c.Datacenter = "dc3"
			c.ACLDatacenter = "dc1"
			c.ACLDownPolicy = "deny"
			c.ACLTokenReplication = true
			c.ACLReplicationRate = 100
			c.ACLReplicationBurst = 100
			c.ACLReplicationApplyLimit = 1000000
		})
		s3.tokens.UpdateReplicationToken("root")
		defer os.RemoveAll(dir3)
		defer s3.Shutdown()

		// Try to join.
		joinWAN(t, s2, s1)
		joinWAN(t, s3, s1)
		testrpc.WaitForLeader(t, s1.RPC, "dc1")
		testrpc.WaitForLeader(t, s1.RPC, "dc2")
		testrpc.WaitForLeader(t, s1.RPC, "dc3")

		// Create a new token.
		arg := structs.ACLRequest{
			Datacenter: "dc1",
			Op:         structs.ACLSet,
			ACL: structs.ACL{
				Name:  "User token",
				Type:  structs.ACLTokenTypeClient,
				Rules: testACLPolicy,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var id string
		if err := s1.RPC("ACL.Apply", &arg, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
		// Wait for replication to occur.
		retry.Run(t, func(r *retry.R) {
			_, acl, err := s2.fsm.State().ACLTokenGetBySecret(nil, id)
			if err != nil {
				r.Fatal(err)
			}
			if acl == nil {
				r.Fatal(nil)
			}
			_, acl, err = s3.fsm.State().ACLTokenGetBySecret(nil, id)
			if err != nil {
				r.Fatal(err)
			}
			if acl == nil {
				r.Fatal(nil)
			}
		})

		// Kill the ACL datacenter.
		s1.Shutdown()

		// Token should resolve on s2, which has replication + extend-cache.
		acl, err := s2.ResolveToken(id)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if acl == nil {
			t.Fatalf("missing acl")
		}

		// Check the policy
		if acl.KeyRead("bar") {
			t.Fatalf("unexpected read")
		}
		if !acl.KeyRead("foo/test") {
			t.Fatalf("unexpected failed read")
		}

		// Although s3 has replication, and we verified that the ACL is there,
		// it can not be used because of the down policy.
		acl, err = s3.ResolveToken(id)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if acl == nil {
			t.Fatalf("missing acl")
		}

		// Check the policy.
		if acl.KeyRead("bar") {
			t.Fatalf("unexpected read")
		}
		if acl.KeyRead("foo/test") {
			t.Fatalf("unexpected read")
		}
	}
}

func TestACL_MultiDC_Found(t *testing.T) {
	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.ACLDatacenter = "dc1"
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.ACLDatacenter = "dc1" // Enable ACLs!
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	joinWAN(t, s2, s1)

	testrpc.WaitForLeader(t, s1.RPC, "dc1")
	testrpc.WaitForLeader(t, s1.RPC, "dc2")

	// Create a new token
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTokenTypeClient,
			Rules: testACLPolicy,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var id string
	if err := s1.RPC("ACL.Apply", &arg, &id); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Token should resolve
	acl, err := s2.ResolveToken(id)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if acl == nil {
		t.Fatalf("missing acl")
	}

	// Check the policy
	if acl.KeyRead("bar") {
		t.Fatalf("unexpected read")
	}
	if !acl.KeyRead("foo/test") {
		t.Fatalf("unexpected failed read")
	}
}
*/

func TestACL_filterHealthChecks(t *testing.T) {
	t.Parallel()
	// Create some health checks.
	fill := func() structs.HealthChecks {
		return structs.HealthChecks{
			&structs.HealthCheck{
				Node:        "node1",
				CheckID:     "check1",
				ServiceName: "foo",
			},
		}
	}

	// Try permissive filtering.
	{
		hc := fill()
		filt := newACLFilter(acl.AllowAll(), nil, false)
		filt.filterHealthChecks(&hc)
		if len(hc) != 1 {
			t.Fatalf("bad: %#v", hc)
		}
	}

	// Try restrictive filtering.
	{
		hc := fill()
		filt := newACLFilter(acl.DenyAll(), nil, false)
		filt.filterHealthChecks(&hc)
		if len(hc) != 0 {
			t.Fatalf("bad: %#v", hc)
		}
	}

	// Allowed to see the service but not the node.
	policy, err := acl.NewPolicyFromSource("", 0, `
service "foo" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// This will work because version 8 ACLs aren't being enforced.
	{
		hc := fill()
		filt := newACLFilter(perms, nil, false)
		filt.filterHealthChecks(&hc)
		if len(hc) != 1 {
			t.Fatalf("bad: %#v", hc)
		}
	}

	// But with version 8 the node will block it.
	{
		hc := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterHealthChecks(&hc)
		if len(hc) != 0 {
			t.Fatalf("bad: %#v", hc)
		}
	}

	// Chain on access to the node.
	policy, err = acl.NewPolicyFromSource("", 0, `
node "node1" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now it should go through.
	{
		hc := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterHealthChecks(&hc)
		if len(hc) != 1 {
			t.Fatalf("bad: %#v", hc)
		}
	}
}

func TestACL_filterIntentions(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	fill := func() structs.Intentions {
		return structs.Intentions{
			&structs.Intention{
				ID:              "f004177f-2c28-83b7-4229-eacc25fe55d1",
				DestinationName: "bar",
			},
			&structs.Intention{
				ID:              "f004177f-2c28-83b7-4229-eacc25fe55d2",
				DestinationName: "foo",
			},
		}
	}

	// Try permissive filtering.
	{
		ixns := fill()
		filt := newACLFilter(acl.AllowAll(), nil, false)
		filt.filterIntentions(&ixns)
		assert.Len(ixns, 2)
	}

	// Try restrictive filtering.
	{
		ixns := fill()
		filt := newACLFilter(acl.DenyAll(), nil, false)
		filt.filterIntentions(&ixns)
		assert.Len(ixns, 0)
	}

	// Policy to see one
	policy, err := acl.NewPolicyFromSource("", 0, `
service "foo" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	assert.Nil(err)
	perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	assert.Nil(err)

	// Filter
	{
		ixns := fill()
		filt := newACLFilter(perms, nil, false)
		filt.filterIntentions(&ixns)
		assert.Len(ixns, 1)
	}
}

func TestACL_filterServices(t *testing.T) {
	t.Parallel()
	// Create some services
	services := structs.Services{
		"service1": []string{},
		"service2": []string{},
		"consul":   []string{},
	}

	// Try permissive filtering.
	filt := newACLFilter(acl.AllowAll(), nil, false)
	filt.filterServices(services, nil)
	if len(services) != 3 {
		t.Fatalf("bad: %#v", services)
	}

	// Try restrictive filtering.
	filt = newACLFilter(acl.DenyAll(), nil, false)
	filt.filterServices(services, nil)
	if len(services) != 1 {
		t.Fatalf("bad: %#v", services)
	}
	if _, ok := services["consul"]; !ok {
		t.Fatalf("bad: %#v", services)
	}

	// Try restrictive filtering with version 8 enforcement.
	filt = newACLFilter(acl.DenyAll(), nil, true)
	filt.filterServices(services, nil)
	if len(services) != 0 {
		t.Fatalf("bad: %#v", services)
	}
}

func TestACL_filterServiceNodes(t *testing.T) {
	t.Parallel()
	// Create some service nodes.
	fill := func() structs.ServiceNodes {
		return structs.ServiceNodes{
			&structs.ServiceNode{
				Node:        "node1",
				ServiceName: "foo",
			},
		}
	}

	// Try permissive filtering.
	{
		nodes := fill()
		filt := newACLFilter(acl.AllowAll(), nil, false)
		filt.filterServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
	}

	// Try restrictive filtering.
	{
		nodes := fill()
		filt := newACLFilter(acl.DenyAll(), nil, false)
		filt.filterServiceNodes(&nodes)
		if len(nodes) != 0 {
			t.Fatalf("bad: %#v", nodes)
		}
	}

	// Allowed to see the service but not the node.
	policy, err := acl.NewPolicyFromSource("", 0, `
service "foo" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// This will work because version 8 ACLs aren't being enforced.
	{
		nodes := fill()
		filt := newACLFilter(perms, nil, false)
		filt.filterServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
	}

	// But with version 8 the node will block it.
	{
		nodes := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterServiceNodes(&nodes)
		if len(nodes) != 0 {
			t.Fatalf("bad: %#v", nodes)
		}
	}

	// Chain on access to the node.
	policy, err = acl.NewPolicyFromSource("", 0, `
node "node1" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now it should go through.
	{
		nodes := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
	}
}

func TestACL_filterNodeServices(t *testing.T) {
	t.Parallel()
	// Create some node services.
	fill := func() *structs.NodeServices {
		return &structs.NodeServices{
			Node: &structs.Node{
				Node: "node1",
			},
			Services: map[string]*structs.NodeService{
				"foo": &structs.NodeService{
					ID:      "foo",
					Service: "foo",
				},
			},
		}
	}

	// Try nil, which is a possible input.
	{
		var services *structs.NodeServices
		filt := newACLFilter(acl.AllowAll(), nil, false)
		filt.filterNodeServices(&services)
		if services != nil {
			t.Fatalf("bad: %#v", services)
		}
	}

	// Try permissive filtering.
	{
		services := fill()
		filt := newACLFilter(acl.AllowAll(), nil, false)
		filt.filterNodeServices(&services)
		if len(services.Services) != 1 {
			t.Fatalf("bad: %#v", services.Services)
		}
	}

	// Try restrictive filtering.
	{
		services := fill()
		filt := newACLFilter(acl.DenyAll(), nil, false)
		filt.filterNodeServices(&services)
		if len((*services).Services) != 0 {
			t.Fatalf("bad: %#v", (*services).Services)
		}
	}

	// Allowed to see the service but not the node.
	policy, err := acl.NewPolicyFromSource("", 0, `
service "foo" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// This will work because version 8 ACLs aren't being enforced.
	{
		services := fill()
		filt := newACLFilter(perms, nil, false)
		filt.filterNodeServices(&services)
		if len((*services).Services) != 1 {
			t.Fatalf("bad: %#v", (*services).Services)
		}
	}

	// But with version 8 the node will block it.
	{
		services := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterNodeServices(&services)
		if services != nil {
			t.Fatalf("bad: %#v", services)
		}
	}

	// Chain on access to the node.
	policy, err = acl.NewPolicyFromSource("", 0, `
node "node1" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now it should go through.
	{
		services := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterNodeServices(&services)
		if len((*services).Services) != 1 {
			t.Fatalf("bad: %#v", (*services).Services)
		}
	}
}

func TestACL_filterCheckServiceNodes(t *testing.T) {
	t.Parallel()
	// Create some nodes.
	fill := func() structs.CheckServiceNodes {
		return structs.CheckServiceNodes{
			structs.CheckServiceNode{
				Node: &structs.Node{
					Node: "node1",
				},
				Service: &structs.NodeService{
					ID:      "foo",
					Service: "foo",
				},
				Checks: structs.HealthChecks{
					&structs.HealthCheck{
						Node:        "node1",
						CheckID:     "check1",
						ServiceName: "foo",
					},
				},
			},
		}
	}

	// Try permissive filtering.
	{
		nodes := fill()
		filt := newACLFilter(acl.AllowAll(), nil, false)
		filt.filterCheckServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
		if len(nodes[0].Checks) != 1 {
			t.Fatalf("bad: %#v", nodes[0].Checks)
		}
	}

	// Try restrictive filtering.
	{
		nodes := fill()
		filt := newACLFilter(acl.DenyAll(), nil, false)
		filt.filterCheckServiceNodes(&nodes)
		if len(nodes) != 0 {
			t.Fatalf("bad: %#v", nodes)
		}
	}

	// Allowed to see the service but not the node.
	policy, err := acl.NewPolicyFromSource("", 0, `
service "foo" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// This will work because version 8 ACLs aren't being enforced.
	{
		nodes := fill()
		filt := newACLFilter(perms, nil, false)
		filt.filterCheckServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
		if len(nodes[0].Checks) != 1 {
			t.Fatalf("bad: %#v", nodes[0].Checks)
		}
	}

	// But with version 8 the node will block it.
	{
		nodes := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterCheckServiceNodes(&nodes)
		if len(nodes) != 0 {
			t.Fatalf("bad: %#v", nodes)
		}
	}

	// Chain on access to the node.
	policy, err = acl.NewPolicyFromSource("", 0, `
node "node1" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now it should go through.
	{
		nodes := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterCheckServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
		if len(nodes[0].Checks) != 1 {
			t.Fatalf("bad: %#v", nodes[0].Checks)
		}
	}
}

func TestACL_filterCoordinates(t *testing.T) {
	t.Parallel()
	// Create some coordinates.
	coords := structs.Coordinates{
		&structs.Coordinate{
			Node:  "node1",
			Coord: generateRandomCoordinate(),
		},
		&structs.Coordinate{
			Node:  "node2",
			Coord: generateRandomCoordinate(),
		},
	}

	// Try permissive filtering.
	filt := newACLFilter(acl.AllowAll(), nil, false)
	filt.filterCoordinates(&coords)
	if len(coords) != 2 {
		t.Fatalf("bad: %#v", coords)
	}

	// Try restrictive filtering without version 8 ACL enforcement.
	filt = newACLFilter(acl.DenyAll(), nil, false)
	filt.filterCoordinates(&coords)
	if len(coords) != 2 {
		t.Fatalf("bad: %#v", coords)
	}

	// Try restrictive filtering with version 8 ACL enforcement.
	filt = newACLFilter(acl.DenyAll(), nil, true)
	filt.filterCoordinates(&coords)
	if len(coords) != 0 {
		t.Fatalf("bad: %#v", coords)
	}
}

func TestACL_filterSessions(t *testing.T) {
	t.Parallel()
	// Create a session list.
	sessions := structs.Sessions{
		&structs.Session{
			Node: "foo",
		},
		&structs.Session{
			Node: "bar",
		},
	}

	// Try permissive filtering.
	filt := newACLFilter(acl.AllowAll(), nil, true)
	filt.filterSessions(&sessions)
	if len(sessions) != 2 {
		t.Fatalf("bad: %#v", sessions)
	}

	// Try restrictive filtering but with version 8 enforcement turned off.
	filt = newACLFilter(acl.DenyAll(), nil, false)
	filt.filterSessions(&sessions)
	if len(sessions) != 2 {
		t.Fatalf("bad: %#v", sessions)
	}

	// Try restrictive filtering with version 8 enforcement turned on.
	filt = newACLFilter(acl.DenyAll(), nil, true)
	filt.filterSessions(&sessions)
	if len(sessions) != 0 {
		t.Fatalf("bad: %#v", sessions)
	}
}

func TestACL_filterNodeDump(t *testing.T) {
	t.Parallel()
	// Create a node dump.
	fill := func() structs.NodeDump {
		return structs.NodeDump{
			&structs.NodeInfo{
				Node: "node1",
				Services: []*structs.NodeService{
					&structs.NodeService{
						ID:      "foo",
						Service: "foo",
					},
				},
				Checks: []*structs.HealthCheck{
					&structs.HealthCheck{
						Node:        "node1",
						CheckID:     "check1",
						ServiceName: "foo",
					},
				},
			},
		}
	}

	// Try permissive filtering.
	{
		dump := fill()
		filt := newACLFilter(acl.AllowAll(), nil, false)
		filt.filterNodeDump(&dump)
		if len(dump) != 1 {
			t.Fatalf("bad: %#v", dump)
		}
		if len(dump[0].Services) != 1 {
			t.Fatalf("bad: %#v", dump[0].Services)
		}
		if len(dump[0].Checks) != 1 {
			t.Fatalf("bad: %#v", dump[0].Checks)
		}
	}

	// Try restrictive filtering.
	{
		dump := fill()
		filt := newACLFilter(acl.DenyAll(), nil, false)
		filt.filterNodeDump(&dump)
		if len(dump) != 1 {
			t.Fatalf("bad: %#v", dump)
		}
		if len(dump[0].Services) != 0 {
			t.Fatalf("bad: %#v", dump[0].Services)
		}
		if len(dump[0].Checks) != 0 {
			t.Fatalf("bad: %#v", dump[0].Checks)
		}
	}

	// Allowed to see the service but not the node.
	policy, err := acl.NewPolicyFromSource("", 0, `
service "foo" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// This will work because version 8 ACLs aren't being enforced.
	{
		dump := fill()
		filt := newACLFilter(perms, nil, false)
		filt.filterNodeDump(&dump)
		if len(dump) != 1 {
			t.Fatalf("bad: %#v", dump)
		}
		if len(dump[0].Services) != 1 {
			t.Fatalf("bad: %#v", dump[0].Services)
		}
		if len(dump[0].Checks) != 1 {
			t.Fatalf("bad: %#v", dump[0].Checks)
		}
	}

	// But with version 8 the node will block it.
	{
		dump := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterNodeDump(&dump)
		if len(dump) != 0 {
			t.Fatalf("bad: %#v", dump)
		}
	}

	// Chain on access to the node.
	policy, err = acl.NewPolicyFromSource("", 0, `
node "node1" {
  policy = "read"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now it should go through.
	{
		dump := fill()
		filt := newACLFilter(perms, nil, true)
		filt.filterNodeDump(&dump)
		if len(dump) != 1 {
			t.Fatalf("bad: %#v", dump)
		}
		if len(dump[0].Services) != 1 {
			t.Fatalf("bad: %#v", dump[0].Services)
		}
		if len(dump[0].Checks) != 1 {
			t.Fatalf("bad: %#v", dump[0].Checks)
		}
	}
}

func TestACL_filterNodes(t *testing.T) {
	t.Parallel()
	// Create a nodes list.
	nodes := structs.Nodes{
		&structs.Node{
			Node: "foo",
		},
		&structs.Node{
			Node: "bar",
		},
	}

	// Try permissive filtering.
	filt := newACLFilter(acl.AllowAll(), nil, true)
	filt.filterNodes(&nodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %#v", nodes)
	}

	// Try restrictive filtering but with version 8 enforcement turned off.
	filt = newACLFilter(acl.DenyAll(), nil, false)
	filt.filterNodes(&nodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %#v", nodes)
	}

	// Try restrictive filtering with version 8 enforcement turned on.
	filt = newACLFilter(acl.DenyAll(), nil, true)
	filt.filterNodes(&nodes)
	if len(nodes) != 0 {
		t.Fatalf("bad: %#v", nodes)
	}
}

func TestACL_filterDatacenterCheckServiceNodes(t *testing.T) {
	t.Parallel()
	// Create some data.
	fixture := map[string]structs.CheckServiceNodes{
		"dc1": []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc1", "gateway1a", "1.2.3.4", 5555, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc1", "gateway2a", "4.3.2.1", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
		"dc2": []structs.CheckServiceNode{
			newTestMeshGatewayNode(
				"dc2", "gateway1b", "5.6.7.8", 9999, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
			newTestMeshGatewayNode(
				"dc2", "gateway2b", "8.7.6.5", 1111, map[string]string{structs.MetaWANFederationKey: "1"}, api.HealthPassing,
			),
		},
	}

	fill := func(t *testing.T) map[string]structs.CheckServiceNodes {
		t.Helper()
		dup, err := copystructure.Copy(fixture)
		require.NoError(t, err)
		return dup.(map[string]structs.CheckServiceNodes)
	}

	// TODO(rb): switch all newACLFilter calls to use a test logger that uses (*testing.T).Logf

	// Try permissive filtering.
	{
		dcNodes := fill(t)
		filt := newACLFilter(acl.AllowAll(), nil, true)
		filt.filterDatacenterCheckServiceNodes(&dcNodes)
		require.Len(t, dcNodes, 2)
		require.Equal(t, fill(t), dcNodes)
	}

	// Try restrictive filtering.
	{
		dcNodes := fill(t)
		filt := newACLFilter(acl.DenyAll(), nil, true)
		filt.filterDatacenterCheckServiceNodes(&dcNodes)
		require.Len(t, dcNodes, 0)
	}

	var (
		policy *acl.Policy
		err    error
		perms  acl.Authorizer
	)
	// Allowed to see the service but not the node.
	policy, err = acl.NewPolicyFromSource("", 0, `
	service_prefix "" { policy = "read" }
	`, acl.SyntaxCurrent, nil, nil)
	require.NoError(t, err)
	perms, err = acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	{
		dcNodes := fill(t)
		filt := newACLFilter(perms, nil, true)
		filt.filterDatacenterCheckServiceNodes(&dcNodes)
		require.Len(t, dcNodes, 0)
	}

	// Allowed to see the node but not the service.
	policy, err = acl.NewPolicyFromSource("", 0, `
	node_prefix "" { policy = "read" }
	`, acl.SyntaxCurrent, nil, nil)
	require.NoError(t, err)
	perms, err = acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	{
		dcNodes := fill(t)
		filt := newACLFilter(perms, nil, true)
		filt.filterDatacenterCheckServiceNodes(&dcNodes)
		require.Len(t, dcNodes, 0)
	}

	// Allowed to see the service AND the node
	policy, err = acl.NewPolicyFromSource("", 0, `
	service_prefix "" { policy = "read" }
	node_prefix "" { policy = "read" }
	`, acl.SyntaxCurrent, nil, nil)
	require.NoError(t, err)
	perms, err = acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	// Now it should go through.
	{
		dcNodes := fill(t)
		filt := newACLFilter(acl.AllowAll(), nil, true)
		filt.filterDatacenterCheckServiceNodes(&dcNodes)
		require.Len(t, dcNodes, 2)
		require.Equal(t, fill(t), dcNodes)
	}
}

func TestACL_redactPreparedQueryTokens(t *testing.T) {
	t.Parallel()
	query := &structs.PreparedQuery{
		ID:    "f004177f-2c28-83b7-4229-eacc25fe55d1",
		Token: "root",
	}

	expected := &structs.PreparedQuery{
		ID:    "f004177f-2c28-83b7-4229-eacc25fe55d1",
		Token: "root",
	}

	// Try permissive filtering with a management token. This will allow the
	// embedded token to be seen.
	filt := newACLFilter(acl.ManageAll(), nil, false)
	filt.redactPreparedQueryTokens(&query)
	if !reflect.DeepEqual(query, expected) {
		t.Fatalf("bad: %#v", &query)
	}

	// Hang on to the entry with a token, which needs to survive the next
	// operation.
	original := query

	// Now try permissive filtering with a client token, which should cause
	// the embedded token to get redacted.
	filt = newACLFilter(acl.AllowAll(), nil, false)
	filt.redactPreparedQueryTokens(&query)
	expected.Token = redactedToken
	if !reflect.DeepEqual(query, expected) {
		t.Fatalf("bad: %#v", *query)
	}

	// Make sure that the original object didn't lose its token.
	if original.Token != "root" {
		t.Fatalf("bad token: %s", original.Token)
	}
}

func TestACL_redactTokenSecret(t *testing.T) {
	t.Parallel()
	delegate := &ACLResolverTestDelegate{
		enabled:       true,
		datacenter:    "dc1",
		legacy:        false,
		localTokens:   true,
		localPolicies: true,
		// No need to provide any of the RPC callbacks
	}
	r := newTestACLResolver(t, delegate, nil)

	token := &structs.ACLToken{
		AccessorID: "6a5e25b3-28f2-4085-9012-c3fb754314d1",
		SecretID:   "6a5e25b3-28f2-4085-9012-c3fb754314d1",
	}

	err := r.filterACL("acl-wr", &token)
	require.NoError(t, err)
	require.Equal(t, "6a5e25b3-28f2-4085-9012-c3fb754314d1", token.SecretID)

	err = r.filterACL("acl-ro", &token)
	require.NoError(t, err)
	require.Equal(t, redactedToken, token.SecretID)
}

func TestACL_redactTokenSecrets(t *testing.T) {
	t.Parallel()
	delegate := &ACLResolverTestDelegate{
		enabled:       true,
		datacenter:    "dc1",
		legacy:        false,
		localTokens:   true,
		localPolicies: true,
		// No need to provide any of the RPC callbacks
	}
	r := newTestACLResolver(t, delegate, nil)

	tokens := structs.ACLTokens{
		&structs.ACLToken{
			AccessorID: "6a5e25b3-28f2-4085-9012-c3fb754314d1",
			SecretID:   "6a5e25b3-28f2-4085-9012-c3fb754314d1",
		},
	}

	err := r.filterACL("acl-wr", &tokens)
	require.NoError(t, err)
	require.Equal(t, "6a5e25b3-28f2-4085-9012-c3fb754314d1", tokens[0].SecretID)

	err = r.filterACL("acl-ro", &tokens)
	require.NoError(t, err)
	require.Equal(t, redactedToken, tokens[0].SecretID)
}

func TestACL_filterPreparedQueries(t *testing.T) {
	t.Parallel()
	queries := structs.PreparedQueries{
		&structs.PreparedQuery{
			ID: "f004177f-2c28-83b7-4229-eacc25fe55d1",
		},
		&structs.PreparedQuery{
			ID:   "f004177f-2c28-83b7-4229-eacc25fe55d2",
			Name: "query-with-no-token",
		},
		&structs.PreparedQuery{
			ID:    "f004177f-2c28-83b7-4229-eacc25fe55d3",
			Name:  "query-with-a-token",
			Token: "root",
		},
	}

	expected := structs.PreparedQueries{
		&structs.PreparedQuery{
			ID: "f004177f-2c28-83b7-4229-eacc25fe55d1",
		},
		&structs.PreparedQuery{
			ID:   "f004177f-2c28-83b7-4229-eacc25fe55d2",
			Name: "query-with-no-token",
		},
		&structs.PreparedQuery{
			ID:    "f004177f-2c28-83b7-4229-eacc25fe55d3",
			Name:  "query-with-a-token",
			Token: "root",
		},
	}

	// Try permissive filtering with a management token. This will allow the
	// embedded token to be seen.
	filt := newACLFilter(acl.ManageAll(), nil, false)
	filt.filterPreparedQueries(&queries)
	if !reflect.DeepEqual(queries, expected) {
		t.Fatalf("bad: %#v", queries)
	}

	// Hang on to the entry with a token, which needs to survive the next
	// operation.
	original := queries[2]

	// Now try permissive filtering with a client token, which should cause
	// the embedded token to get redacted, and the query with no name to get
	// filtered out.
	filt = newACLFilter(acl.AllowAll(), nil, false)
	filt.filterPreparedQueries(&queries)
	expected[2].Token = redactedToken
	expected = append(structs.PreparedQueries{}, expected[1], expected[2])
	if !reflect.DeepEqual(queries, expected) {
		t.Fatalf("bad: %#v", queries)
	}

	// Make sure that the original object didn't lose its token.
	if original.Token != "root" {
		t.Fatalf("bad token: %s", original.Token)
	}

	// Now try restrictive filtering.
	filt = newACLFilter(acl.DenyAll(), nil, false)
	filt.filterPreparedQueries(&queries)
	if len(queries) != 0 {
		t.Fatalf("bad: %#v", queries)
	}
}

func TestACL_unhandledFilterType(t *testing.T) {
	t.Parallel()
	defer func(t *testing.T) {
		if recover() == nil {
			t.Fatalf("should panic")
		}
	}(t)

	// Create the server
	dir, token, srv, client := testACLFilterServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer client.Close()

	// Pass an unhandled type into the ACL filter.
	srv.filterACL(token, &structs.HealthCheck{})
}

func TestACL_vetRegisterWithACL(t *testing.T) {
	t.Parallel()
	args := &structs.RegisterRequest{
		Node:    "nope",
		Address: "127.0.0.1",
	}

	// With a nil ACL, the update should be allowed.
	if err := vetRegisterWithACL(nil, args, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a basic node policy.
	policy, err := acl.NewPolicyFromSource("", 0, `
node "node" {
  policy = "write"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// With that policy, the update should now be blocked for node reasons.
	err = vetRegisterWithACL(perms, args, nil)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Now use a permitted node name.
	args.Node = "node"
	if err := vetRegisterWithACL(perms, args, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Build some node info that matches what we have now.
	ns := &structs.NodeServices{
		Node: &structs.Node{
			Node:    "node",
			Address: "127.0.0.1",
		},
		Services: make(map[string]*structs.NodeService),
	}

	// Try to register a service, which should be blocked.
	args.Service = &structs.NodeService{
		Service: "service",
		ID:      "my-id",
	}
	err = vetRegisterWithACL(perms, args, ns)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Chain on a basic service policy.
	policy, err = acl.NewPolicyFromSource("", 0, `
service "service" {
  policy = "write"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// With the service ACL, the update should go through.
	if err := vetRegisterWithACL(perms, args, ns); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add an existing service that they are clobbering and aren't allowed
	// to write to.
	ns.Services["my-id"] = &structs.NodeService{
		Service: "other",
		ID:      "my-id",
	}
	err = vetRegisterWithACL(perms, args, ns)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Chain on a policy that allows them to write to the other service.
	policy, err = acl.NewPolicyFromSource("", 0, `
service "other" {
  policy = "write"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Now it should go through.
	if err := vetRegisterWithACL(perms, args, ns); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try creating the node and the service at once by having no existing
	// node record. This should be ok since we have node and service
	// permissions.
	if err := vetRegisterWithACL(perms, args, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add a node-level check to the member, which should be rejected.
	args.Check = &structs.HealthCheck{
		Node: "node",
	}
	err = vetRegisterWithACL(perms, args, ns)
	if err == nil || !strings.Contains(err.Error(), "check member must be nil") {
		t.Fatalf("bad: %v", err)
	}

	// Move the check into the slice, but give a bad node name.
	args.Check.Node = "nope"
	args.Checks = append(args.Checks, args.Check)
	args.Check = nil
	err = vetRegisterWithACL(perms, args, ns)
	if err == nil || !strings.Contains(err.Error(), "doesn't match register request node") {
		t.Fatalf("bad: %v", err)
	}

	// Fix the node name, which should now go through.
	args.Checks[0].Node = "node"
	if err := vetRegisterWithACL(perms, args, ns); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Add a service-level check.
	args.Checks = append(args.Checks, &structs.HealthCheck{
		Node:      "node",
		ServiceID: "my-id",
	})
	if err := vetRegisterWithACL(perms, args, ns); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try creating everything at once. This should be ok since we have all
	// the permissions we need. It also makes sure that we can register a
	// new node, service, and associated checks.
	if err := vetRegisterWithACL(perms, args, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nil out the service registration, which'll skip the special case
	// and force us to look at the ns data (it will look like we are
	// writing to the "other" service which also has "my-id").
	args.Service = nil
	if err := vetRegisterWithACL(perms, args, ns); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Chain on a policy that forbids them to write to the other service.
	policy, err = acl.NewPolicyFromSource("", 0, `
service "other" {
  policy = "deny"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// This should get rejected.
	err = vetRegisterWithACL(perms, args, ns)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Change the existing service data to point to a service name they
	// car write to. This should go through.
	ns.Services["my-id"] = &structs.NodeService{
		Service: "service",
		ID:      "my-id",
	}
	if err := vetRegisterWithACL(perms, args, ns); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Chain on a policy that forbids them to write to the node.
	policy, err = acl.NewPolicyFromSource("", 0, `
node "node" {
  policy = "deny"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizerWithDefaults(perms, []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// This should get rejected because there's a node-level check in here.
	err = vetRegisterWithACL(perms, args, ns)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Change the node-level check into a service check, and then it should
	// go through.
	args.Checks[0].ServiceID = "my-id"
	if err := vetRegisterWithACL(perms, args, ns); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Finally, attempt to update the node part of the data and make sure
	// that gets rejected since they no longer have permissions.
	args.Address = "127.0.0.2"
	err = vetRegisterWithACL(perms, args, ns)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}
}

func TestACL_vetDeregisterWithACL(t *testing.T) {
	t.Parallel()
	args := &structs.DeregisterRequest{
		Node: "nope",
	}

	// With a nil ACL, the update should be allowed.
	if err := vetDeregisterWithACL(nil, args, nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a basic node policy.
	policy, err := acl.NewPolicyFromSource("", 0, `
node "node" {
  policy = "write"
}
`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	nodePerms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	policy, err = acl.NewPolicyFromSource("", 0, `
	service "my-service" {
	  policy = "write"
	}
	`, acl.SyntaxLegacy, nil, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	servicePerms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	for _, args := range []struct {
		DeregisterRequest structs.DeregisterRequest
		Service           *structs.NodeService
		Check             *structs.HealthCheck
		Perms             acl.Authorizer
		Expected          bool
		Name              string
	}{
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "nope",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no right on node",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "nope",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "right on service but node dergister request",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no rights on node nor service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "no rights on node but rights on service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no right on node nor service for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "nope",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "no rights on node but rights on service for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "nope",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: false,
			Name:     "no right on node for node check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "nope",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "rights on service but no right on node for node check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "node",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "rights on node for node",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node: "node",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "rights on service but not on node for node",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "rights on node for service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "rights on service for service",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "right on node for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:      "node",
				ServiceID: "my-service-id",
				CheckID:   "my-check",
			},
			Service: &structs.NodeService{
				Service: "my-service",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: true,
			Name:     "rights on service for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "node",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    nodePerms,
			Expected: true,
			Name:     "rights on node for check",
		},
		{
			DeregisterRequest: structs.DeregisterRequest{
				Node:    "node",
				CheckID: "my-check",
			},
			Check: &structs.HealthCheck{
				CheckID: "my-check",
			},
			Perms:    servicePerms,
			Expected: false,
			Name:     "rights on service for node check",
		},
	} {
		t.Run(args.Name, func(t *testing.T) {
			err = vetDeregisterWithACL(args.Perms, &args.DeregisterRequest, args.Service, args.Check)
			if !args.Expected {
				if err == nil {
					t.Errorf("expected error with %+v", args.DeregisterRequest)
				}
				if !acl.IsErrPermissionDenied(err) {
					t.Errorf("expected permission denied error with %+v, instead got %+v", args.DeregisterRequest, err)
				}
			} else if err != nil {
				t.Errorf("expected no error with %+v", args.DeregisterRequest)
			}
		})
	}
}

func TestDedupeServiceIdentities(t *testing.T) {
	srvid := func(name string, datacenters ...string) *structs.ACLServiceIdentity {
		return &structs.ACLServiceIdentity{
			ServiceName: name,
			Datacenters: datacenters,
		}
	}

	tests := []struct {
		name   string
		in     []*structs.ACLServiceIdentity
		expect []*structs.ACLServiceIdentity
	}{
		{
			name:   "empty",
			in:     nil,
			expect: nil,
		},
		{
			name: "one",
			in: []*structs.ACLServiceIdentity{
				srvid("foo"),
			},
			expect: []*structs.ACLServiceIdentity{
				srvid("foo"),
			},
		},
		{
			name: "just names",
			in: []*structs.ACLServiceIdentity{
				srvid("fooZ"),
				srvid("fooA"),
				srvid("fooY"),
				srvid("fooB"),
			},
			expect: []*structs.ACLServiceIdentity{
				srvid("fooA"),
				srvid("fooB"),
				srvid("fooY"),
				srvid("fooZ"),
			},
		},
		{
			name: "just names with dupes",
			in: []*structs.ACLServiceIdentity{
				srvid("fooZ"),
				srvid("fooA"),
				srvid("fooY"),
				srvid("fooB"),
				srvid("fooA"),
				srvid("fooB"),
				srvid("fooY"),
				srvid("fooZ"),
			},
			expect: []*structs.ACLServiceIdentity{
				srvid("fooA"),
				srvid("fooB"),
				srvid("fooY"),
				srvid("fooZ"),
			},
		},
		{
			name: "names with dupes and datacenters",
			in: []*structs.ACLServiceIdentity{
				srvid("fooZ", "dc2", "dc4"),
				srvid("fooA"),
				srvid("fooY", "dc1"),
				srvid("fooB"),
				srvid("fooA", "dc9", "dc8"),
				srvid("fooB"),
				srvid("fooY", "dc1"),
				srvid("fooZ", "dc3", "dc4"),
			},
			expect: []*structs.ACLServiceIdentity{
				srvid("fooA"),
				srvid("fooB"),
				srvid("fooY", "dc1"),
				srvid("fooZ", "dc2", "dc3", "dc4"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := dedupeServiceIdentities(test.in)
			require.ElementsMatch(t, test.expect, got)
		})
	}
}

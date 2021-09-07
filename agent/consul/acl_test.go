package consul

import (
	"fmt"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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
	_, authz, err := r.ResolveTokenToIdentityAndAuthorizer(token)
	ch <- &asyncResolutionResult{authz: authz, err: err}
}

// Deprecated: use resolveToken or ACLResolver.ResolveTokenToIdentityAndAuthorizer instead
func (r *ACLResolver) ResolveToken(token string) (acl.Authorizer, error) {
	_, authz, err := r.ResolveTokenToIdentityAndAuthorizer(token)
	return authz, err
}

func resolveToken(t *testing.T, r *ACLResolver, token string) acl.Authorizer {
	t.Helper()
	_, authz, err := r.ResolveTokenToIdentityAndAuthorizer(token)
	require.NoError(t, err)
	return authz
}

func testIdentityForToken(token string) (bool, structs.ACLIdentity, error) {
	switch token {
	case "missing-policy":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "not-found",
				},
				{
					ID: "acl-ro",
				},
			},
		}, nil
	case "missing-role":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: "not-found",
				},
				{
					ID: "acl-ro",
				},
			},
		}, nil
	case "missing-policy-on-role":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Roles: []structs.ACLTokenRoleLink{
				{
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
				{
					ID: "node-wr",
				},
				{
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
				{
					ID: "found",
				},
			},
		}, nil
	case "found-policy-and-role":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "node-wr",
				},
				{
					ID: "dc2-key-wr",
				},
			},
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: "service-ro",
				},
			},
		}, nil
	case "found-synthetic-policy-1":
		return true, &structs.ACLToken{
			AccessorID: "f6c5a5fb-4da4-422b-9abf-2c942813fc71",
			SecretID:   "55cb7d69-2bea-42c3-a68f-2a1443d2abbc",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					ServiceName: "service1",
				},
			},
		}, nil
	case "found-synthetic-policy-2":
		return true, &structs.ACLToken{
			AccessorID: "7c87dfad-be37-446e-8305-299585677cb5",
			SecretID:   "dfca9676-ac80-453a-837b-4c0cf923473c",
			ServiceIdentities: []*structs.ACLServiceIdentity{
				{
					ServiceName: "service2",
				},
			},
		}, nil
	case "found-synthetic-policy-3":
		return true, &structs.ACLToken{
			AccessorID: "bebccc92-3987-489d-84c2-ffd00d93ef93",
			SecretID:   "de70f2e2-69d9-4e88-9815-f91c03c6bcb1",
			NodeIdentities: []*structs.ACLNodeIdentity{
				{
					NodeName:   "test-node1",
					Datacenter: "dc1",
				},
				// as the resolver is in dc1 this identity should be ignored
				{
					NodeName:   "test-node-dc2",
					Datacenter: "dc2",
				},
			},
		}, nil
	case "found-synthetic-policy-4":
		return true, &structs.ACLToken{
			AccessorID: "359b9927-25fd-46b9-bd14-3470f848ec65",
			SecretID:   "83c4d500-847d-49f7-8c08-0483f6b4156e",
			NodeIdentities: []*structs.ACLNodeIdentity{
				{
					NodeName:   "test-node2",
					Datacenter: "dc1",
				},
				// as the resolver is in dc1 this identity should be ignored
				{
					NodeName:   "test-node-dc2",
					Datacenter: "dc2",
				},
			},
		}, nil
	case "found-role-node-identity":
		return true, &structs.ACLToken{
			AccessorID: "f3f47a09-de29-4c57-8f54-b65a9be79641",
			SecretID:   "e96aca00-5951-4b97-b0e5-5816f42dfb93",
			Roles: []structs.ACLTokenRoleLink{
				{
					ID: "node-identity",
				},
			},
		}, nil
	case "acl-ro":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "acl-ro",
				},
			},
		}, nil
	case "acl-wr":
		return true, &structs.ACLToken{
			AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
			SecretID:   "b1b6be70-ed2e-4c80-8495-bdb3db110b1e",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "acl-wr",
				},
			},
		}, nil
	case "racey-unmodified":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "node-wr",
				},
				{
					ID: "acl-wr",
				},
			},
		}, nil
	case "racey-modified":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "node-wr",
				},
			},
		}, nil
	case "concurrent-resolve":
		return true, &structs.ACLToken{
			AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
			SecretID:   "a1a54629-5050-4d17-8a4e-560d2423f835",
			Policies: []structs.ACLTokenPolicyLink{
				{
					ID: "node-wr",
				},
				{
					ID: "acl-wr",
				},
			},
		}, nil
	case anonymousToken:
		return true, &structs.ACLToken{
			AccessorID: "00000000-0000-0000-0000-000000000002",
			SecretID:   anonymousToken,
			Policies: []structs.ACLTokenPolicyLink{
				{
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
		p := &structs.ACLPolicy{
			ID:          "acl-ro",
			Name:        "acl-ro",
			Description: "acl-ro",
			Rules:       `acl = "read"`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}
		p.SetHash(false)
		return true, p, nil
	case "acl-wr":
		p := &structs.ACLPolicy{
			ID:          "acl-wr",
			Name:        "acl-wr",
			Description: "acl-wr",
			Rules:       `acl = "write"`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}
		p.SetHash(false)
		return true, p, nil
	case "service-ro":
		p := &structs.ACLPolicy{
			ID:          "service-ro",
			Name:        "service-ro",
			Description: "service-ro",
			Rules:       `service_prefix "" { policy = "read" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}
		p.SetHash(false)
		return true, p, nil
	case "service-wr":
		p := &structs.ACLPolicy{
			ID:          "service-wr",
			Name:        "service-wr",
			Description: "service-wr",
			Rules:       `service_prefix "" { policy = "write" }`,
			Syntax:      acl.SyntaxCurrent,
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}
		p.SetHash(false)
		return true, p, nil
	case "node-wr":
		p := &structs.ACLPolicy{
			ID:          "node-wr",
			Name:        "node-wr",
			Description: "node-wr",
			Rules:       `node_prefix "" { policy = "write"}`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc1"},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}
		p.SetHash(false)
		return true, p, nil
	case "dc2-key-wr":
		p := &structs.ACLPolicy{
			ID:          "dc2-key-wr",
			Name:        "dc2-key-wr",
			Description: "dc2-key-wr",
			Rules:       `key_prefix "" { policy = "write"}`,
			Syntax:      acl.SyntaxCurrent,
			Datacenters: []string{"dc2"},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}
		p.SetHash(false)
		return true, p, nil
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
				{
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
				{
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
				{
					ID: "not-found",
				},
				{
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
				{
					ID: "node-wr",
				},
				{
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
				{
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
				{
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
				{
					ID: "node-wr",
				},
				{
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
				{
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
				{
					ID: "node-wr",
				},
				{
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
				{
					ID: "node-wr",
				},
				{
					ID: "acl-wr",
				},
			},
		}, nil
	case "node-identity":
		return true, &structs.ACLRole{
			ID:          "node-identity",
			Name:        "node-identity",
			Description: "node-identity",
			NodeIdentities: []*structs.ACLNodeIdentity{
				{
					NodeName:   "test-node",
					Datacenter: "dc1",
				},
				{
					NodeName:   "test-node-dc2",
					Datacenter: "dc2",
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
	// enabled is no longer part of the delegate. It is still here as a field on
	// the fake delegate because many tests use this field to enable ACLs. This field
	// is now used to set ACLResolverConfig.Config.ACLsEnabled.
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

	localTokenResolutions   int32
	remoteTokenResolutions  int32
	localPolicyResolutions  int32
	remotePolicyResolutions int32
	localRoleResolutions    int32
	remoteRoleResolutions   int32
	remoteLegacyResolutions int32

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

	atomic.AddInt32(&d.localTokenResolutions, 1)
	return testIdentityForToken(token)
}

func (d *ACLResolverTestDelegate) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	if !d.localPolicies {
		return false, nil, nil
	}

	atomic.AddInt32(&d.localPolicyResolutions, 1)
	return testPolicyForID(policyID)
}

func (d *ACLResolverTestDelegate) ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error) {
	if !d.localRoles {
		return false, nil, nil
	}

	atomic.AddInt32(&d.localRoleResolutions, 1)
	return testRoleForID(roleID)
}

func (d *ACLResolverTestDelegate) RPC(method string, args interface{}, reply interface{}) error {
	switch method {
	case "ACL.GetPolicy":
		atomic.AddInt32(&d.remoteLegacyResolutions, 1)
		if d.getPolicyFn != nil {
			return d.getPolicyFn(args.(*structs.ACLPolicyResolveLegacyRequest), reply.(*structs.ACLPolicyResolveLegacyResponse))
		}
		panic("Bad Test Implementation: should provide a getPolicyFn to the ACLResolverTestDelegate")
	case "ACL.TokenRead":
		atomic.AddInt32(&d.remoteTokenResolutions, 1)
		if d.tokenReadFn != nil {
			return d.tokenReadFn(args.(*structs.ACLTokenGetRequest), reply.(*structs.ACLTokenResponse))
		}
		panic("Bad Test Implementation: should provide a tokenReadFn to the ACLResolverTestDelegate")
	case "ACL.PolicyResolve":
		atomic.AddInt32(&d.remotePolicyResolutions, 1)
		if d.policyResolveFn != nil {
			return d.policyResolveFn(args.(*structs.ACLPolicyBatchGetRequest), reply.(*structs.ACLPolicyBatchResponse))
		}
		panic("Bad Test Implementation: should provide a policyResolveFn to the ACLResolverTestDelegate")
	case "ACL.RoleResolve":
		atomic.AddInt32(&d.remoteRoleResolutions, 1)
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

func newTestACLResolver(t *testing.T, delegate *ACLResolverTestDelegate, cb func(*ACLResolverConfig)) *ACLResolver {
	config := DefaultConfig()
	config.ACLResolverSettings.ACLDefaultPolicy = "deny"
	config.ACLResolverSettings.ACLDownPolicy = "extend-cache"
	config.ACLResolverSettings.ACLsEnabled = delegate.enabled
	rconf := &ACLResolverConfig{
		Config: config.ACLResolverSettings,
		Logger: testutil.Logger(t),
		CacheConfig: &structs.ACLCachesConfig{
			Identities:     4,
			Policies:       4,
			ParsedPolicies: 4,
			Authorizers:    4,
			Roles:          4,
		},
		DisableDuration: aclClientDisabledTTL,
		Delegate:        delegate,
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
	require.Equal(t, acl.ManageAll(), authz)
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
			tokenReadFn: func(_ *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
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
			tokenReadFn: func(_ *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			tokenReadFn: func(_ *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
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

	t.Run("Resolve-Identity", func(t *testing.T) {
		t.Parallel()

		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
		}

		delegate.tokenReadFn = delegate.plainTokenReadFn
		delegate.policyResolveFn = delegate.plainPolicyResolveFn
		delegate.roleResolveFn = delegate.plainRoleResolveFn

		r := newTestACLResolver(t, delegate, nil)

		ident, err := r.ResolveTokenToIdentity("found-policy-and-role")
		require.NoError(t, err)
		require.NotNil(t, ident)
		require.Equal(t, "5f57c1f6-6a89-4186-9445-531b316e01df", ident.ID())
		require.EqualValues(t, 0, delegate.localTokenResolutions)
		require.EqualValues(t, 1, delegate.remoteTokenResolutions)
		require.EqualValues(t, 0, delegate.localPolicyResolutions)
		require.EqualValues(t, 0, delegate.remotePolicyResolutions)
		require.EqualValues(t, 0, delegate.localRoleResolutions)
		require.EqualValues(t, 0, delegate.remoteRoleResolutions)
		require.EqualValues(t, 0, delegate.remoteLegacyResolutions)
	})

	t.Run("Resolve-Identity-Legacy", func(t *testing.T) {
		t.Parallel()

		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        true,
			localTokens:   false,
			localPolicies: false,
			getPolicyFn: func(_ *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				reply.Parent = "deny"
				reply.TTL = 30
				reply.ETag = "nothing"
				reply.Policy = &acl.Policy{
					ID: "not-needed",
					PolicyRules: acl.PolicyRules{
						Nodes: []*acl.NodeRule{
							{
								Name:   "foo",
								Policy: acl.PolicyWrite,
							},
						},
					},
				}
				return nil
			},
		}

		r := newTestACLResolver(t, delegate, nil)

		ident, err := r.ResolveTokenToIdentity("found-policy-and-role")
		require.NoError(t, err)
		require.NotNil(t, ident)
		require.Equal(t, "legacy-token", ident.ID())
		require.EqualValues(t, 0, delegate.localTokenResolutions)
		require.EqualValues(t, 0, delegate.remoteTokenResolutions)
		require.EqualValues(t, 0, delegate.localPolicyResolutions)
		require.EqualValues(t, 0, delegate.remotePolicyResolutions)
		require.EqualValues(t, 0, delegate.localRoleResolutions)
		require.EqualValues(t, 0, delegate.remoteRoleResolutions)
		require.EqualValues(t, 1, delegate.remoteLegacyResolutions)
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
			config.Config.ACLPolicyTTL = 30 * time.Second
			config.Config.ACLRoleTTL = 30 * time.Second
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
		authz := resolveToken(t, r, "missing-policy")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Missing Role", func(t *testing.T) {
		authz := resolveToken(t, r, "missing-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Missing Policy on Role", func(t *testing.T) {
		authz := resolveToken(t, r, "missing-policy-on-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Policy", func(t *testing.T) {
		authz := resolveToken(t, r, "found")
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Role", func(t *testing.T) {
		authz := resolveToken(t, r, "found-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Policy and Role", func(t *testing.T) {
		authz := resolveToken(t, r, "found-policy-and-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("bar", nil))
	})

	runTwiceAndReset("Role With Node Identity", func(t *testing.T) {
		authz := resolveToken(t, r, "found-role-node-identity")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("test-node", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("test-node-dc2", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("something", nil))
		require.Equal(t, acl.Deny, authz.ServiceWrite("something", nil))
	})

	runTwiceAndReset("Synthetic Policies Independently Cache", func(t *testing.T) {
		// We resolve these tokens in the same cache session
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
		{
			authz, err := r.ResolveToken("found-synthetic-policy-3")
			require.NoError(t, err)
			require.NotNil(t, authz)

			// spot check some random perms
			require.Equal(t, acl.Deny, authz.ACLRead(nil))
			require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
			// ensure we didn't bleed over to the other synthetic policy
			require.Equal(t, acl.Deny, authz.NodeWrite("test-node2", nil))
			// check our own synthetic policy
			require.Equal(t, acl.Allow, authz.ServiceRead("literally-anything", nil))
			require.Equal(t, acl.Allow, authz.NodeWrite("test-node1", nil))
			// ensure node identity for other DC is ignored
			require.Equal(t, acl.Deny, authz.NodeWrite("test-node-dc2", nil))
		}
		{
			authz, err := r.ResolveToken("found-synthetic-policy-4")
			require.NoError(t, err)
			require.NotNil(t, authz)

			// spot check some random perms
			require.Equal(t, acl.Deny, authz.ACLRead(nil))
			require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
			// ensure we didn't bleed over to the other synthetic policy
			require.Equal(t, acl.Deny, authz.NodeWrite("test-node1", nil))
			// check our own synthetic policy
			require.Equal(t, acl.Allow, authz.ServiceRead("literally-anything", nil))
			require.Equal(t, acl.Allow, authz.NodeWrite("test-node2", nil))
			// ensure node identity for other DC is ignored
			require.Equal(t, acl.Deny, authz.NodeWrite("test-node-dc2", nil))
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
		require.Equal(t, acl.Deny, authz.MeshRead(nil))
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
			getPolicyFn: func(_ *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 30
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								{
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
			getPolicyFn: func(_ *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								{
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
			getPolicyFn: func(_ *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								{
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
			getPolicyFn: func(_ *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								{
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
			getPolicyFn: func(_ *structs.ACLPolicyResolveLegacyRequest, reply *structs.ACLPolicyResolveLegacyResponse) error {
				if !cached {
					reply.Parent = "deny"
					reply.TTL = 0
					reply.ETag = "nothing"
					reply.Policy = &acl.Policy{
						ID: "not-needed",
						PolicyRules: acl.PolicyRules{
							Nodes: []*acl.NodeRule{
								{
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
			c.PrimaryDatacenter = "dc1"
			c.ACLMasterToken = "root"
		})
		defer os.RemoveAll(dir1)
		defer s1.Shutdown()
		client := rpcClient(t, s1)
		defer client.Close()

		dir2, s2 := testServerWithConfig(t, func(c *Config) {
			c.Datacenter = "dc2"
			c.PrimaryDatacenter = "dc1"
			c.ACLResolverSettings.ACLDefaultPolicy = "deny"
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
			c.PrimaryDatacenter = "dc1"
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
		c.PrimaryDatacenter = "dc1"
		c.ACLMasterToken = "root"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.Datacenter = "dc2"
		c.PrimaryDatacenter = "dc1" // Enable ACLs!
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

	{
		hc := fill()
		filt := newACLFilter(acl.DenyAll(), nil)
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

	{
		hc := fill()
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(acl.AllowAll(), nil)
		filt.filterIntentions(&ixns)
		assert.Len(ixns, 2)
	}

	// Try restrictive filtering.
	{
		ixns := fill()
		filt := newACLFilter(acl.DenyAll(), nil)
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
		filt := newACLFilter(perms, nil)
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
	filt := newACLFilter(acl.AllowAll(), nil)
	filt.filterServices(services, nil)
	if len(services) != 3 {
		t.Fatalf("bad: %#v", services)
	}

	// Try restrictive filtering.
	filt = newACLFilter(acl.DenyAll(), nil)
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
		filt := newACLFilter(acl.AllowAll(), nil)
		filt.filterServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
	}

	// Try restrictive filtering.
	{
		nodes := fill()
		filt := newACLFilter(acl.DenyAll(), nil)
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

	// But with version 8 the node will block it.
	{
		nodes := fill()
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(perms, nil)
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
				"foo": {
					ID:      "foo",
					Service: "foo",
				},
			},
		}
	}

	// Try nil, which is a possible input.
	{
		var services *structs.NodeServices
		filt := newACLFilter(acl.AllowAll(), nil)
		filt.filterNodeServices(&services)
		if services != nil {
			t.Fatalf("bad: %#v", services)
		}
	}

	// Try permissive filtering.
	{
		services := fill()
		filt := newACLFilter(acl.AllowAll(), nil)
		filt.filterNodeServices(&services)
		if len(services.Services) != 1 {
			t.Fatalf("bad: %#v", services.Services)
		}
	}

	// Try restrictive filtering.
	{
		services := fill()
		filt := newACLFilter(acl.DenyAll(), nil)
		filt.filterNodeServices(&services)
		if services != nil {
			t.Fatalf("bad: %#v", *services)
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

	// Node will block it.
	{
		services := fill()
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(acl.AllowAll(), nil)
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
		filt := newACLFilter(acl.DenyAll(), nil)
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

	{
		nodes := fill()
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(perms, nil)
		filt.filterCheckServiceNodes(&nodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %#v", nodes)
		}
		if len(nodes[0].Checks) != 1 {
			t.Fatalf("bad: %#v", nodes[0].Checks)
		}
	}
}

func TestACL_filterServiceTopology(t *testing.T) {
	t.Parallel()
	// Create some nodes.
	fill := func() structs.ServiceTopology {
		return structs.ServiceTopology{
			Upstreams: structs.CheckServiceNodes{
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
			},
			Downstreams: structs.CheckServiceNodes{
				structs.CheckServiceNode{
					Node: &structs.Node{
						Node: "node2",
					},
					Service: &structs.NodeService{
						ID:      "bar",
						Service: "bar",
					},
					Checks: structs.HealthChecks{
						&structs.HealthCheck{
							Node:        "node2",
							CheckID:     "check1",
							ServiceName: "bar",
						},
					},
				},
			},
		}
	}
	original := fill()

	t.Run("allow all without permissions", func(t *testing.T) {
		topo := fill()
		f := newACLFilter(acl.AllowAll(), nil)

		filtered := f.filterServiceTopology(&topo)
		if filtered {
			t.Fatalf("should not have been filtered")
		}
		assert.Equal(t, original, topo)
	})

	t.Run("deny all without permissions", func(t *testing.T) {
		topo := fill()
		f := newACLFilter(acl.DenyAll(), nil)

		filtered := f.filterServiceTopology(&topo)
		if !filtered {
			t.Fatalf("should have been marked as filtered")
		}
		assert.Len(t, topo.Upstreams, 0)
		assert.Len(t, topo.Upstreams, 0)
	})

	t.Run("only upstream permissions", func(t *testing.T) {
		rules := `
node "node1" {
  policy = "read"
}
service "foo" {
  policy = "read"
}`
		policy, err := acl.NewPolicyFromSource("", 0, rules, acl.SyntaxLegacy, nil, nil)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		topo := fill()
		f := newACLFilter(perms, nil)

		filtered := f.filterServiceTopology(&topo)
		if !filtered {
			t.Fatalf("should have been marked as filtered")
		}
		assert.Equal(t, original.Upstreams, topo.Upstreams)
		assert.Len(t, topo.Downstreams, 0)
	})

	t.Run("only downstream permissions", func(t *testing.T) {
		rules := `
node "node2" {
  policy = "read"
}
service "bar" {
  policy = "read"
}`
		policy, err := acl.NewPolicyFromSource("", 0, rules, acl.SyntaxLegacy, nil, nil)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		topo := fill()
		f := newACLFilter(perms, nil)

		filtered := f.filterServiceTopology(&topo)
		if !filtered {
			t.Fatalf("should have been marked as filtered")
		}
		assert.Equal(t, original.Downstreams, topo.Downstreams)
		assert.Len(t, topo.Upstreams, 0)
	})

	t.Run("upstream and downstream permissions", func(t *testing.T) {
		rules := `
node "node1" {
  policy = "read"
}
service "foo" {
  policy = "read"
}
node "node2" {
  policy = "read"
}
service "bar" {
  policy = "read"
}`
		policy, err := acl.NewPolicyFromSource("", 0, rules, acl.SyntaxLegacy, nil, nil)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		perms, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		topo := fill()
		f := newACLFilter(perms, nil)

		filtered := f.filterServiceTopology(&topo)
		if filtered {
			t.Fatalf("should not have been filtered")
		}

		original := fill()
		assert.Equal(t, original, topo)
	})
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
	filt := newACLFilter(acl.AllowAll(), nil)
	filt.filterCoordinates(&coords)
	if len(coords) != 2 {
		t.Fatalf("bad: %#v", coords)
	}

	// Try restrictive filtering
	filt = newACLFilter(acl.DenyAll(), nil)
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
	filt := newACLFilter(acl.AllowAll(), nil)
	filt.filterSessions(&sessions)
	if len(sessions) != 2 {
		t.Fatalf("bad: %#v", sessions)
	}

	// Try restrictive filtering
	filt = newACLFilter(acl.DenyAll(), nil)
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
					{
						ID:      "foo",
						Service: "foo",
					},
				},
				Checks: []*structs.HealthCheck{
					{
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
		filt := newACLFilter(acl.AllowAll(), nil)
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
		filt := newACLFilter(acl.DenyAll(), nil)
		filt.filterNodeDump(&dump)
		if len(dump) != 0 {
			t.Fatalf("bad: %#v", dump)
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

	// But the node will block it.
	{
		dump := fill()
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(perms, nil)
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
	filt := newACLFilter(acl.AllowAll(), nil)
	filt.filterNodes(&nodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %#v", nodes)
	}

	// Try restrictive filtering
	filt = newACLFilter(acl.DenyAll(), nil)
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

	// Try permissive filtering.
	{
		dcNodes := fill(t)
		filt := newACLFilter(acl.AllowAll(), nil)
		filt.filterDatacenterCheckServiceNodes(&dcNodes)
		require.Len(t, dcNodes, 2)
		require.Equal(t, fill(t), dcNodes)
	}

	// Try restrictive filtering.
	{
		dcNodes := fill(t)
		filt := newACLFilter(acl.DenyAll(), nil)
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
		filt := newACLFilter(perms, nil)
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
		filt := newACLFilter(perms, nil)
		filt.filterDatacenterCheckServiceNodes(&dcNodes)
		require.Len(t, dcNodes, 0)
	}

	// Allowed to see the service AND the node
	policy, err = acl.NewPolicyFromSource("", 0, `
	service_prefix "" { policy = "read" }
	node_prefix "" { policy = "read" }
	`, acl.SyntaxCurrent, nil, nil)
	require.NoError(t, err)
	_, err = acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
	require.NoError(t, err)

	// Now it should go through.
	{
		dcNodes := fill(t)
		filt := newACLFilter(acl.AllowAll(), nil)
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
	filt := newACLFilter(acl.ManageAll(), nil)
	filt.redactPreparedQueryTokens(&query)
	if !reflect.DeepEqual(query, expected) {
		t.Fatalf("bad: %#v", &query)
	}

	// Hang on to the entry with a token, which needs to survive the next
	// operation.
	original := query

	// Now try permissive filtering with a client token, which should cause
	// the embedded token to get redacted.
	filt = newACLFilter(acl.AllowAll(), nil)
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

func TestFilterACL_redactTokenSecret(t *testing.T) {
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

	err := filterACL(r, "acl-wr", &token)
	require.NoError(t, err)
	require.Equal(t, "6a5e25b3-28f2-4085-9012-c3fb754314d1", token.SecretID)

	err = filterACL(r, "acl-ro", &token)
	require.NoError(t, err)
	require.Equal(t, redactedToken, token.SecretID)
}

func TestFilterACL_redactTokenSecrets(t *testing.T) {
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

	err := filterACL(r, "acl-wr", &tokens)
	require.NoError(t, err)
	require.Equal(t, "6a5e25b3-28f2-4085-9012-c3fb754314d1", tokens[0].SecretID)

	err = filterACL(r, "acl-ro", &tokens)
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
	filt := newACLFilter(acl.ManageAll(), nil)
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
	filt = newACLFilter(acl.AllowAll(), nil)
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
	filt = newACLFilter(acl.DenyAll(), nil)
	filt.filterPreparedQueries(&queries)
	if len(queries) != 0 {
		t.Fatalf("bad: %#v", queries)
	}
}

func TestACL_unhandledFilterType(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
func TestACL_LocalToken(t *testing.T) {
	t.Run("local token in same dc", func(t *testing.T) {
		d := &ACLResolverTestDelegate{
			datacenter: "dc1",
			tokenReadFn: func(_ *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				reply.Token = &structs.ACLToken{Local: true}
				// different dc
				reply.SourceDatacenter = "dc1"
				return nil
			},
		}
		r := newTestACLResolver(t, d, nil)
		_, err := r.fetchAndCacheIdentityFromToken("", nil)
		require.NoError(t, err)
	})

	t.Run("non local token in remote dc", func(t *testing.T) {
		d := &ACLResolverTestDelegate{
			datacenter: "dc1",
			tokenReadFn: func(_ *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				reply.Token = &structs.ACLToken{Local: false}
				// different dc
				reply.SourceDatacenter = "remote"
				return nil
			},
		}
		r := newTestACLResolver(t, d, nil)
		_, err := r.fetchAndCacheIdentityFromToken("", nil)
		require.NoError(t, err)
	})

	t.Run("local token in remote dc", func(t *testing.T) {
		d := &ACLResolverTestDelegate{
			datacenter: "dc1",
			tokenReadFn: func(_ *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				reply.Token = &structs.ACLToken{Local: true}
				// different dc
				reply.SourceDatacenter = "remote"
				return nil
			},
		}
		r := newTestACLResolver(t, d, nil)
		_, err := r.fetchAndCacheIdentityFromToken("", nil)
		require.Equal(t, acl.PermissionDeniedError{Cause: "This is a local token in datacenter \"remote\""}, err)
	})
}

func TestACLResolver_AgentMaster(t *testing.T) {
	var tokens token.Store

	d := &ACLResolverTestDelegate{
		datacenter: "dc1",
		enabled:    true,
	}
	r := newTestACLResolver(t, d, func(cfg *ACLResolverConfig) {
		cfg.Tokens = &tokens
		cfg.Config.NodeName = "foo"
		cfg.DisableDuration = 0
	})

	tokens.UpdateAgentMasterToken("9a184a11-5599-459e-b71a-550e5f9a5a23", token.TokenSourceConfig)

	ident, authz, err := r.ResolveTokenToIdentityAndAuthorizer("9a184a11-5599-459e-b71a-550e5f9a5a23")
	require.NoError(t, err)
	require.NotNil(t, ident)
	require.Equal(t, "agent-master:foo", ident.ID())
	require.NotNil(t, authz)
	require.Equal(t, r.agentMasterAuthz, authz)
	require.Equal(t, acl.Allow, authz.AgentWrite("foo", nil))
	require.Equal(t, acl.Allow, authz.NodeRead("bar", nil))
	require.Equal(t, acl.Deny, authz.NodeWrite("bar", nil))
}

func TestACLResolver_ACLsEnabled(t *testing.T) {
	type testCase struct {
		name     string
		resolver *ACLResolver
		enabled  bool
	}

	run := func(t *testing.T, tc testCase) {
		require.Equal(t, tc.enabled, tc.resolver.ACLsEnabled())
	}

	var testCases = []testCase{
		{
			name:     "config disabled",
			resolver: &ACLResolver{},
		},
		{
			name: "config enabled, disableDuration=0 (Server)",
			resolver: &ACLResolver{
				config: ACLResolverSettings{ACLsEnabled: true},
			},
			enabled: true,
		},
		{
			name: "config enabled, disabled by RPC (Client)",
			resolver: &ACLResolver{
				config:          ACLResolverSettings{ACLsEnabled: true},
				disableDuration: 10 * time.Second,
				disabledUntil:   time.Now().Add(5 * time.Second),
			},
		},
		{
			name: "config enabled, past disabledUntil (Client)",
			resolver: &ACLResolver{
				config:          ACLResolverSettings{ACLsEnabled: true},
				disableDuration: 10 * time.Second,
				disabledUntil:   time.Now().Add(-5 * time.Second),
			},
			enabled: true,
		},
		{
			name: "config enabled, no disabledUntil (Client)",
			resolver: &ACLResolver{
				config:          ACLResolverSettings{ACLsEnabled: true},
				disableDuration: 10 * time.Second,
			},
			enabled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}

}

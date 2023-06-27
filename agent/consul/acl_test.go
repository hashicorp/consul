// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
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

func verifyAuthorizerChain(t *testing.T, expected resolver.Result, actual resolver.Result) {
	t.Helper()
	expectedChainAuthz, ok := expected.Authorizer.(*acl.ChainedAuthorizer)
	require.True(t, ok, "expected Authorizer is not a ChainedAuthorizer")
	actualChainAuthz, ok := actual.Authorizer.(*acl.ChainedAuthorizer)
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

func resolveTokenSecret(t *testing.T, r *ACLResolver, token string) acl.Authorizer {
	t.Helper()
	authz, err := r.ResolveToken(token)
	require.NoError(t, err)
	return authz
}

func testIdentityForToken(token string) (bool, structs.ACLIdentity, error) {
	switch token {
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
	default:
		return true, nil, acl.ErrNotFound
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
			Datacenters: []string{"dc2"},
			RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
		}
		p.SetHash(false)
		return true, p, nil
	default:
		return true, nil, acl.ErrNotFound
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
		return true, nil, acl.ErrNotFound
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
	tokenReadFn     func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error
	policyResolveFn func(*structs.ACLPolicyBatchGetRequest, *structs.ACLPolicyBatchResponse) error
	roleResolveFn   func(*structs.ACLRoleBatchGetRequest, *structs.ACLRoleBatchResponse) error

	// testTokens is used by plainTokenReadFn if not nil
	testTokens map[string]*structs.ACLToken
	// testPolicies is used by plainPolicyResolveFn if not nil
	testPolicies map[string]*structs.ACLPolicy
	// testRoles is used by plainRoleResolveFn if not nil
	testRoles map[string]*structs.ACLRole

	testServerManagementToken string

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

func (d *ACLResolverTestDelegate) IsServerManagementToken(token string) bool {
	return token == d.testServerManagementToken
}

// UseTestLocalData will force delegate-local maps to be used in lieu of the
// global factory functions.
func (d *ACLResolverTestDelegate) UseTestLocalData(data []interface{}) {
	d.testTokens = make(map[string]*structs.ACLToken)
	d.testPolicies = make(map[string]*structs.ACLPolicy)
	d.testRoles = make(map[string]*structs.ACLRole)

	var rest []interface{}
	for _, item := range data {
		switch x := item.(type) {
		case *structs.ACLToken:
			d.testTokens[x.SecretID] = x
		case *structs.ACLPolicy:
			d.testPolicies[x.ID] = x
		case *structs.ACLRole:
			d.testRoles[x.ID] = x
		case string:
			parts := strings.SplitN(x, ":", 2)
			switch parts[0] {
			case "token-not-found":
				d.testTokens[parts[1]] = nil
			case "policy-not-found":
				d.testPolicies[parts[1]] = nil
			case "role-not-found":
				d.testRoles[parts[1]] = nil
			default:
				rest = append(rest, item)
			}
		default:
			rest = append(rest, item)
		}
	}

	d.EnterpriseACLResolverTestDelegate.UseTestLocalData(rest)
}

// UseDefaultData will force the global factory functions to be used instead of
// delegate-local maps.
func (d *ACLResolverTestDelegate) UseDefaultData() {
	d.testTokens = nil
	d.testPolicies = nil
	d.testRoles = nil
	d.EnterpriseACLResolverTestDelegate.UseDefaultData()
}

func (d *ACLResolverTestDelegate) Reset() {
	d.tokenCached = false
	d.policyCached = false
	d.roleCached = false
	d.EnterpriseACLResolverTestDelegate.Reset()
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
	if d.testTokens != nil {
		token, ok := d.testTokens[args.TokenID]
		if ok {
			if token == nil {
				return acl.ErrNotFound
			}
			reply.Token = token
		}
		return nil
	}

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
		if d.testPolicies != nil {
			if policy := d.testPolicies[policyID]; policy != nil {
				reply.Policies = append(reply.Policies, policy)
			}
		} else {
			_, policy, _ := testPolicyForID(policyID)
			if policy != nil {
				reply.Policies = append(reply.Policies, policy)
			}
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
		if d.testRoles != nil {
			if role := d.testRoles[roleID]; role != nil {
				reply.Roles = append(reply.Roles, role)
			}
		} else {
			_, role, _ := testRoleForID(roleID)
			if role != nil {
				reply.Roles = append(reply.Roles, role)
			}
		}
	}

	return nil
}

func (d *ACLResolverTestDelegate) ACLDatacenter() string {
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
	if d.testTokens != nil {
		if token, ok := d.testTokens[token]; ok {
			if token != nil {
				return true, token, nil
			}
		}
		return true, nil, acl.ErrNotFound
	}
	return testIdentityForToken(token)
}

func (d *ACLResolverTestDelegate) ResolvePolicyFromID(policyID string) (bool, *structs.ACLPolicy, error) {
	if !d.localPolicies {
		return false, nil, nil
	}

	atomic.AddInt32(&d.localPolicyResolutions, 1)
	if d.testPolicies != nil {
		if policy, ok := d.testPolicies[policyID]; ok {
			if policy != nil {
				return true, policy, nil
			}
		}
		return true, nil, acl.ErrNotFound
	}
	return testPolicyForID(policyID)
}

func (d *ACLResolverTestDelegate) ResolveRoleFromID(roleID string) (bool, *structs.ACLRole, error) {
	if !d.localRoles {
		return false, nil, nil
	}

	atomic.AddInt32(&d.localRoleResolutions, 1)
	if d.testRoles != nil {
		if role, ok := d.testRoles[roleID]; ok {
			if role != nil {
				return true, role, nil
			}
		}
		return true, nil, acl.ErrNotFound
	}
	return testRoleForID(roleID)
}

func (d *ACLResolverTestDelegate) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
	switch method {
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
	if handled, err := d.EnterpriseACLResolverTestDelegate.RPC(context.Background(), method, args, reply); handled {
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
		Backend:         delegate,
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
	require.Equal(t, resolver.Result{Authorizer: acl.ManageAll()}, authz)
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
		_, err := r.ResolveToken("allow")
		require.Error(t, err)
		require.True(t, acl.IsErrRootDenied(err))
	})

	t.Run("Deny", func(t *testing.T) {
		_, err := r.ResolveToken("deny")
		require.Error(t, err)
		require.True(t, acl.IsErrRootDenied(err))
	})

	t.Run("Manage", func(t *testing.T) {
		_, err := r.ResolveToken("manage")
		require.Error(t, err)
		require.True(t, acl.IsErrRootDenied(err))
	})
}

func TestACLResolver_DownPolicy(t *testing.T) {
	requireIdentityCached := func(t *testing.T, r *ACLResolver, secretID string, present bool, msg string) {
		t.Helper()

		cacheVal := r.cache.GetIdentityWithSecretToken(secretID)
		if present {
			require.NotNil(t, cacheVal, msg)
			require.NotNil(t, cacheVal.Identity, msg)
		} else {
			require.Nil(t, cacheVal, msg)
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
		expected := resolver.Result{
			Authorizer:  acl.DenyAll(),
			ACLIdentity: &missingIdentity{reason: "primary-dc-down", token: "foo"},
		}
		require.Equal(t, expected, authz)

		requireIdentityCached(t, r, "foo", false, "not present")
	})

	t.Run("Allow", func(t *testing.T) {
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
		expected := resolver.Result{
			Authorizer:  acl.AllowAll(),
			ACLIdentity: &missingIdentity{reason: "primary-dc-down", token: "foo"},
		}
		require.Equal(t, expected, authz)

		requireIdentityCached(t, r, "foo", false, "not present")
	})

	t.Run("Expired-Policy", func(t *testing.T) {
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

		requireIdentityCached(t, r, "found", true, "cached")

		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache with no cache entry defaults to default_policy", func(t *testing.T) {
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			localPolicies: true,
			localRoles:    true,
		}
		delegate.tokenReadFn = func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error {
			return ACLRemoteError{Err: fmt.Errorf("connection problem")}
		}

		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
		})

		authz, err := r.ResolveToken("not-found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache-Role", func(t *testing.T) {
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

		requireIdentityCached(t, r, "found-role", true, "still cached")

		authz2, err := r.ResolveToken("found-role")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache-Expired-Policy", func(t *testing.T) {
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

		requireIdentityCached(t, r, "found", true, "cached")

		// The identity should have been cached so this should still be valid
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		verifyAuthorizerChain(t, authz, authz2)
		require.Equal(t, acl.Allow, authz2.NodeWrite("foo", nil))

		// the go routine spawned will eventually return and this will be a not found error
		retry.Run(t, func(t *retry.R) {
			_, err := r.ResolveToken("found")
			assert.Error(t, err)
			assert.True(t, acl.IsErrNotFound(err))
		})

		requireIdentityCached(t, r, "found", false, "no longer cached")
	})

	t.Run("PolicyResolve-TokenNotFound", func(t *testing.T) {
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
		requireIdentityCached(t, r, secretID, true, "cached")
		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		// Nuke 1 policy from the cache so that we force a policy resolve
		// during token resolve.
		r.cache.RemovePolicy("dc2-key-wr")

		_, err = r.ResolveToken(secretID)
		require.True(t, acl.IsErrNotFound(err))

		requireIdentityCached(t, r, secretID, false, "identity not found cached")
		requirePolicyCached(t, r, "node-wr", true, "still cached")
		require.Nil(t, r.cache.GetPolicy("dc2-key-wr"), "not stored at all")
	})

	t.Run("PolicyResolve-PermissionDenied", func(t *testing.T) {
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
		requireIdentityCached(t, r, secretID, true, "cached")
		requirePolicyCached(t, r, "node-wr", true, "cached")    // from "found" token
		requirePolicyCached(t, r, "dc2-key-wr", true, "cached") // from "found" token

		// Nuke 1 policy from the cache so that we force a policy resolve
		// during token resolve.
		r.cache.RemovePolicy("dc2-key-wr")

		_, err = r.ResolveToken(secretID)
		require.True(t, acl.IsErrPermissionDenied(err))

		require.Nil(t, r.cache.GetIdentityWithSecretToken(secretID), "identity not stored at all")
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
		_, err = r.ResolveToken("a1a54629-5050-4d17-8a4e-560d2423f835")
		require.EqualError(t, err, acl.ErrNotFound.Error())

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
		delegate.UseTestLocalData(nil)
		_, err := r.ResolveToken("doesn't exist")
		require.Error(t, err)
		require.True(t, acl.IsErrNotFound(err))
	})

	runTwiceAndReset("Missing Policy", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
				SecretID:   "missing-policy",
				Policies: []structs.ACLTokenPolicyLink{
					{ID: "not-found"},
					{ID: "acl-ro"},
				},
			},
			"policy-not-found:not-found",
			&structs.ACLPolicy{
				ID:          "acl-ro",
				Name:        "acl-ro",
				Description: "acl-ro",
				Rules:       `acl = "read"`,
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz := resolveTokenSecret(t, r, "missing-policy")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Missing Role", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
				SecretID:   "missing-role",
				Roles: []structs.ACLTokenRoleLink{
					{ID: "not-found"},
					{ID: "acl-ro"},
				},
			},
			"role-not-found:not-found",
			&structs.ACLRole{
				ID:          "acl-ro",
				Name:        "acl-ro",
				Description: "acl-ro",
				Policies: []structs.ACLRolePolicyLink{
					{ID: "acl-ro"},
				},
			},
			&structs.ACLPolicy{
				ID:          "acl-ro",
				Name:        "acl-ro",
				Description: "acl-ro",
				Rules:       `acl = "read"`,
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz := resolveTokenSecret(t, r, "missing-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Missing Policy on Role", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "435a75af-1763-4980-89f4-f0951dda53b4",
				SecretID:   "missing-policy-on-role",
				Roles: []structs.ACLTokenRoleLink{
					{ID: "missing-policy"},
				},
			},
			&structs.ACLRole{
				ID:          "missing-policy",
				Name:        "missing-policy",
				Description: "missing-policy",
				Policies: []structs.ACLRolePolicyLink{
					{ID: "not-found"},
					{ID: "acl-ro"},
				},
				RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
			"policy-not-found:not-found",
			&structs.ACLPolicy{
				ID:          "acl-ro",
				Name:        "acl-ro",
				Description: "acl-ro",
				Rules:       `acl = "read"`,
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz := resolveTokenSecret(t, r, "missing-policy-on-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ACLRead(nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Policy", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
				SecretID:   "found",
				Policies: []structs.ACLTokenPolicyLink{
					{ID: "node-wr"},
					{ID: "dc2-key-wr"},
				},
			},
			&structs.ACLPolicy{
				ID:          "node-wr",
				Name:        "node-wr",
				Description: "node-wr",
				Rules:       `node_prefix "" { policy = "write"}`,
				Datacenters: []string{"dc1"},
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
			&structs.ACLPolicy{
				ID:          "dc2-key-wr",
				Name:        "dc2-key-wr",
				Description: "dc2-key-wr",
				Rules:       `key_prefix "" { policy = "write"}`,
				Datacenters: []string{"dc2"},
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz := resolveTokenSecret(t, r, "found")
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Role", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
				SecretID:   "found-role",
				Roles: []structs.ACLTokenRoleLink{
					{ID: "found"},
				},
			},
			&structs.ACLRole{
				ID:          "found",
				Name:        "found",
				Description: "found",
				Policies: []structs.ACLRolePolicyLink{
					{ID: "node-wr"},
					{ID: "dc2-key-wr"},
				},
			},
			&structs.ACLPolicy{
				ID:          "node-wr",
				Name:        "node-wr",
				Description: "node-wr",
				Rules:       `node_prefix "" { policy = "write"}`,
				Datacenters: []string{"dc1"},
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
			&structs.ACLPolicy{
				ID:          "dc2-key-wr",
				Name:        "dc2-key-wr",
				Description: "dc2-key-wr",
				Rules:       `key_prefix "" { policy = "write"}`,
				Datacenters: []string{"dc2"},
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz := resolveTokenSecret(t, r, "found-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("Normal with Policy and Role", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
				SecretID:   "found-policy-and-role",
				Policies: []structs.ACLTokenPolicyLink{
					{ID: "node-wr"},
					{ID: "dc2-key-wr"},
				},
				Roles: []structs.ACLTokenRoleLink{
					{ID: "service-ro"},
				},
			},
			&structs.ACLPolicy{
				ID:          "node-wr",
				Name:        "node-wr",
				Description: "node-wr",
				Rules:       `node_prefix "" { policy = "write"}`,
				Datacenters: []string{"dc1"},
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
			&structs.ACLPolicy{
				ID:          "dc2-key-wr",
				Name:        "dc2-key-wr",
				Description: "dc2-key-wr",
				Rules:       `key_prefix "" { policy = "write"}`,
				Datacenters: []string{"dc2"},
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
			&structs.ACLRole{
				ID:          "service-ro",
				Name:        "service-ro",
				Description: "service-ro",
				Policies: []structs.ACLRolePolicyLink{
					{ID: "service-ro"},
				},
				RaftIndex: structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
			&structs.ACLPolicy{
				ID:          "service-ro",
				Name:        "service-ro",
				Description: "service-ro",
				Rules:       `service_prefix "" { policy = "read" }`,
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz := resolveTokenSecret(t, r, "found-policy-and-role")
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("bar", nil))
	})

	runTwiceAndReset("Role With Node Identity", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "f3f47a09-de29-4c57-8f54-b65a9be79641",
				SecretID:   "found-role-node-identity",
				Roles: []structs.ACLTokenRoleLink{
					{ID: "node-identity"},
				},
			},
			&structs.ACLRole{
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
			},
		})
		authz := resolveTokenSecret(t, r, "found-role-node-identity")
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.NodeWrite("test-node", nil))
		require.Equal(t, acl.Deny, authz.NodeWrite("test-node-dc2", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("something", nil))
		require.Equal(t, acl.Deny, authz.ServiceWrite("something", nil))
	})

	runTwiceAndReset("Synthetic Policies Independently Cache", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "f6c5a5fb-4da4-422b-9abf-2c942813fc71",
				SecretID:   "found-synthetic-policy-1",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: "service1"},
				},
			},
			&structs.ACLToken{
				AccessorID: "7c87dfad-be37-446e-8305-299585677cb5",
				SecretID:   "found-synthetic-policy-2",
				ServiceIdentities: []*structs.ACLServiceIdentity{
					{ServiceName: "service2"},
				},
			},
			&structs.ACLToken{
				AccessorID: "bebccc92-3987-489d-84c2-ffd00d93ef93",
				SecretID:   "found-synthetic-policy-3",
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
			},
			&structs.ACLToken{
				AccessorID: "359b9927-25fd-46b9-bd14-3470f848ec65",
				SecretID:   "found-synthetic-policy-4",
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
			},
		})

		// We resolve these tokens in the same cache session
		// to verify that the keys for caching synthetic policies don't bleed
		// over between each other.
		t.Run("synthetic-policy-1", func(t *testing.T) { // service identity
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
		})
		t.Run("synthetic-policy-2", func(t *testing.T) { // service identity
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
		})
		t.Run("synthetic-policy-3", func(t *testing.T) { // node identity
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
		})
		t.Run("synthetic-policy-4", func(t *testing.T) { // node identity
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
		})
	})

	runTwiceAndReset("Anonymous", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "00000000-0000-0000-0000-000000000002",
				SecretID:   anonymousToken,
				Policies: []structs.ACLTokenPolicyLink{
					{ID: "node-wr"},
				},
			},
			&structs.ACLPolicy{
				ID:          "node-wr",
				Name:        "node-wr",
				Description: "node-wr",
				Rules:       `node_prefix "" { policy = "write"}`,
				Datacenters: []string{"dc1"},
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz, err := r.ResolveToken("")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.Equal(t, acl.Deny, authz.ACLRead(nil))
		require.Equal(t, acl.Allow, authz.NodeWrite("foo", nil))
	})

	runTwiceAndReset("service and intention wildcard write", func(t *testing.T) {
		delegate.UseTestLocalData([]interface{}{
			&structs.ACLToken{
				AccessorID: "5f57c1f6-6a89-4186-9445-531b316e01df",
				SecretID:   "with-intentions",
				Policies: []structs.ACLTokenPolicyLink{
					{ID: "ixn-write"},
				},
			},
			&structs.ACLPolicy{
				ID:          "ixn-write",
				Name:        "ixn-write",
				Description: "ixn-write",
				Rules:       `service_prefix "" { policy = "write" intentions = "write" }`,
				RaftIndex:   structs.RaftIndex{CreateIndex: 1, ModifyIndex: 2},
			},
		})
		authz, err := r.ResolveToken("with-intentions")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.ServiceRead("", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("foo", nil))
		require.Equal(t, acl.Allow, authz.ServiceRead("bar", nil))
		require.Equal(t, acl.Allow, authz.ServiceWrite("", nil))
		require.Equal(t, acl.Allow, authz.ServiceWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.ServiceWrite("bar", nil))
		require.Equal(t, acl.Allow, authz.IntentionRead("", nil))
		require.Equal(t, acl.Allow, authz.IntentionRead("foo", nil))
		require.Equal(t, acl.Allow, authz.IntentionRead("bar", nil))
		require.Equal(t, acl.Allow, authz.IntentionWrite("", nil))
		require.Equal(t, acl.Allow, authz.IntentionWrite("foo", nil))
		require.Equal(t, acl.Allow, authz.IntentionWrite("bar", nil))
		require.Equal(t, acl.Deny, authz.NodeRead("server", nil))
	})
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

func TestACLResolver_AgentRecovery(t *testing.T) {
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

	tokens.UpdateAgentRecoveryToken("9a184a11-5599-459e-b71a-550e5f9a5a23", token.TokenSourceConfig)

	authz, err := r.ResolveToken("9a184a11-5599-459e-b71a-550e5f9a5a23")
	require.NoError(t, err)
	require.NotNil(t, authz.ACLIdentity)
	require.Equal(t, "agent-recovery:foo", authz.ACLIdentity.ID())
	require.NotNil(t, authz.Authorizer)
	require.Equal(t, r.agentRecoveryAuthz, authz.Authorizer)
	require.Equal(t, acl.Allow, authz.AgentWrite("foo", nil))
	require.Equal(t, acl.Allow, authz.NodeRead("bar", nil))
	require.Equal(t, acl.Deny, authz.NodeWrite("bar", nil))
}

func TestACLResolver_ServerManagementToken(t *testing.T) {
	const testToken = "1bb0900e-3683-46a5-b04c-4882d7773b83"

	d := &ACLResolverTestDelegate{
		datacenter:                "dc1",
		enabled:                   true,
		testServerManagementToken: testToken,
	}
	r := newTestACLResolver(t, d, func(cfg *ACLResolverConfig) {
		cfg.Tokens = &token.Store{}
		cfg.Config.NodeName = "foo"
	})

	authz, err := r.ResolveToken(testToken)
	require.NoError(t, err)
	require.NotNil(t, authz.ACLIdentity)
	require.Equal(t, structs.ServerManagementTokenAccessorID, authz.ACLIdentity.ID())
	require.NotNil(t, authz.Authorizer)
	require.Equal(t, acl.ManageAll(), authz.Authorizer)
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

func TestACLResolver_ResolveToken_UpdatesPurgeTheCache(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	_, srv, codec := testACLServerWithConfig(t, nil, false)
	waitForLeaderEstablishment(t, srv)

	reqPolicy := structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			Name:  "the-policy",
			Rules: `key_prefix "" { policy = "read"}`,
		},
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}
	var respPolicy = structs.ACLPolicy{}
	err := msgpackrpc.CallWithCodec(codec, "ACL.PolicySet", &reqPolicy, &respPolicy)
	require.NoError(t, err)

	token, err := uuid.GenerateUUID()
	require.NoError(t, err)

	reqToken := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			SecretID: token,
			Policies: []structs.ACLTokenPolicyLink{{Name: "the-policy"}},
		},
		WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
	}
	var respToken structs.ACLToken
	err = msgpackrpc.CallWithCodec(codec, "ACL.TokenSet", &reqToken, &respToken)
	require.NoError(t, err)

	testutil.RunStep(t, "first resolve", func(t *testing.T) {
		authz, err := srv.ACLResolver.ResolveToken(token)
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Allow, authz.KeyRead("foo", nil))
	})

	testutil.RunStep(t, "update the policy and resolve again", func(t *testing.T) {
		reqPolicy := structs.ACLPolicySetRequest{
			Datacenter: "dc1",
			Policy: structs.ACLPolicy{
				ID:    respPolicy.ID,
				Name:  "the-policy",
				Rules: `{"key_prefix": {"": {"policy": "deny"}}}`,
			},
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		err := msgpackrpc.CallWithCodec(codec, "ACL.PolicySet", &reqPolicy, &structs.ACLPolicy{})
		require.NoError(t, err)

		authz, err := srv.ACLResolver.ResolveToken(token)
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, acl.Deny, authz.KeyRead("foo", nil))
	})

	testutil.RunStep(t, "delete the token", func(t *testing.T) {
		req := structs.ACLTokenDeleteRequest{
			Datacenter:   "dc1",
			TokenID:      respToken.AccessorID,
			WriteRequest: structs.WriteRequest{Token: TestDefaultInitialManagementToken},
		}
		var resp string
		err := msgpackrpc.CallWithCodec(codec, "ACL.TokenDelete", &req, &resp)
		require.NoError(t, err)

		_, err = srv.ACLResolver.ResolveToken(token)
		require.True(t, acl.IsErrNotFound(err), "Error %v is not acl.ErrNotFound", err)
	})
}

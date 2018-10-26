package consul

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testutil/retry"
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
		return true, nil, acl.ErrNotFound
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
		return true, nil, acl.ErrNotFound
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
	getPolicyFn     func(*structs.ACLPolicyResolveLegacyRequest, *structs.ACLPolicyResolveLegacyResponse) error
	tokenReadFn     func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error
	policyResolveFn func(*structs.ACLPolicyBatchGetRequest, *structs.ACLPolicyBatchResponse) error
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

func (d *ACLResolverTestDelegate) RPC(method string, args interface{}, reply interface{}) error {
	switch method {
	case "ACL.GetPolicy":
		if d.getPolicyFn != nil {
			return d.getPolicyFn(args.(*structs.ACLPolicyResolveLegacyRequest), reply.(*structs.ACLPolicyResolveLegacyResponse))
		}
		panic("Bad Test Implmentation: should provide a getPolicyFn to the ACLResolverTestDelegate")
	case "ACL.TokenRead":
		if d.tokenReadFn != nil {
			return d.tokenReadFn(args.(*structs.ACLTokenGetRequest), reply.(*structs.ACLTokenResponse))
		}
		panic("Bad Test Implmentation: should provide a tokenReadFn to the ACLResolverTestDelegate")
	case "ACL.PolicyResolve":
		if d.policyResolveFn != nil {
			return d.policyResolveFn(args.(*structs.ACLPolicyBatchGetRequest), reply.(*structs.ACLPolicyBatchResponse))
		}
		panic("Bad Test Implmentation: should provide a policyResolveFn to the ACLResolverTestDelegate")
	}
	panic("Bad Test Implementation: Was the ACLResolver updated to use new RPC methods")
}

func newTestACLResolver(t *testing.T, delegate ACLResolverDelegate, cb func(*ACLResolverConfig)) *ACLResolver {
	config := DefaultConfig()
	config.ACLDefaultPolicy = "deny"
	config.ACLDownPolicy = "extend-cache"
	rconf := &ACLResolverConfig{
		Config: config,
		Logger: log.New(os.Stdout, t.Name()+" - ", log.LstdFlags|log.Lmicroseconds),
		CacheConfig: &structs.ACLCachesConfig{
			Identities:     4,
			Policies:       4,
			ParsedPolicies: 4,
			Authorizers:    4,
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

	t.Run("Deny", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			tokenReadFn: func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error {
				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "deny"
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, authz, acl.DenyAll())
	})

	t.Run("Allow", func(t *testing.T) {
		t.Parallel()
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			tokenReadFn: func(*structs.ACLTokenGetRequest, *structs.ACLTokenResponse) error {
				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "allow"
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.Equal(t, authz, acl.AllowAll())
	})

	t.Run("Expired-Policy", func(t *testing.T) {
		t.Parallel()
		policyCached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				if !policyCached {
					for _, policyID := range args.PolicyIDs {
						_, policy, _ := testPolicyForID(policyID)
						if policy != nil {
							reply.Policies = append(reply.Policies, policy)
						}
					}

					policyCached = true
					return nil
				}

				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "deny"
			config.Config.ACLPolicyTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz.NodeWrite("foo", nil))

		// policy cache expired - so we will fail to resolve that policy and use the default policy only
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		require.False(t, authz == authz2)
		require.False(t, authz2.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache", func(t *testing.T) {
		t.Parallel()
		cached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				if !cached {
					_, token, _ := testIdentityForToken("found")
					reply.Token = token.(*structs.ACLToken)
					cached = true
					return nil
				}
				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz.NodeWrite("foo", nil))

		authz2, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		require.True(t, authz == authz2)
		require.True(t, authz.NodeWrite("foo", nil))
	})

	t.Run("Extend-Cache-Expired-Policy", func(t *testing.T) {
		t.Parallel()
		policyCached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				if !policyCached {
					for _, policyID := range args.PolicyIDs {
						_, policy, _ := testPolicyForID(policyID)
						if policy != nil {
							reply.Policies = append(reply.Policies, policy)
						}
					}

					policyCached = true
					return nil
				}

				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLPolicyTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz.NodeWrite("foo", nil))

		// Will just use the policy cache
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		require.True(t, authz == authz2)
		require.True(t, authz.NodeWrite("foo", nil))
	})

	t.Run("Async-Cache-Expired-Policy", func(t *testing.T) {
		t.Parallel()
		policyCached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   true,
			localPolicies: false,
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				if !policyCached {
					for _, policyID := range args.PolicyIDs {
						_, policy, _ := testPolicyForID(policyID)
						if policy != nil {
							reply.Policies = append(reply.Policies, policy)
						}
					}

					policyCached = true
					return nil
				}

				// We don't need to return acl.ErrNotFound here but we could. The ACLResolver will search for any
				// policies not in the response and emit an ACL not found for any not-found within the result set.
				return nil
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "async-cache"
			config.Config.ACLPolicyTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz.NodeWrite("foo", nil))

		// The identity should have been cached so this should still be valid
		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		require.True(t, authz == authz2)
		require.True(t, authz.NodeWrite("foo", nil))

		// the go routine spawned will eventually return with a authz that doesn't have the policy
		retry.Run(t, func(t *retry.R) {
			authz3, err := r.ResolveToken("found")
			assert.NoError(t, err)
			assert.NotNil(t, authz3)
			assert.False(t, authz3.NodeWrite("foo", nil))
		})
	})

	t.Run("Extend-Cache-Client", func(t *testing.T) {
		t.Parallel()
		tokenCached := false
		policyCached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: false,
			tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				if !tokenCached {
					_, token, _ := testIdentityForToken("found")
					reply.Token = token.(*structs.ACLToken)
					tokenCached = true
					return nil
				}
				return fmt.Errorf("Induced RPC Error")
			},
			policyResolveFn: func(args *structs.ACLPolicyBatchGetRequest, reply *structs.ACLPolicyBatchResponse) error {
				if !policyCached {
					for _, policyID := range args.PolicyIDs {
						_, policy, _ := testPolicyForID(policyID)
						if policy != nil {
							reply.Policies = append(reply.Policies, policy)
						}
					}

					policyCached = true
					return nil
				}

				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLDownPolicy = "extend-cache"
			config.Config.ACLTokenTTL = 0
			config.Config.ACLPolicyTTL = 0
		})

		authz, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz.NodeWrite("foo", nil))

		authz2, err := r.ResolveToken("found")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		require.True(t, authz == authz2)
		require.True(t, authz.NodeWrite("foo", nil))
	})

	t.Run("Async-Cache", func(t *testing.T) {
		t.Parallel()
		cached := false
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc1",
			legacy:        false,
			localTokens:   false,
			localPolicies: true,
			tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
				if !cached {
					_, token, _ := testIdentityForToken("found")
					reply.Token = token.(*structs.ACLToken)
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
		require.True(t, authz.NodeWrite("foo", nil))

		// The identity should have been cached so this should still be valid
		authz2, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz2)
		// testing pointer equality - these will be the same object because it is cached.
		require.True(t, authz == authz2)
		require.True(t, authz.NodeWrite("foo", nil))

		// the go routine spawned will eventually return and this will be a not found error
		retry.Run(t, func(t *retry.R) {
			authz3, err := r.ResolveToken("foo")
			assert.Error(t, err)
			assert.True(t, acl.IsErrNotFound(err))
			assert.Nil(t, authz3)
		})
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
			// No need to provide any of the RPC callbacks
		}
		r := newTestACLResolver(t, delegate, nil)

		authz, err := r.ResolveToken("found")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.False(t, authz.ACLRead())
		require.True(t, authz.NodeWrite("foo", nil))
		require.False(t, authz.KeyWrite("foo", nil))
	})

	t.Run("dc2", func(t *testing.T) {
		delegate := &ACLResolverTestDelegate{
			enabled:       true,
			datacenter:    "dc2",
			legacy:        false,
			localTokens:   true,
			localPolicies: true,
			// No need to provide any of the RPC callbacks
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.Datacenter = "dc2"
		})

		authz, err := r.ResolveToken("found")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.False(t, authz.ACLRead())
		require.False(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.KeyWrite("foo", nil))
	})
}

func TestACLResolver_LocalTokensAndPolicies(t *testing.T) {
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

	t.Run("Missing Identity", func(t *testing.T) {
		authz, err := r.ResolveToken("doesn't exist")
		require.Nil(t, authz)
		require.Error(t, err)
		require.True(t, acl.IsErrNotFound(err))
	})

	t.Run("Missing Policy", func(t *testing.T) {
		authz, err := r.ResolveToken("missing-policy")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz.ACLRead())
		require.False(t, authz.NodeWrite("foo", nil))
	})

	t.Run("Normal", func(t *testing.T) {
		authz, err := r.ResolveToken("found")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.False(t, authz.ACLRead())
		require.True(t, authz.NodeWrite("foo", nil))
	})

	t.Run("Anonymous", func(t *testing.T) {
		authz, err := r.ResolveToken("")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.False(t, authz.ACLRead())
		require.True(t, authz.NodeWrite("foo", nil))
	})

	t.Run("legacy-management", func(t *testing.T) {
		authz, err := r.ResolveToken("legacy-management")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.True(t, authz.ACLWrite())
		require.True(t, authz.KeyRead("foo"))
	})

	t.Run("legacy-client", func(t *testing.T) {
		authz, err := r.ResolveToken("legacy-client")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.False(t, authz.OperatorRead())
		require.True(t, authz.ServiceRead("foo"))
	})
}

func TestACLResolver_LocalPolicies(t *testing.T) {
	t.Parallel()
	delegate := &ACLResolverTestDelegate{
		enabled:       true,
		datacenter:    "dc1",
		legacy:        false,
		localTokens:   false,
		localPolicies: true,
		tokenReadFn: func(args *structs.ACLTokenGetRequest, reply *structs.ACLTokenResponse) error {
			_, token, err := testIdentityForToken(args.TokenID)

			if token != nil {
				reply.Token = token.(*structs.ACLToken)
			}
			return err
		},
	}
	r := newTestACLResolver(t, delegate, nil)

	t.Run("Missing Identity", func(t *testing.T) {
		authz, err := r.ResolveToken("doesn't exist")
		require.Nil(t, authz)
		require.Error(t, err)
		require.True(t, acl.IsErrNotFound(err))
	})

	t.Run("Missing Policy", func(t *testing.T) {
		authz, err := r.ResolveToken("missing-policy")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.True(t, authz.ACLRead())
		require.False(t, authz.NodeWrite("foo", nil))
	})

	t.Run("Normal", func(t *testing.T) {
		authz, err := r.ResolveToken("found")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.False(t, authz.ACLRead())
		require.True(t, authz.NodeWrite("foo", nil))
	})

	t.Run("Anonymous", func(t *testing.T) {
		authz, err := r.ResolveToken("")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.False(t, authz.ACLRead())
		require.True(t, authz.NodeWrite("foo", nil))
	})

	t.Run("legacy-management", func(t *testing.T) {
		authz, err := r.ResolveToken("legacy-management")
		require.NotNil(t, authz)
		require.NoError(t, err)
		require.True(t, authz.ACLWrite())
		require.True(t, authz.KeyRead("foo"))
	})

	t.Run("legacy-client", func(t *testing.T) {
		authz, err := r.ResolveToken("legacy-client")
		require.NoError(t, err)
		require.NotNil(t, authz)
		require.False(t, authz.OperatorRead())
		require.True(t, authz.ServiceRead("foo"))
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
						Nodes: []*acl.NodePolicy{
							&acl.NodePolicy{
								Name:   "foo",
								Policy: acl.PolicyWrite,
							},
						},
					}
					cached = true
					return nil
				}
				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, nil)

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))
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
						Nodes: []*acl.NodePolicy{
							&acl.NodePolicy{
								Name:   "foo",
								Policy: acl.PolicyWrite,
							},
						},
					}
					cached = true
					return nil
				}
				return fmt.Errorf("Induced RPC Error")
			},
		}
		r := newTestACLResolver(t, delegate, func(config *ACLResolverConfig) {
			config.Config.ACLTokenTTL = 0
		})

		authz, err := r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))
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
						Nodes: []*acl.NodePolicy{
							&acl.NodePolicy{
								Name:   "foo",
								Policy: acl.PolicyWrite,
							},
						},
					}
					cached = true
					return nil
				}
				return fmt.Errorf("Induced RPC Error")
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
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.True(t, authz.NodeWrite("fo", nil))
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
						Nodes: []*acl.NodePolicy{
							&acl.NodePolicy{
								Name:   "foo",
								Policy: acl.PolicyWrite,
							},
						},
					}
					cached = true
					return nil
				}
				return fmt.Errorf("Induced RPC Error")
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
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))

		// this should be from the cache
		authz, err = r.ResolveToken("foo")
		require.NoError(t, err)
		require.NotNil(t, authz)
		// there is a bit of translation that happens
		require.False(t, authz.NodeWrite("foo", nil))
		require.False(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))
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
						Nodes: []*acl.NodePolicy{
							&acl.NodePolicy{
								Name:   "foo",
								Policy: acl.PolicyWrite,
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
		require.True(t, authz.NodeWrite("foo", nil))
		require.True(t, authz.NodeWrite("foo/bar", nil))
		require.False(t, authz.NodeWrite("fo", nil))

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
		s2.tokens.UpdateACLReplicationToken("root")
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
		s3.tokens.UpdateACLReplicationToken("root")
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	assert.Nil(err)
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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
	filt.filterServices(services)
	if len(services) != 3 {
		t.Fatalf("bad: %#v", services)
	}

	// Try restrictive filtering.
	filt = newACLFilter(acl.DenyAll(), nil, false)
	filt.filterServices(services)
	if len(services) != 1 {
		t.Fatalf("bad: %#v", services)
	}
	if _, ok := services["consul"]; !ok {
		t.Fatalf("bad: %#v", services)
	}

	// Try restrictive filtering with version 8 enforcement.
	filt = newACLFilter(acl.DenyAll(), nil, true)
	filt.filterServices(services)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err = acl.NewPolicyAuthorizer(perms, []*acl.Policy{policy}, nil)
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
service "service" {
  policy = "write"
}
`, acl.SyntaxLegacy, nil)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	perms, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// With that policy, the update should now be blocked for node reasons.
	err = vetDeregisterWithACL(perms, args, nil, nil)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Now use a permitted node name.
	args.Node = "node"
	if err := vetDeregisterWithACL(perms, args, nil, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try an unknown check.
	args.CheckID = "check-id"
	err = vetDeregisterWithACL(perms, args, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Unknown check") {
		t.Fatalf("bad: %v", err)
	}

	// Now pass in a check that should be blocked.
	nc := &structs.HealthCheck{
		Node:        "node",
		CheckID:     "check-id",
		ServiceID:   "service-id",
		ServiceName: "nope",
	}
	err = vetDeregisterWithACL(perms, args, nil, nc)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Change it to an allowed service, which should go through.
	nc.ServiceName = "service"
	if err := vetDeregisterWithACL(perms, args, nil, nc); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Switch to a node check that should be blocked.
	args.Node = "nope"
	nc.Node = "nope"
	nc.ServiceID = ""
	nc.ServiceName = ""
	err = vetDeregisterWithACL(perms, args, nil, nc)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Switch to an allowed node check, which should go through.
	args.Node = "node"
	nc.Node = "node"
	if err := vetDeregisterWithACL(perms, args, nil, nc); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try an unknown service.
	args.ServiceID = "service-id"
	err = vetDeregisterWithACL(perms, args, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Unknown service") {
		t.Fatalf("bad: %v", err)
	}

	// Now pass in a service that should be blocked.
	ns := &structs.NodeService{
		ID:      "service-id",
		Service: "nope",
	}
	err = vetDeregisterWithACL(perms, args, ns, nil)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("bad: %v", err)
	}

	// Change it to an allowed service, which should go through.
	ns.Service = "service"
	if err := vetDeregisterWithACL(perms, args, ns, nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	envoy_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func TestRemoveIntentionPrecedence(t *testing.T) {
	type ixnOpts struct {
		src    string
		peer   string
		action structs.IntentionAction
	}
	testIntention := func(t *testing.T, opts ixnOpts) *structs.Intention {
		t.Helper()
		ixn := structs.TestIntention(t)
		ixn.SourceName = opts.src
		ixn.SourcePeer = opts.peer
		ixn.Action = opts.action

		// Destination is hardcoded, since RBAC rules are generated for a single destination
		ixn.DestinationName = "api"

		//nolint:staticcheck
		ixn.UpdatePrecedence()
		return ixn
	}
	testSourceIntention := func(opts ixnOpts) *structs.Intention {
		return testIntention(t, opts)
	}
	testSourcePermIntention := func(src string, perms ...*structs.IntentionPermission) *structs.Intention {
		opts := ixnOpts{src: src}
		ixn := testIntention(t, opts)
		ixn.Permissions = perms
		return ixn
	}
	sorted := func(ixns ...*structs.Intention) structs.SimplifiedIntentions {
		sort.SliceStable(ixns, func(i, j int) bool {
			return ixns[j].Precedence < ixns[i].Precedence
		})
		return structs.SimplifiedIntentions(ixns)
	}
	testPeerTrustBundle := map[string]*pbpeering.PeeringTrustBundle{
		"peer1": {
			PeerName:          "peer1",
			TrustDomain:       "peer1.domain",
			ExportedPartition: "part1",
		},
	}
	testTrustDomain := "test.consul"

	var (
		nameWild = rbacService{ServiceName: structs.NewServiceName("*", nil),
			TrustDomain: testTrustDomain}
		nameWeb = rbacService{ServiceName: structs.NewServiceName("web", nil),
			TrustDomain: testTrustDomain}
		nameWildPeered = rbacService{ServiceName: structs.NewServiceName("*", nil),
			Peer: "peer1", TrustDomain: "peer1.domain", ExportedPartition: "part1"}
		nameWebPeered = rbacService{ServiceName: structs.NewServiceName("web", nil),
			Peer: "peer1", TrustDomain: "peer1.domain", ExportedPartition: "part1"}
		permSlashPrefix = &structs.IntentionPermission{
			Action: structs.IntentionActionAllow,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/",
			},
		}
		permDenySlashPrefix = &structs.IntentionPermission{
			Action: structs.IntentionActionDeny,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/",
			},
		}
		xdsPermSlashPrefix = &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_UrlPath{
				UrlPath: &envoy_matcher_v3.PathMatcher{
					Rule: &envoy_matcher_v3.PathMatcher_Path{
						Path: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Prefix{
								Prefix: "/",
							},
						},
					},
				},
			},
		}
	)

	// NOTE: these default=(allow|deny) wild=(allow|deny) path=(allow|deny)
	// tests below are meant to verify some of the behaviors work as expected
	// when the default acl mode changes for the system
	tests := map[string]struct {
		intentionDefaultAllow bool
		http                  bool
		intentions            structs.SimplifiedIntentions
		expect                []*rbacIntention
	}{
		"default-allow-path-allow": {
			intentionDefaultAllow: true,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
			),
			expect: []*rbacIntention{}, // EMPTY, just use the defaults
		},
		"default-deny-path-allow": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
			),
			expect: []*rbacIntention{
				{
					Source: nameWeb,
					Action: intentionActionLayer7,
					Permissions: []*rbacPermission{
						{
							Definition:         permSlashPrefix,
							Action:             intentionActionAllow,
							Perm:               xdsPermSlashPrefix,
							NotPerms:           nil,
							Skip:               false,
							ComputedPermission: xdsPermSlashPrefix,
						},
					},
					Precedence:        9,
					Skip:              false,
					ComputedPrincipal: idPrincipal(nameWeb),
				},
			},
		},
		"default-allow-path-deny": {
			intentionDefaultAllow: true,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
			),
			expect: []*rbacIntention{
				{
					Source: nameWeb,
					Action: intentionActionLayer7,
					Permissions: []*rbacPermission{
						{
							Definition:         permDenySlashPrefix,
							Action:             intentionActionDeny,
							Perm:               xdsPermSlashPrefix,
							NotPerms:           nil,
							Skip:               false,
							ComputedPermission: xdsPermSlashPrefix,
						},
					},
					Precedence:        9,
					Skip:              false,
					ComputedPrincipal: idPrincipal(nameWeb),
				},
			},
		},
		"default-deny-path-deny": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
			),
			expect: []*rbacIntention{},
		},
		// ========================
		"default-allow-deny-all-and-path-allow": {
			intentionDefaultAllow: true,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionDeny}),
			),
			expect: []*rbacIntention{
				{
					Source: nameWild,
					NotSources: []rbacService{
						nameWeb,
					},
					Action:      intentionActionDeny,
					Permissions: nil,
					Precedence:  8,
					Skip:        false,
					ComputedPrincipal: andPrincipals(
						[]*envoy_rbac_v3.Principal{
							idPrincipal(nameWild),
							notPrincipal(
								idPrincipal(nameWeb),
							),
						},
					),
				},
			},
		},
		"default-deny-deny-all-and-path-allow": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionDeny}),
			),
			expect: []*rbacIntention{
				{
					Source: nameWeb,
					Action: intentionActionLayer7,
					Permissions: []*rbacPermission{
						{
							Definition:         permSlashPrefix,
							Action:             intentionActionAllow,
							Perm:               xdsPermSlashPrefix,
							NotPerms:           nil,
							Skip:               false,
							ComputedPermission: xdsPermSlashPrefix,
						},
					},
					Precedence:        9,
					Skip:              false,
					ComputedPrincipal: idPrincipal(nameWeb),
				},
			},
		},
		"default-allow-deny-all-and-path-deny": {
			intentionDefaultAllow: true,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionDeny}),
			),
			expect: []*rbacIntention{
				{
					Source: nameWeb,
					Action: intentionActionLayer7,
					Permissions: []*rbacPermission{
						{
							Definition:         permDenySlashPrefix,
							Action:             intentionActionDeny,
							Perm:               xdsPermSlashPrefix,
							NotPerms:           nil,
							Skip:               false,
							ComputedPermission: xdsPermSlashPrefix,
						},
					},
					Precedence:        9,
					Skip:              false,
					ComputedPrincipal: idPrincipal(nameWeb),
				},
				{
					Source: nameWild,
					NotSources: []rbacService{
						nameWeb,
					},
					Action:      intentionActionDeny,
					Permissions: nil,
					Precedence:  8,
					Skip:        false,
					ComputedPrincipal: andPrincipals(
						[]*envoy_rbac_v3.Principal{
							idPrincipal(nameWild),
							notPrincipal(
								idPrincipal(nameWeb),
							),
						},
					),
				},
			},
		},
		"default-deny-deny-all-and-path-deny": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionDeny}),
			),
			expect: []*rbacIntention{},
		},
		// ========================
		"default-allow-allow-all-and-path-allow": {
			intentionDefaultAllow: true,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionAllow}),
			),
			expect: []*rbacIntention{},
		},
		"default-deny-allow-all-and-path-allow": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionAllow}),
			),
			expect: []*rbacIntention{
				{
					Source: nameWeb,
					Action: intentionActionLayer7,
					Permissions: []*rbacPermission{
						{
							Definition:         permSlashPrefix,
							Action:             intentionActionAllow,
							Perm:               xdsPermSlashPrefix,
							NotPerms:           nil,
							Skip:               false,
							ComputedPermission: xdsPermSlashPrefix,
						},
					},
					Precedence:        9,
					Skip:              false,
					ComputedPrincipal: idPrincipal(nameWeb),
				},
				{
					Source: nameWild,
					NotSources: []rbacService{
						nameWeb,
					},
					Action:      intentionActionAllow,
					Permissions: nil,
					Precedence:  8,
					Skip:        false,
					ComputedPrincipal: andPrincipals(
						[]*envoy_rbac_v3.Principal{
							idPrincipal(nameWild),
							notPrincipal(
								idPrincipal(nameWeb),
							),
						},
					),
				},
			},
		},
		"default-allow-allow-all-and-path-deny": {
			intentionDefaultAllow: true,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionAllow}),
			),
			expect: []*rbacIntention{
				{
					Source: nameWeb,
					Action: intentionActionLayer7,
					Permissions: []*rbacPermission{
						{
							Definition:         permDenySlashPrefix,
							Action:             intentionActionDeny,
							Perm:               xdsPermSlashPrefix,
							NotPerms:           nil,
							Skip:               false,
							ComputedPermission: xdsPermSlashPrefix,
						},
					},
					Precedence:        9,
					Skip:              false,
					ComputedPrincipal: idPrincipal(nameWeb),
				},
			},
		},
		"default-deny-allow-all-and-path-deny": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
				testSourceIntention(ixnOpts{src: "*", action: structs.IntentionActionAllow}),
			),
			expect: []*rbacIntention{
				{
					Source: nameWild,
					NotSources: []rbacService{
						nameWeb,
					},
					Action:      intentionActionAllow,
					Permissions: nil,
					Precedence:  8,
					Skip:        false,
					ComputedPrincipal: andPrincipals(
						[]*envoy_rbac_v3.Principal{
							idPrincipal(nameWild),
							notPrincipal(
								idPrincipal(nameWeb),
							),
						},
					),
				},
			},
		},
		// ========= Sanity check that peers get passed through
		"default-deny-peered": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourceIntention(ixnOpts{
					src:    "*",
					action: structs.IntentionActionAllow,
					peer:   "peer1",
				}),
				testSourceIntention(ixnOpts{
					src:    "web",
					action: structs.IntentionActionAllow,
					peer:   "peer1",
				}),
			),
			expect: []*rbacIntention{
				{
					Source:            nameWebPeered,
					Action:            intentionActionAllow,
					Permissions:       nil,
					Precedence:        9,
					Skip:              false,
					ComputedPrincipal: idPrincipal(nameWebPeered),
				},
				{
					Source: nameWildPeered,
					Action: intentionActionAllow,
					NotSources: []rbacService{
						nameWebPeered,
					},
					Permissions: nil,
					Precedence:  8,
					Skip:        false,
					ComputedPrincipal: andPrincipals(
						[]*envoy_rbac_v3.Principal{
							idPrincipal(nameWildPeered),
							notPrincipal(
								idPrincipal(nameWebPeered),
							),
						},
					),
				},
			},
		},
	}

	testLocalInfo := rbacLocalInfo{
		trustDomain: testTrustDomain,
		datacenter:  "dc1",
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rbacIxns, err := intentionListToIntermediateRBACForm(tt.intentions, testLocalInfo, tt.http, testPeerTrustBundle, nil)
			intentionDefaultAction := intentionActionFromBool(tt.intentionDefaultAllow)
			rbacIxns = removeIntentionPrecedence(rbacIxns, intentionDefaultAction, testLocalInfo)

			require.NoError(t, err)
			require.Equal(t, tt.expect, rbacIxns)
		})
	}
}

func TestMakeRBACNetworkAndHTTPFilters(t *testing.T) {
	testIntention := func(t *testing.T, src, dst string, action structs.IntentionAction) *structs.Intention {
		t.Helper()
		ixn := structs.TestIntention(t)
		ixn.SourceName = src
		ixn.DestinationName = dst
		ixn.Action = action
		//nolint:staticcheck
		ixn.UpdatePrecedence()
		return ixn
	}
	testSourceIntention := func(src string, action structs.IntentionAction) *structs.Intention {
		return testIntention(t, src, "api", action)
	}
	testIntentionPeered := func(src string, peer string, action structs.IntentionAction) *structs.Intention {
		ixn := testIntention(t, src, "api", action)
		ixn.SourcePeer = peer
		return ixn
	}
	testSourcePermIntention := func(src string, perms ...*structs.IntentionPermission) *structs.Intention {
		ixn := testIntention(t, src, "api", "")
		ixn.Permissions = perms
		return ixn
	}
	testIntentionWithJWT := func(src string, action structs.IntentionAction, jwt *structs.IntentionJWTRequirement, perms ...*structs.IntentionPermission) *structs.Intention {
		ixn := testIntention(t, src, "api", action)
		ixn.JWT = jwt
		ixn.Action = action
		if perms != nil {
			ixn.Permissions = perms
			ixn.Action = ""
		}

		return ixn
	}
	testPeerTrustBundle := []*pbpeering.PeeringTrustBundle{
		{
			PeerName:          "peer1",
			TrustDomain:       "peer1.domain",
			ExportedPartition: "part1",
		},
	}
	testTrustDomain := "test.consul"
	sorted := func(ixns ...*structs.Intention) structs.SimplifiedIntentions {
		sort.SliceStable(ixns, func(i, j int) bool {
			return ixns[j].Precedence < ixns[i].Precedence
		})
		return structs.SimplifiedIntentions(ixns)
	}

	var (
		permSlashPrefix = &structs.IntentionPermission{
			Action: structs.IntentionActionAllow,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/",
			},
		}
		oktaWithClaims = structs.IntentionJWTProvider{
			Name: "okta",
			VerifyClaims: []*structs.IntentionJWTClaimVerification{
				{Path: []string{"roles"}, Value: "testing"},
			},
		}
		auth0WithClaims = structs.IntentionJWTProvider{
			Name: "auth0",
			VerifyClaims: []*structs.IntentionJWTClaimVerification{
				{Path: []string{"perms", "role"}, Value: "admin"},
			},
		}
		testJWTProviderConfigEntry = map[string]*structs.JWTProviderConfigEntry{
			"okta":  {Name: "okta", Issuer: "mytest.okta-issuer"},
			"auth0": {Name: "auth0", Issuer: "mytest.auth0-issuer"},
		}
		jwtRequirement = &structs.IntentionJWTRequirement{
			Providers: []*structs.IntentionJWTProvider{
				&oktaWithClaims,
			},
		}
		auth0Requirement = &structs.IntentionJWTRequirement{
			Providers: []*structs.IntentionJWTProvider{
				&auth0WithClaims,
			},
		}
		permDenySlashPrefix = &structs.IntentionPermission{
			Action: structs.IntentionActionDeny,
			HTTP: &structs.IntentionHTTPPermission{
				PathPrefix: "/",
			},
		}
	)

	tests := map[string]struct {
		intentionDefaultAllow bool
		intentions            structs.SimplifiedIntentions
	}{
		"default-deny-mixed-precedence": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testIntention(t, "web", "api", structs.IntentionActionAllow),
				testIntention(t, "*", "api", structs.IntentionActionDeny),
				testIntention(t, "web", "*", structs.IntentionActionDeny),
			),
		},
		"default-deny-service-wildcard-allow": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourceIntention("*", structs.IntentionActionAllow),
			),
		},
		"default-allow-service-wildcard-deny": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourceIntention("*", structs.IntentionActionDeny),
			),
		},
		"default-deny-one-allow": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourceIntention("web", structs.IntentionActionAllow),
			),
		},
		"default-allow-one-deny": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourceIntention("web", structs.IntentionActionDeny),
			),
		},
		"default-deny-allow-deny": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourceIntention("web", structs.IntentionActionDeny),
				testSourceIntention("*", structs.IntentionActionAllow),
			),
		},
		"default-deny-kitchen-sink": {
			intentionDefaultAllow: false,
			intentions: sorted(
				// (double exact)
				testSourceIntention("web", structs.IntentionActionAllow),
				testSourceIntention("unsafe", structs.IntentionActionDeny),
				testSourceIntention("cron", structs.IntentionActionAllow),
				testSourceIntention("*", structs.IntentionActionAllow),
			),
		},
		"default-allow-kitchen-sink": {
			intentionDefaultAllow: true,
			intentions: sorted(
				// (double exact)
				testSourceIntention("web", structs.IntentionActionDeny),
				testSourceIntention("unsafe", structs.IntentionActionAllow),
				testSourceIntention("cron", structs.IntentionActionDeny),
				testSourceIntention("*", structs.IntentionActionDeny),
			),
		},
		"default-deny-peered-kitchen-sink": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourceIntention("web", structs.IntentionActionAllow),
				testIntentionPeered("*", "peer1", structs.IntentionActionAllow),
				testIntentionPeered("web", "peer1", structs.IntentionActionDeny),
			),
		},
		// ========================
		"default-allow-path-allow": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
			),
		},
		"default-deny-path-allow": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
			),
		},
		"default-allow-path-deny": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
			),
		},
		"default-deny-path-deny": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourcePermIntention("web", permDenySlashPrefix),
			),
		},
		// ========================
		"default-allow-deny-all-and-path-allow": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/",
						},
					},
				),
				testSourceIntention("*", structs.IntentionActionDeny),
			),
		},
		"default-deny-deny-all-and-path-allow": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/",
						},
					},
				),
				testSourceIntention("*", structs.IntentionActionDeny),
			),
		},
		"default-allow-deny-all-and-path-deny": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/",
						},
					},
				),
				testSourceIntention("*", structs.IntentionActionDeny),
			),
		},
		"default-deny-deny-all-and-path-deny": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/",
						},
					},
				),
				testSourceIntention("*", structs.IntentionActionDeny),
			),
		},
		// ========================
		"default-deny-two-path-deny-and-path-allow": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/secret",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/admin",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/",
						},
					},
				),
			),
		},
		"default-allow-two-path-deny-and-path-allow": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/secret",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/admin",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/",
						},
					},
				),
			),
		},
		"default-deny-single-intention-with-kitchen-sink-perms": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/secret",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/v1",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathRegex: "/v[123]",
							Methods:   []string{"GET", "HEAD", "OPTIONS"},
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							Header: []structs.IntentionHTTPHeaderPermission{
								{Name: "x-foo", Present: true},
								{Name: "x-bar", Exact: "xyz"},
								{Name: "x-dib", Prefix: "gaz"},
								{Name: "x-gir", Suffix: "zim"},
								{Name: "x-zim", Regex: "gi[rR]"},
								{Name: "z-foo", Present: true, Invert: true},
								{Name: "z-bar", Exact: "xyz", Invert: true},
								{Name: "z-dib", Prefix: "gaz", Invert: true},
								{Name: "z-gir", Suffix: "zim", Invert: true},
								{Name: "z-zim", Regex: "gi[rR]", Invert: true},
							},
						},
					},
				),
			),
		},
		"default-allow-single-intention-with-kitchen-sink-perms": {
			intentionDefaultAllow: true,
			intentions: sorted(
				testSourcePermIntention("web",
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/secret",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathPrefix: "/v1",
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							PathRegex: "/v[123]",
							Methods:   []string{"GET", "HEAD", "OPTIONS"},
						},
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionDeny,
						HTTP: &structs.IntentionHTTPPermission{
							Header: []structs.IntentionHTTPHeaderPermission{
								{Name: "x-foo", Present: true},
								{Name: "x-bar", Exact: "xyz"},
								{Name: "x-dib", Prefix: "gaz"},
								{Name: "x-gir", Suffix: "zim"},
								{Name: "x-zim", Regex: "gi[rR]"},
								{Name: "z-foo", Present: true, Invert: true},
								{Name: "z-bar", Exact: "xyz", Invert: true},
								{Name: "z-dib", Prefix: "gaz", Invert: true},
								{Name: "z-gir", Suffix: "zim", Invert: true},
								{Name: "z-zim", Regex: "gi[rR]", Invert: true},
							},
						},
					},
				),
			),
		},
		// ========= JWTAuthn Filter checks
		"top-level-jwt-no-permissions": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testIntentionWithJWT("web", structs.IntentionActionAllow, jwtRequirement),
			),
		},
		"empty-top-level-jwt-with-one-permission": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testIntentionWithJWT("web", structs.IntentionActionAllow, nil, &structs.IntentionPermission{
					Action: structs.IntentionActionAllow,
					HTTP: &structs.IntentionHTTPPermission{
						PathPrefix: "some-path",
					},
					JWT: jwtRequirement,
				}),
			),
		},
		"top-level-jwt-with-one-permission": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testIntentionWithJWT("web",
					structs.IntentionActionAllow,
					jwtRequirement,
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/secret",
						},
						JWT: auth0Requirement,
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/admin",
						},
					},
				),
			),
		},
		"top-level-jwt-with-multiple-permissions": {
			intentionDefaultAllow: false,
			intentions: sorted(
				testIntentionWithJWT("web",
					structs.IntentionActionAllow,
					jwtRequirement,
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/secret",
						},
						JWT: auth0Requirement,
					},
					&structs.IntentionPermission{
						Action: structs.IntentionActionAllow,
						HTTP: &structs.IntentionHTTPPermission{
							PathExact: "/v1/admin",
						},
						JWT: auth0Requirement,
					},
				),
			),
		},
	}

	testLocalInfo := rbacLocalInfo{
		trustDomain: testTrustDomain,
		datacenter:  "dc1",
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Run("network filter", func(t *testing.T) {
				filter, err := makeRBACNetworkFilter(tt.intentions, tt.intentionDefaultAllow, testLocalInfo, testPeerTrustBundle)
				require.NoError(t, err)

				t.Run("current", func(t *testing.T) {
					gotJSON := protoToJSON(t, filter)

					require.JSONEq(t, goldenSimple(t, filepath.Join("rbac", name), gotJSON), gotJSON)
				})
			})
			t.Run("http filter", func(t *testing.T) {
				filter, err := makeRBACHTTPFilter(tt.intentions, tt.intentionDefaultAllow, testLocalInfo, testPeerTrustBundle, testJWTProviderConfigEntry)
				require.NoError(t, err)

				t.Run("current", func(t *testing.T) {
					gotJSON := protoToJSON(t, filter)

					require.JSONEq(t, goldenSimple(t, filepath.Join("rbac", name+"--httpfilter"), gotJSON), gotJSON)
				})
			})
		})
	}
}

func TestRemoveSameSourceIntentions(t *testing.T) {
	testIntention := func(t *testing.T, src, dst string) *structs.Intention {
		t.Helper()
		ixn := structs.TestIntention(t)
		ixn.SourceName = src
		ixn.DestinationName = dst
		//nolint:staticcheck
		ixn.UpdatePrecedence()
		return ixn
	}
	testIntentionPeered := func(t *testing.T, src, dst, peer string) *structs.Intention {
		t.Helper()
		ixn := structs.TestIntention(t)
		ixn.SourceName = src
		ixn.SourcePeer = peer
		ixn.DestinationName = dst
		//nolint:staticcheck
		ixn.UpdatePrecedence()
		return ixn
	}
	sorted := func(ixns ...*structs.Intention) structs.SimplifiedIntentions {
		sort.SliceStable(ixns, func(i, j int) bool {
			return ixns[j].Precedence < ixns[i].Precedence
		})
		return structs.SimplifiedIntentions(ixns)
	}
	tests := map[string]struct {
		in     structs.SimplifiedIntentions
		expect structs.SimplifiedIntentions
	}{
		"empty": {},
		"one": {
			in: sorted(
				testIntention(t, "*", "*"),
			),
			expect: sorted(
				testIntention(t, "*", "*"),
			),
		},
		"two with no match": {
			in: sorted(
				testIntention(t, "*", "foo"),
				testIntention(t, "bar", "*"),
			),
			expect: sorted(
				testIntention(t, "*", "foo"),
				testIntention(t, "bar", "*"),
			),
		},
		"two with match, exact": {
			in: sorted(
				testIntention(t, "bar", "foo"),
				testIntention(t, "bar", "*"),
			),
			expect: sorted(
				testIntention(t, "bar", "foo"),
			),
		},
		"two with match, wildcard": {
			in: sorted(
				testIntention(t, "*", "foo"),
				testIntention(t, "*", "*"),
			),
			expect: sorted(
				testIntention(t, "*", "foo"),
			),
		},
		"kitchen sink with peers": {
			in: sorted(
				testIntention(t, "bar", "foo"),
				testIntentionPeered(t, "bar", "foo", "peer1"),
				testIntentionPeered(t, "bar", "*", "peer1"),
				testIntentionPeered(t, "*", "foo", "peer1"),
				testIntentionPeered(t, "*", "*", "peer1"),
			),
			expect: sorted(
				testIntention(t, "bar", "foo"),
				testIntentionPeered(t, "bar", "foo", "peer1"),
				testIntentionPeered(t, "*", "foo", "peer1"),
			),
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got := removeSameSourceIntentions(tc.in)
			require.Equal(t, tc.expect, got)
		})
	}
}

func TestSimplifyNotSourceSlice(t *testing.T) {
	tests := map[string]struct {
		in     []string
		expect []string
	}{
		"empty": {},
		"one": {
			[]string{"bar"},
			[]string{"bar"},
		},
		"two with no match": {
			[]string{"foo", "bar"},
			[]string{"foo", "bar"},
		},
		"two with match": {
			[]string{"*", "bar"},
			[]string{"*"},
		},
		"three with two matches down to one": {
			[]string{"*", "foo", "bar"},
			[]string{"*"},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			got := simplifyNotSourceSlice(makeServiceNameSlice(tc.in))
			require.Equal(t, makeServiceNameSlice(tc.expect), got)
		})
	}
}

func TestIxnSourceMatches(t *testing.T) {
	tests := []struct {
		tester      string
		testerPeer  string
		against     string
		againstPeer string
		matches     bool
	}{
		// identical precedence
		{"web", "", "api", "", false},
		{"*", "", "*", "", false},
		// backwards precedence
		{"*", "", "web", "", false},
		// name wildcards
		{"web", "", "*", "", true},

		// peered cmp peered
		{"web", "peer1", "api", "peer1", false},
		{"*", "peer1", "*", "peer1", false},
		// no match if peer is different
		{"web", "peer1", "web", "", false},
		{"*", "peer1", "*", "peer2", false},
		// name wildcards with peer
		{"web", "peer1", "*", "peer1", true},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s%s cmp %s%s", tc.testerPeer, tc.tester, tc.againstPeer, tc.against), func(t *testing.T) {
			matches := ixnSourceMatches(
				rbacService{ServiceName: structs.ServiceNameFromString(tc.tester), Peer: tc.testerPeer},
				rbacService{ServiceName: structs.ServiceNameFromString(tc.against), Peer: tc.againstPeer},
			)
			assert.Equal(t, tc.matches, matches)
		})
	}
}

func makeServiceNameSlice(slice []string) []rbacService {
	if len(slice) == 0 {
		return nil
	}
	var out []rbacService
	for _, src := range slice {
		out = append(out, rbacService{ServiceName: structs.ServiceNameFromString(src)})
	}
	return out
}

func TestSpiffeMatcher(t *testing.T) {
	cases := map[string]struct {
		xfcc        string
		trustDomain string
		namespace   string
		partition   string
		datacenter  string
		service     string
	}{
		"between admin partitions": {
			xfcc:        `By=spiffe://70c72965-291c-d138-e5a6-cfd8a66b395e.consul/ap/ap1/ns/default/dc/primary/svc/s2;Hash=377330adafa619abe52672246b7be7410d74b7497e9d88a8396d641fd6f82ad2;Cert="-----BEGIN%20CERTIFICATE-----%0AMIICGTCCAb%2BgAwIBAgIBCzAKBggqhkjOPQQDAjAwMS4wLAYDVQQDEyVwcmktMTJj%0AOWtvbS5jb25zdWwuY2EuNzBjNzI5NjUuY29uc3VsMB4XDTIyMTIyMjE0MjE1NVoX%0ADTIyMTIyNTE0MjE1NVowADBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPuJbVdQ%0AYsT8RnvMLT%2FpsuZwltWbCkwxzBR03%2FEC4f7TyLy1Mfe6gm%2Fz5K8Tc29d7W16PBT0%0AR%2B1XPfpigopVanyjgfkwgfYwDgYDVR0PAQH%2FBAQDAgO4MB0GA1UdJQQWMBQGCCsG%0AAQUFBwMCBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMCkGA1UdDgQiBCBBxpy1QXfp%0AS4V8QFH%2BEfF39VP51Qbhlj75N5gbUSxGajArBgNVHSMEJDAigCCjWP%2BlGhzd4jbD%0A2QI66cvAAgIkLqG0lz0PyzTz76QoOzBfBgNVHREBAf8EVTBThlFzcGlmZmU6Ly83%0AMGM3Mjk2NS0yOTFjLWQxMzgtZTVhNi1jZmQ4YTY2YjM5NWUuY29uc3VsL25zL2Rl%0AZmF1bHQvZGMvcHJpbWFyeS9zdmMvczEwCgYIKoZIzj0EAwIDSAAwRQIhAJxWHplX%0Aqgmd4cRDMllJsCtOmTZ3v%2B6qDnc545tm%2Bg%2FzAiBwWOqqTZ81BtAtzzWpip1XmUFR%0Afv2SYupWQueXYrOjhw%3D%3D%0A-----END%20CERTIFICATE-----%0A";Chain="-----BEGIN%20CERTIFICATE-----%0AMIICGTCCAb%2BgAwIBAgIBCzAKBggqhkjOPQQDAjAwMS4wLAYDVQQDEyVwcmktMTJj%0AOWtvbS5jb25zdWwuY2EuNzBjNzI5NjUuY29uc3VsMB4XDTIyMTIyMjE0MjE1NVoX%0ADTIyMTIyNTE0MjE1NVowADBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPuJbVdQ%0AYsT8RnvMLT%2FpsuZwltWbCkwxzBR03%2FEC4f7TyLy1Mfe6gm%2Fz5K8Tc29d7W16PBT0%0AR%2B1XPfpigopVanyjgfkwgfYwDgYDVR0PAQH%2FBAQDAgO4MB0GA1UdJQQWMBQGCCsG%0AAQUFBwMCBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMCkGA1UdDgQiBCBBxpy1QXfp%0AS4V8QFH%2BEfF39VP51Qbhlj75N5gbUSxGajArBgNVHSMEJDAigCCjWP%2BlGhzd4jbD%0A2QI66cvAAgIkLqG0lz0PyzTz76QoOzBfBgNVHREBAf8EVTBThlFzcGlmZmU6Ly83%0AMGM3Mjk2NS0yOTFjLWQxMzgtZTVhNi1jZmQ4YTY2YjM5NWUuY29uc3VsL25zL2Rl%0AZmF1bHQvZGMvcHJpbWFyeS9zdmMvczEwCgYIKoZIzj0EAwIDSAAwRQIhAJxWHplX%0Aqgmd4cRDMllJsCtOmTZ3v%2B6qDnc545tm%2Bg%2FzAiBwWOqqTZ81BtAtzzWpip1XmUFR%0Afv2SYupWQueXYrOjhw%3D%3D%0A-----END%20CERTIFICATE-----%0A";Subject="";URI=spiffe://70c72965-291c-d138-e5a6-cfd8a66b395e.consul/ap/ap9/ns/default/dc/primary/svc/s1`,
			trustDomain: "70c72965-291c-d138-e5a6-cfd8a66b395e.consul",
			namespace:   "default",
			partition:   "ap9",
			datacenter:  "primary",
			service:     "s1",
		},
		"between services": {
			xfcc:        `By=spiffe://f1efe25e-a9b1-1ae1-b580-98000b84a935.consul/ns/default/dc/primary/svc/s2;Hash=c552ee3990fd6e9bb38b1a8bdd28e8358c339d282e6bb92fc86d04915407f47d;Cert="-----BEGIN%20CERTIFICATE-----%0AMIICGjCCAcCgAwIBAgIBCzAKBggqhkjOPQQDAjAxMS8wLQYDVQQDEyZwcmktOGFt%0AMjNueXouY29uc3VsLmNhLmYxZWZlMjVlLmNvbnN1bDAeFw0yMjEyMjIxNTIxMDVa%0AFw0yMjEyMjUxNTIxMDVaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAASrChLh%0AelrBB5e8X78fSvbKxD8yieadFg4XUeJtZh2xwdWckCGDEtT984ihgM8Hu4E%2FGpgD%0AJcExohFnS4H%2BG3uco4H5MIH2MA4GA1UdDwEB%2FwQEAwIDuDAdBgNVHSUEFjAUBggr%0ABgEFBQcDAgYIKwYBBQUHAwEwDAYDVR0TAQH%2FBAIwADApBgNVHQ4EIgQghpyuV%2F4g%0Ac6x%2B5jC9uOZQMY4Km2YZwAnSmmTydjjn7qwwKwYDVR0jBCQwIoAgdO0jdTJzfKYq%0ARCYrWbHr7q%2Bq66ispOnMs6HsEwlxV%2F8wXwYDVR0RAQH%2FBFUwU4ZRc3BpZmZlOi8v%0AZjFlZmUyNWUtYTliMS0xYWUxLWI1ODAtOTgwMDBiODRhOTM1LmNvbnN1bC9ucy9k%0AZWZhdWx0L2RjL3ByaW1hcnkvc3ZjL3MxMAoGCCqGSM49BAMCA0gAMEUCIQDTNsze%0AXCj16YvFsX0PUeUBcX4Hh0nmIkMOHCQiPkXTiAIgKJKf038s6muFJw9UQJJ5SSg%2F%0A3RL1wIWXRhsqy1Y89JQ%3D%0A-----END%20CERTIFICATE-----%0A";Chain="-----BEGIN%20CERTIFICATE-----%0AMIICGjCCAcCgAwIBAgIBCzAKBggqhkjOPQQDAjAxMS8wLQYDVQQDEyZwcmktOGFt%0AMjNueXouY29uc3VsLmNhLmYxZWZlMjVlLmNvbnN1bDAeFw0yMjEyMjIxNTIxMDVa%0AFw0yMjEyMjUxNTIxMDVaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAASrChLh%0AelrBB5e8X78fSvbKxD8yieadFg4XUeJtZh2xwdWckCGDEtT984ihgM8Hu4E%2FGpgD%0AJcExohFnS4H%2BG3uco4H5MIH2MA4GA1UdDwEB%2FwQEAwIDuDAdBgNVHSUEFjAUBggr%0ABgEFBQcDAgYIKwYBBQUHAwEwDAYDVR0TAQH%2FBAIwADApBgNVHQ4EIgQghpyuV%2F4g%0Ac6x%2B5jC9uOZQMY4Km2YZwAnSmmTydjjn7qwwKwYDVR0jBCQwIoAgdO0jdTJzfKYq%0ARCYrWbHr7q%2Bq66ispOnMs6HsEwlxV%2F8wXwYDVR0RAQH%2FBFUwU4ZRc3BpZmZlOi8v%0AZjFlZmUyNWUtYTliMS0xYWUxLWI1ODAtOTgwMDBiODRhOTM1LmNvbnN1bC9ucy9k%0AZWZhdWx0L2RjL3ByaW1hcnkvc3ZjL3MxMAoGCCqGSM49BAMCA0gAMEUCIQDTNsze%0AXCj16YvFsX0PUeUBcX4Hh0nmIkMOHCQiPkXTiAIgKJKf038s6muFJw9UQJJ5SSg%2F%0A3RL1wIWXRhsqy1Y89JQ%3D%0A-----END%20CERTIFICATE-----%0A";Subject="";URI=spiffe://f1efe25e-a9b1-1ae1-b580-98000b84a935.consul/ns/default/dc/primary/svc/s1`,
			trustDomain: "f1efe25e-a9b1-1ae1-b580-98000b84a935.consul",
			namespace:   "default",
			datacenter:  "primary",
			service:     "s1",
		},
		"between peers": {
			xfcc:        `By=spiffe://ca9857da-71aa-c5be-ec8f-abcd90cae693.consul/gateway/mesh/dc/alpha;Hash=419c850ddc7a32edc752d73bb0f0c6e4c2f5b40feae7cf0cdeeb6f3dd759ed1f;Cert="-----BEGIN%20CERTIFICATE-----%0AMIICGzCCAcCgAwIBAgIBCzAKBggqhkjOPQQDAjAxMS8wLQYDVQQDEyZwcmktcTgw%0AdmcxMXQuY29uc3VsLmNhLmZjOWEwOGVmLmNvbnN1bDAeFw0yMjEyMjIxNTIyNTBa%0AFw0yMjEyMjUxNTIyNTBaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQnQtQ6%0AFS%2FqjpopxZIaJtYL3pOx%2BgrzoLtKStCS0SUtGbTBmxmTeIX5l5HHD4yqCWk4M1Iv%0AXNflWvKcpw5KS1tLo4H5MIH2MA4GA1UdDwEB%2FwQEAwIDuDAdBgNVHSUEFjAUBggr%0ABgEFBQcDAgYIKwYBBQUHAwEwDAYDVR0TAQH%2FBAIwADApBgNVHQ4EIgQg%2B8FyVm2p%0AdpzfijuCYeByJQH5mUkqY6%2FciCC2yScNusQwKwYDVR0jBCQwIoAgy0MyubT%2BMNQv%0A%2BuZGeBqa1yU9Fx9641epfbY%2BuSs7cbowXwYDVR0RAQH%2FBFUwU4ZRc3BpZmZlOi8v%0AZmM5YTA4ZWYtZWZiNC1iYmM5LWIzZWMtYjkzZTc2OGFiZmMyLmNvbnN1bC9ucy9k%0AZWZhdWx0L2RjL3ByaW1hcnkvc3ZjL3MxMAoGCCqGSM49BAMCA0kAMEYCIQDp7hX0%0AJ%2FjrAP71jDt2w3uKQJnfZ93d%2FRub2t%2FRwQfsVAIhAL4VUbk5XUvBzwabuEfMCf4O%0AT5rjXDbCWYNN2m4xZFtt%0A-----END%20CERTIFICATE-----%0A";Chain="-----BEGIN%20CERTIFICATE-----%0AMIICGzCCAcCgAwIBAgIBCzAKBggqhkjOPQQDAjAxMS8wLQYDVQQDEyZwcmktcTgw%0AdmcxMXQuY29uc3VsLmNhLmZjOWEwOGVmLmNvbnN1bDAeFw0yMjEyMjIxNTIyNTBa%0AFw0yMjEyMjUxNTIyNTBaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQnQtQ6%0AFS%2FqjpopxZIaJtYL3pOx%2BgrzoLtKStCS0SUtGbTBmxmTeIX5l5HHD4yqCWk4M1Iv%0AXNflWvKcpw5KS1tLo4H5MIH2MA4GA1UdDwEB%2FwQEAwIDuDAdBgNVHSUEFjAUBggr%0ABgEFBQcDAgYIKwYBBQUHAwEwDAYDVR0TAQH%2FBAIwADApBgNVHQ4EIgQg%2B8FyVm2p%0AdpzfijuCYeByJQH5mUkqY6%2FciCC2yScNusQwKwYDVR0jBCQwIoAgy0MyubT%2BMNQv%0A%2BuZGeBqa1yU9Fx9641epfbY%2BuSs7cbowXwYDVR0RAQH%2FBFUwU4ZRc3BpZmZlOi8v%0AZmM5YTA4ZWYtZWZiNC1iYmM5LWIzZWMtYjkzZTc2OGFiZmMyLmNvbnN1bC9ucy9k%0AZWZhdWx0L2RjL3ByaW1hcnkvc3ZjL3MxMAoGCCqGSM49BAMCA0kAMEYCIQDp7hX0%0AJ%2FjrAP71jDt2w3uKQJnfZ93d%2FRub2t%2FRwQfsVAIhAL4VUbk5XUvBzwabuEfMCf4O%0AT5rjXDbCWYNN2m4xZFtt%0A-----END%20CERTIFICATE-----%0A";Subject="";URI=spiffe://fc9a08ef-efb4-bbc9-b3ec-b93e768abfc2.consul/ns/default/dc/primary/svc/s1,By=spiffe://ca9857da-71aa-c5be-ec8f-abcd90cae693.consul/ns/default/dc/alpha/svc/s2;Hash=1db4ea1e68df1ea0cec7d7ba882ca734d3e1a29a0fe64e73275b6ab796234295;Cert="-----BEGIN%20CERTIFICATE-----%0AMIICEjCCAbmgAwIBAgIBDDAKBggqhkjOPQQDAjAxMS8wLQYDVQQDEyZwcmktMXky%0AZXVpbHkuY29uc3VsLmNhLmNhOTg1N2RhLmNvbnN1bDAeFw0yMjEyMjIxNTIzMDVa%0AFw0yMjEyMjUxNTIzMDVaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAROaLaT%0A%2BzyYZKfujWX4vOde%2BnnsGP3z0xaEGQFbgi%2BGU%2BrFfMdadzYF1oXDItS%2FpuBADuha%0Ao0iH2i2aRPUbTm4Ko4HyMIHvMA4GA1UdDwEB%2FwQEAwIDuDAdBgNVHSUEFjAUBggr%0ABgEFBQcDAgYIKwYBBQUHAwEwDAYDVR0TAQH%2FBAIwADApBgNVHQ4EIgQgWTznn%2BPz%0A4eNoiwdO%2FID3uqbyiBJBFbZFAGs7m5KnoCkwKwYDVR0jBCQwIoAgAdVe5N4m4Qlv%0Afgp9tvw0MGq7puWWuLfiw7qghdr1VDIwWAYDVR0RAQH%2FBE4wTIZKc3BpZmZlOi8v%0AY2E5ODU3ZGEtNzFhYS1jNWJlLWVjOGYtYWJjZDkwY2FlNjkzLmNvbnN1bC9nYXRl%0Ad2F5L21lc2gvZGMvYWxwaGEwCgYIKoZIzj0EAwIDRwAwRAIgJu5Z6O10nQe9HAzk%0ARonRMODgENawDHbErpkQ1q91ZTYCIEHccGIEp3OybkvkmIB9s%2Bu%2FbguUjJ4ZKAiD%0AV0dKf1Ao%0A-----END%20CERTIFICATE-----%0A";Chain="-----BEGIN%20CERTIFICATE-----%0AMIICEjCCAbmgAwIBAgIBDDAKBggqhkjOPQQDAjAxMS8wLQYDVQQDEyZwcmktMXky%0AZXVpbHkuY29uc3VsLmNhLmNhOTg1N2RhLmNvbnN1bDAeFw0yMjEyMjIxNTIzMDVa%0AFw0yMjEyMjUxNTIzMDVaMAAwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAROaLaT%0A%2BzyYZKfujWX4vOde%2BnnsGP3z0xaEGQFbgi%2BGU%2BrFfMdadzYF1oXDItS%2FpuBADuha%0Ao0iH2i2aRPUbTm4Ko4HyMIHvMA4GA1UdDwEB%2FwQEAwIDuDAdBgNVHSUEFjAUBggr%0ABgEFBQcDAgYIKwYBBQUHAwEwDAYDVR0TAQH%2FBAIwADApBgNVHQ4EIgQgWTznn%2BPz%0A4eNoiwdO%2FID3uqbyiBJBFbZFAGs7m5KnoCkwKwYDVR0jBCQwIoAgAdVe5N4m4Qlv%0Afgp9tvw0MGq7puWWuLfiw7qghdr1VDIwWAYDVR0RAQH%2FBE4wTIZKc3BpZmZlOi8v%0AY2E5ODU3ZGEtNzFhYS1jNWJlLWVjOGYtYWJjZDkwY2FlNjkzLmNvbnN1bC9nYXRl%0Ad2F5L21lc2gvZGMvYWxwaGEwCgYIKoZIzj0EAwIDRwAwRAIgJu5Z6O10nQe9HAzk%0ARonRMODgENawDHbErpkQ1q91ZTYCIEHccGIEp3OybkvkmIB9s%2Bu%2FbguUjJ4ZKAiD%0AV0dKf1Ao%0A-----END%20CERTIFICATE-----%0A";Subject="";URI=spiffe://ca9857da-71aa-c5be-ec8f-abcd90cae693.consul/gateway/mesh/dc/alpha`,
			trustDomain: "fc9a08ef-efb4-bbc9-b3ec-b93e768abfc2.consul",
			namespace:   "default",
			datacenter:  "primary",
			service:     "s1",
		},
	}

	re := regexp.MustCompile(downstreamServiceIdentityMatcher)

	for n, c := range cases {
		t.Run(n, func(t *testing.T) {
			matches := re.FindAllStringSubmatch(c.xfcc, -1)
			require.Len(t, matches, 1)

			m := matches[0]
			require.Equal(t, c.trustDomain, m[1])
			require.Equal(t, c.partition, m[2])
			require.Equal(t, c.namespace, m[3])
			require.Equal(t, c.datacenter, m[4])
			require.Equal(t, c.service, m[5])
		})
	}
}

func TestPathToSegments(t *testing.T) {
	tests := map[string]struct {
		key      string
		paths    []string
		expected []*envoy_matcher_v3.MetadataMatcher_PathSegment
	}{
		"single-path": {
			key:   "jwt_payload_okta",
			paths: []string{"perms"},
			expected: []*envoy_matcher_v3.MetadataMatcher_PathSegment{
				{
					Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "jwt_payload_okta"},
				},
				{
					Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "perms"},
				},
			},
		},
		"multi-paths": {
			key:   "jwt_payload_okta",
			paths: []string{"perms", "roles"},
			expected: []*envoy_matcher_v3.MetadataMatcher_PathSegment{
				{
					Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "jwt_payload_okta"},
				},
				{
					Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "perms"},
				},
				{
					Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "roles"},
				},
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			segments := pathToSegments(tt.paths, tt.key)
			require.ElementsMatch(t, segments, tt.expected)
		})
	}
}

func TestJWTClaimsToPrincipals(t *testing.T) {
	var (
		firstClaim = structs.IntentionJWTClaimVerification{
			Path:  []string{"perms"},
			Value: "admin",
		}
		secondClaim = structs.IntentionJWTClaimVerification{
			Path:  []string{"passage"},
			Value: "secret",
		}
		payloadKey     = "dummy-key"
		firstPrincipal = envoy_rbac_v3.Principal{
			Identifier: &envoy_rbac_v3.Principal_Metadata{
				Metadata: &envoy_matcher_v3.MetadataMatcher{
					Filter: jwtEnvoyFilter,
					Path:   pathToSegments(firstClaim.Path, payloadKey),
					Value: &envoy_matcher_v3.ValueMatcher{
						MatchPattern: &envoy_matcher_v3.ValueMatcher_StringMatch{
							StringMatch: &envoy_matcher_v3.StringMatcher{
								MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
									Exact: firstClaim.Value,
								},
							},
						},
					},
				},
			},
		}
		secondPrincipal = envoy_rbac_v3.Principal{
			Identifier: &envoy_rbac_v3.Principal_Metadata{
				Metadata: &envoy_matcher_v3.MetadataMatcher{
					Filter: jwtEnvoyFilter,
					Path:   pathToSegments(secondClaim.Path, payloadKey),
					Value: &envoy_matcher_v3.ValueMatcher{
						MatchPattern: &envoy_matcher_v3.ValueMatcher_StringMatch{
							StringMatch: &envoy_matcher_v3.StringMatcher{
								MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
									Exact: secondClaim.Value,
								},
							},
						},
					},
				},
			},
		}
	)
	tests := map[string]struct {
		claims             []*structs.IntentionJWTClaimVerification
		metadataPayloadKey string
		expected           *envoy_rbac_v3.Principal
	}{
		"single-claim": {
			claims:             []*structs.IntentionJWTClaimVerification{&firstClaim},
			metadataPayloadKey: payloadKey,
			expected:           &firstPrincipal,
		},
		"multiple-claims": {
			claims:             []*structs.IntentionJWTClaimVerification{&firstClaim, &secondClaim},
			metadataPayloadKey: payloadKey,
			expected: &envoy_rbac_v3.Principal{
				Identifier: &envoy_rbac_v3.Principal_AndIds{
					AndIds: &envoy_rbac_v3.Principal_Set{
						Ids: []*envoy_rbac_v3.Principal{&firstPrincipal, &secondPrincipal},
					},
				},
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			principal := jwtClaimsToPrincipals(tt.claims, tt.metadataPayloadKey)
			require.Equal(t, principal, tt.expected)
		})
	}
}

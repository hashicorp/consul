package xds

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	envoy_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestRemoveIntentionPrecedence(t *testing.T) {
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
	testSourcePermIntention := func(src string, perms ...*structs.IntentionPermission) *structs.Intention {
		ixn := testIntention(t, src, "api", "")
		ixn.Permissions = perms
		return ixn
	}
	sorted := func(ixns ...*structs.Intention) structs.Intentions {
		sort.SliceStable(ixns, func(i, j int) bool {
			return ixns[j].Precedence < ixns[i].Precedence
		})
		return structs.Intentions(ixns)
	}

	var (
		nameWild        = structs.NewServiceName("*", nil)
		nameWeb         = structs.NewServiceName("web", nil)
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
		intentions            structs.Intentions
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
				testSourceIntention("*", structs.IntentionActionDeny),
			),
			expect: []*rbacIntention{
				{
					Source: nameWild,
					NotSources: []structs.ServiceName{
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
				testSourceIntention("*", structs.IntentionActionDeny),
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
				testSourceIntention("*", structs.IntentionActionDeny),
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
					NotSources: []structs.ServiceName{
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
				testSourceIntention("*", structs.IntentionActionDeny),
			),
			expect: []*rbacIntention{},
		},
		// ========================
		"default-allow-allow-all-and-path-allow": {
			intentionDefaultAllow: true,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
				testSourceIntention("*", structs.IntentionActionAllow),
			),
			expect: []*rbacIntention{},
		},
		"default-deny-allow-all-and-path-allow": {
			intentionDefaultAllow: false,
			http:                  true,
			intentions: sorted(
				testSourcePermIntention("web", permSlashPrefix),
				testSourceIntention("*", structs.IntentionActionAllow),
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
					NotSources: []structs.ServiceName{
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
				testSourceIntention("*", structs.IntentionActionAllow),
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
				testSourceIntention("*", structs.IntentionActionAllow),
			),
			expect: []*rbacIntention{
				{
					Source: nameWild,
					NotSources: []structs.ServiceName{
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
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rbacIxns := intentionListToIntermediateRBACForm(tt.intentions, tt.http)
			intentionDefaultAction := intentionActionFromBool(tt.intentionDefaultAllow)
			rbacIxns = removeIntentionPrecedence(rbacIxns, intentionDefaultAction)

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
	testSourcePermIntention := func(src string, perms ...*structs.IntentionPermission) *structs.Intention {
		ixn := testIntention(t, src, "api", "")
		ixn.Permissions = perms
		return ixn
	}
	sorted := func(ixns ...*structs.Intention) structs.Intentions {
		sort.SliceStable(ixns, func(i, j int) bool {
			return ixns[j].Precedence < ixns[i].Precedence
		})
		return structs.Intentions(ixns)
	}

	var (
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
	)

	tests := map[string]struct {
		intentionDefaultAllow bool
		intentions            structs.Intentions
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
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Run("network filter", func(t *testing.T) {
				filter, err := makeRBACNetworkFilter(tt.intentions, tt.intentionDefaultAllow)
				require.NoError(t, err)

				t.Run("current", func(t *testing.T) {
					gotJSON := protoToJSON(t, filter)

					require.JSONEq(t, goldenSimple(t, filepath.Join("rbac", name), gotJSON), gotJSON)
				})
			})
			t.Run("http filter", func(t *testing.T) {
				filter, err := makeRBACHTTPFilter(tt.intentions, tt.intentionDefaultAllow)
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
	sorted := func(ixns ...*structs.Intention) structs.Intentions {
		sort.SliceStable(ixns, func(i, j int) bool {
			return ixns[j].Precedence < ixns[i].Precedence
		})
		return structs.Intentions(ixns)
	}
	tests := map[string]struct {
		in     structs.Intentions
		expect structs.Intentions
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
		tester, against string
		matches         bool
	}{
		// identical precedence
		{"web", "api", false},
		{"*", "*", false},
		// backwards precedence
		{"*", "web", false},
		// name wildcards
		{"web", "*", true},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s cmp %s", tc.tester, tc.against), func(t *testing.T) {
			matches := ixnSourceMatches(
				structs.ServiceNameFromString(tc.tester),
				structs.ServiceNameFromString(tc.against),
			)
			assert.Equal(t, tc.matches, matches)
		})
	}
}

func makeServiceNameSlice(slice []string) []structs.ServiceName {
	if len(slice) == 0 {
		return nil
	}
	var out []structs.ServiceName
	for _, src := range slice {
		out = append(out, structs.ServiceNameFromString(src))
	}
	return out
}

package xds

import (
	"fmt"
	"sort"

	envoylistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoyhttprbac "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rbac/v2"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	envoynetrbac "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rbac/v2"
	envoyrbac "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v2"
	envoymatcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher"
	"github.com/hashicorp/consul/agent/structs"
)

func makeRBACNetworkFilter(intentions structs.Intentions, intentionDefaultAllow bool) (*envoylistener.Filter, error) {
	rules, err := makeRBACRules(intentions, intentionDefaultAllow)
	if err != nil {
		return nil, err
	}

	cfg := &envoynetrbac.RBAC{
		StatPrefix: "connect_authz",
		Rules:      rules,
	}
	return makeFilter("envoy.filters.network.rbac", cfg, false)
}

func makeRBACHTTPFilter(intentions structs.Intentions, intentionDefaultAllow bool) (*envoyhttp.HttpFilter, error) {
	rules, err := makeRBACRules(intentions, intentionDefaultAllow)
	if err != nil {
		return nil, err
	}

	cfg := &envoyhttprbac.RBAC{
		Rules: rules,
	}
	return makeEnvoyHTTPFilter("envoy.filters.http.rbac", cfg)
}

type rbacIntention struct {
	Source     structs.ServiceName
	NotSources []structs.ServiceName
	Allow      bool
	Precedence int
	Skip       bool
}

func (r *rbacIntention) Simplify() {
	r.NotSources = simplifyNotSourceSlice(r.NotSources)
}

func simplifyNotSourceSlice(notSources []structs.ServiceName) []structs.ServiceName {
	if len(notSources) <= 1 {
		return notSources
	}

	// Collapse NotSources elements together if any element is a subset of
	// another.

	// Sort, keeping the least wildcarded elements first.
	sort.SliceStable(notSources, func(i, j int) bool {
		return countWild(notSources[i]) < countWild(notSources[j])
	})

	keep := make([]structs.ServiceName, 0, len(notSources))
	for i := 0; i < len(notSources); i++ {
		si := notSources[i]
		remove := false
		for j := i + 1; j < len(notSources); j++ {
			sj := notSources[j]

			if ixnSourceMatches(si, sj) {
				remove = true
				break
			}
		}
		if !remove {
			keep = append(keep, si)
		}
	}

	return keep
}

// makeRBACRules translates Consul intentions into RBAC Policies for Envoy.
//
// Consul lets you define up to 9 different kinds of intentions that apply at
// different levels of precedence (this is limited to 4 if not using Consul
// Enterprise). Each intention in this flat list (sorted by precedence) can either
// be an allow rule or a deny rule. Here’s a concrete example of this at work:
//
//     intern/trusted-app => billing/payment-svc : ALLOW (prec=9)
//     intern/*           => billing/payment-svc : DENY  (prec=8)
//     */*                => billing/payment-svc : ALLOW (prec=7)
//     ::: ACL default policy :::                : DENY  (prec=N/A)
//
// In contrast, Envoy lets you either configure a filter to be based on an
// allow-list or a deny-list based on the action attribute of the RBAC rules
// struct.
//
// On the surface it would seem that the configuration model of Consul
// intentions is incompatible with that of Envoy’s RBAC engine. For any given
// destination service Consul’s model requires evaluating a list of rules and
// short circuiting later rules once an earlier rule matches. After a rule is
// found to match then we decide if it is allow/deny. Envoy on the other hand
// requires the rules to express all conditions to allow access or all conditions
// to deny access.
//
// Despite the surface incompatibility it is possible to marry these two
// models. For clarity I’ll rewrite the earlier example intentions in an
// abbreviated form:
//
//     A         : ALLOW
//     B         : DENY
//     C         : ALLOW
//     <default> : DENY
//
// 1. Given that the overall intention default is set to deny, we start by
//    choosing to build an allow-list in Envoy (this is also the variant that I find
//    easier to think about).
// 2. Next we traverse the list in precedence order (top down) and any DENY
//    intentions are combined with later intentions using logical operations.
// 3. Now that all of the intentions result in the same action (allow) we have
//    successfully removed precedence and we can express this in as a set of Envoy
//    RBAC policies.
//
// After this the earlier A/B/C/default list becomes:
//
//     A            : ALLOW
//     C AND NOT(B) : ALLOW
//     <default>    : DENY
//
// Which really is just an allow-list of [A, C AND NOT(B)]
func makeRBACRules(intentions structs.Intentions, intentionDefaultAllow bool) (*envoyrbac.RBAC, error) {
	// Note that we DON'T explicitly validate the trust-domain matches ours.
	//
	// For now we don't validate the trust domain of the _destination_ at all.
	// The RBAC policies below ignore the trust domain and it's implicit that
	// the request is for the correct cluster. We might want to reconsider this
	// later but plumbing in additional machinery to check the clusterID here
	// is not really necessary for now unless the Envoys are badly configured.
	// Our threat model _requires_ correctly configured and well behaved
	// proxies given that they have ACLs to fetch certs and so can do whatever
	// they want including not authorizing traffic at all or routing it do a
	// different service than they auth'd against.

	// TODO(banks,rb): Implement revocation list checking?

	// Omit any lower-precedence intentions that share the same source.
	intentions = removeSameSourceIntentions(intentions)

	// First build up just the basic principal matches.
	rbacIxns := make([]*rbacIntention, 0, len(intentions))
	for _, ixn := range intentions {
		rbacIxns = append(rbacIxns, &rbacIntention{
			Source:     ixn.SourceServiceName(),
			Allow:      (ixn.Action == structs.IntentionActionAllow),
			Precedence: ixn.Precedence,
		})
	}

	// Normalize: if we are in default-deny then all intentions must be allows and vice versa

	var rbacAction envoyrbac.RBAC_Action
	if intentionDefaultAllow {
		// The RBAC policies deny access to principals. The rest is allowed.
		// This is block-list style access control.
		rbacAction = envoyrbac.RBAC_DENY
	} else {
		// The RBAC policies grant access to principals. The rest is denied.
		// This is safe-list style access control. This is the default type.
		rbacAction = envoyrbac.RBAC_ALLOW
	}

	// First walk backwards and if we encounter an intention with an action
	// that is the same as the default intention action, add it to all
	// subsequent statements (via AND NOT $x) and mark the rule itself for
	// erasure.
	//
	// i.e. for a default-deny setup we look for denies.
	if len(rbacIxns) > 0 {
		for i := len(rbacIxns) - 1; i >= 0; i-- {
			if rbacIxns[i].Allow == intentionDefaultAllow {
				for j := i + 1; j < len(rbacIxns); j++ {
					if rbacIxns[j].Skip {
						continue
					}
					// [i] is the intention candidate that we are distributing
					// [j] is the thing to maybe NOT [i] from
					if ixnSourceMatches(rbacIxns[i].Source, rbacIxns[j].Source) {
						rbacIxns[j].NotSources = append(rbacIxns[j].NotSources, rbacIxns[i].Source)
					}
				}
				// since this is default-FOO, any trailing FOO intentions will just evaporate
				rbacIxns[i].Skip = true // mark for deletion
			}
		}
	}
	// At this point precedence doesn't matter since all roads lead to the same action.

	var principals []*envoyrbac.Principal
	for _, rbacIxn := range rbacIxns {
		if rbacIxn.Skip {
			continue
		}

		// NOTE: at this point "rbacIxn.Allow != intentionDefaultAllow"

		rbacIxn.Simplify()

		if len(rbacIxn.NotSources) > 0 {
			andIDs := make([]*envoyrbac.Principal, 0, len(rbacIxn.NotSources)+1)
			andIDs = append(andIDs, idPrincipal(rbacIxn.Source))
			for _, src := range rbacIxn.NotSources {
				andIDs = append(andIDs, notPrincipal(
					idPrincipal(src),
				))
			}
			principals = append(principals, andPrincipals(andIDs))
		} else {
			principals = append(principals, idPrincipal(rbacIxn.Source))
		}
	}

	rbac := &envoyrbac.RBAC{
		Action: rbacAction,
	}
	if len(principals) > 0 {
		policy := &envoyrbac.Policy{
			Principals:  principals,
			Permissions: []*envoyrbac.Permission{anyPermission()},
		}
		rbac.Policies = map[string]*envoyrbac.Policy{
			"consul-intentions": policy,
		}
	}

	return rbac, nil
}

func removeSameSourceIntentions(intentions structs.Intentions) structs.Intentions {
	if len(intentions) < 2 {
		return intentions
	}

	var (
		out        = make(structs.Intentions, 0, len(intentions))
		changed    = false
		seenSource = make(map[structs.ServiceName]struct{})
	)
	for _, ixn := range intentions {
		sn := ixn.SourceServiceName()
		if _, ok := seenSource[sn]; ok {
			// A higher precedence intention already used this exact source
			// definition with a different destination.
			changed = true
			continue
		}
		seenSource[sn] = struct{}{}
		out = append(out, ixn)
	}

	if !changed {
		return intentions
	}
	return out
}

type sourceMatch int

const (
	sourceMatchIgnore   sourceMatch = 0
	sourceMatchSuperset sourceMatch = 1
	matchSameSubset     sourceMatch = 2
)

// ixnSourceMatches deterines if the 'tester' service name is matched by the
// 'against' service name via wildcard rules.
//
// For instance:
// - (web, api)               => false, because these have no wildcards
// - (web, *)                 => true,  because "all services" includes "web"
// - (default/web, default/*) => true,  because "all services in the default NS" includes "default/web"
// - (default/*, */*)         => true,  "any service in any NS" includes "all services in the default NS"
func ixnSourceMatches(tester, against structs.ServiceName) bool {
	// We assume that we can't have the same intention twice before arriving
	// here.
	numWildTester := countWild(tester)
	numWildAgainst := countWild(against)

	if numWildTester == numWildAgainst {
		return false
	} else if numWildTester > numWildAgainst {
		return false
	}

	matchesNS := tester.NamespaceOrDefault() == against.NamespaceOrDefault() || against.NamespaceOrDefault() == structs.WildcardSpecifier
	matchesName := tester.Name == against.Name || against.Name == structs.WildcardSpecifier
	return matchesNS && matchesName
}

// countWild counts the number of wildcard values in the given namespace and name.
func countWild(src structs.ServiceName) int {
	// If NS is wildcard, it must be 2 since wildcards only follow exact
	if src.NamespaceOrDefault() == structs.WildcardSpecifier {
		return 2
	}

	// Same reasoning as above, a wildcard can only follow an exact value
	// and an exact value cannot follow a wildcard, so if name is a wildcard
	// we must have exactly one.
	if src.Name == structs.WildcardSpecifier {
		return 1
	}

	return 0
}

func andPrincipals(ids []*envoyrbac.Principal) *envoyrbac.Principal {
	return &envoyrbac.Principal{
		Identifier: &envoyrbac.Principal_AndIds{
			AndIds: &envoyrbac.Principal_Set{
				Ids: ids,
			},
		},
	}
}

func notPrincipal(id *envoyrbac.Principal) *envoyrbac.Principal {
	return &envoyrbac.Principal{
		Identifier: &envoyrbac.Principal_NotId{
			NotId: id,
		},
	}
}

func idPrincipal(src structs.ServiceName) *envoyrbac.Principal {
	pattern := makeSpiffePattern(src.NamespaceOrDefault(), src.Name)

	return &envoyrbac.Principal{
		Identifier: &envoyrbac.Principal_Authenticated_{
			Authenticated: &envoyrbac.Principal_Authenticated{
				PrincipalName: &envoymatcher.StringMatcher{
					MatchPattern: &envoymatcher.StringMatcher_SafeRegex{
						SafeRegex: makeEnvoyRegexMatch(pattern),
					},
				},
			},
		},
	}
}
func makeSpiffePattern(sourceNS, sourceName string) string {
	const (
		anyPath        = `[^/]+`
		spiffeTemplate = `^spiffe://%s/ns/%s/dc/%s/svc/%s$`
	)
	switch {
	case sourceNS != structs.WildcardSpecifier && sourceName != structs.WildcardSpecifier:
		return fmt.Sprintf(spiffeTemplate, anyPath, sourceNS, anyPath, sourceName)
	case sourceNS != structs.WildcardSpecifier && sourceName == structs.WildcardSpecifier:
		return fmt.Sprintf(spiffeTemplate, anyPath, sourceNS, anyPath, anyPath)
	case sourceNS == structs.WildcardSpecifier && sourceName == structs.WildcardSpecifier:
		return fmt.Sprintf(spiffeTemplate, anyPath, anyPath, anyPath, anyPath)
	default:
		panic(fmt.Sprintf("not possible to have a wildcarded namespace %q but an exact service %q", sourceNS, sourceName))
	}
}

func anyPermission() *envoyrbac.Permission {
	return &envoyrbac.Permission{
		Rule: &envoyrbac.Permission_Any{Any: true},
	}
}

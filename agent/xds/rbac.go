package xds

import (
	"fmt"
	"sort"
	"strings"

	envoylistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoyhttprbac "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rbac/v2"
	envoyhttp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	envoynetrbac "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rbac/v2"
	envoyrbac "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v2"
	envoymatcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher"
	"github.com/hashicorp/consul/agent/structs"
)

func makeRBACNetworkFilter(intentions structs.Intentions, intentionDefaultAllow bool) (*envoylistener.Filter, error) {
	rules, err := makeRBACRules(intentions, intentionDefaultAllow, false)
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
	rules, err := makeRBACRules(intentions, intentionDefaultAllow, true)
	if err != nil {
		return nil, err
	}

	cfg := &envoyhttprbac.RBAC{
		Rules: rules,
	}
	return makeEnvoyHTTPFilter("envoy.filters.http.rbac", cfg)
}

func intentionListToIntermediateRBACForm(intentions structs.Intentions, isHTTP bool) []*rbacIntention {
	sort.Sort(structs.IntentionPrecedenceSorter(intentions))

	// Omit any lower-precedence intentions that share the same source.
	intentions = removeSameSourceIntentions(intentions)

	rbacIxns := make([]*rbacIntention, 0, len(intentions))
	for _, ixn := range intentions {
		rixn := intentionToIntermediateRBACForm(ixn, isHTTP)
		rbacIxns = append(rbacIxns, rixn)
	}
	return rbacIxns
}

func removeSourcePrecedence(rbacIxns []*rbacIntention, intentionDefaultAction intentionAction) []*rbacIntention {
	if len(rbacIxns) == 0 {
		return nil
	}

	// Remove source precedence:
	//
	// First walk backwards and add each intention to all subsequent statements
	// (via AND NOT $x).
	//
	// If it is L4 and has the same action as the default intention action then
	// mark the rule itself for erasure.
	numRetained := 0
	for i := len(rbacIxns) - 1; i >= 0; i-- {
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
		if rbacIxns[i].Action == intentionDefaultAction {
			// Lower precedence intentions that match the default intention
			// action are skipped, since they're handled by the default
			// catch-all.
			rbacIxns[i].Skip = true // mark for deletion
		} else {
			numRetained++
		}
	}
	// At this point precedence doesn't matter for the source element.

	// Remove skipped intentions and also compute the final Principals for each
	// intention.
	out := make([]*rbacIntention, 0, numRetained)
	for _, rixn := range rbacIxns {
		if rixn.Skip {
			continue
		}

		rixn.ComputedPrincipal = rixn.FlattenPrincipal()
		out = append(out, rixn)
	}

	return out
}

func removeIntentionPrecedence(rbacIxns []*rbacIntention, intentionDefaultAction intentionAction) []*rbacIntention {
	// Remove source precedence. After this completes precedence doesn't matter
	// between any two intentions.
	rbacIxns = removeSourcePrecedence(rbacIxns, intentionDefaultAction)

	for _, rbacIxn := range rbacIxns {
		// Remove permission precedence. After this completes precedence
		// doesn't matter between any two permissions on this intention.
		rbacIxn.Permissions = removePermissionPrecedence(rbacIxn.Permissions, intentionDefaultAction)
	}

	return rbacIxns
}

func removePermissionPrecedence(perms []*rbacPermission, intentionDefaultAction intentionAction) []*rbacPermission {
	if len(perms) == 0 {
		return nil
	}

	// First walk backwards and add each permission to all subsequent
	// statements (via AND NOT $x).
	//
	// If it has the same action as the default intention action then mark the
	// permission itself for erasure.
	numRetained := 0
	for i := len(perms) - 1; i >= 0; i-- {
		for j := i + 1; j < len(perms); j++ {
			if perms[j].Skip {
				continue
			}
			// [i] is the permission candidate that we are distributing
			// [j] is the thing to maybe NOT [i] from
			perms[j].NotPerms = append(
				perms[j].NotPerms,
				perms[i].Perm,
			)
		}
		if perms[i].Action == intentionDefaultAction {
			// Lower precedence permissions that match the default intention
			// action are skipped, since they're handled by the default
			// catch-all.
			perms[i].Skip = true // mark for deletion
		} else {
			numRetained++
		}
	}

	// Remove skipped permissions and also compute the final Permissions for each item.
	out := make([]*rbacPermission, 0, numRetained)
	for _, perm := range perms {
		if perm.Skip {
			continue
		}

		perm.ComputedPermission = perm.Flatten()
		out = append(out, perm)
	}

	return out
}

func intentionToIntermediateRBACForm(ixn *structs.Intention, isHTTP bool) *rbacIntention {
	rixn := &rbacIntention{
		Source:     ixn.SourceServiceName(),
		Precedence: ixn.Precedence,
	}
	if len(ixn.Permissions) > 0 {
		if isHTTP {
			rixn.Action = intentionActionLayer7
			rixn.Permissions = make([]*rbacPermission, 0, len(ixn.Permissions))
			for _, perm := range ixn.Permissions {
				rixn.Permissions = append(rixn.Permissions, &rbacPermission{
					Definition: perm,
					Action:     intentionActionFromString(perm.Action),
					Perm:       convertPermission(perm),
				})
			}
		} else {
			// In case L7 intentions slip through to here, treat them as deny intentions.
			rixn.Action = intentionActionDeny
		}
	} else {
		rixn.Action = intentionActionFromString(ixn.Action)
	}

	return rixn
}

type intentionAction int

const (
	intentionActionDeny intentionAction = iota
	intentionActionAllow
	intentionActionLayer7
)

func intentionActionFromBool(v bool) intentionAction {
	if v {
		return intentionActionAllow
	} else {
		return intentionActionDeny
	}
}
func intentionActionFromString(s structs.IntentionAction) intentionAction {
	if s == structs.IntentionActionAllow {
		return intentionActionAllow
	}
	return intentionActionDeny
}

type rbacIntention struct {
	Source      structs.ServiceName
	NotSources  []structs.ServiceName
	Action      intentionAction
	Permissions []*rbacPermission
	Precedence  int

	// Skip is field used to indicate that this intention can be deleted in the
	// final pass. Items marked as true should generally not escape the method
	// that marked them.
	Skip bool

	ComputedPrincipal *envoyrbac.Principal
}

func (r *rbacIntention) FlattenPrincipal() *envoyrbac.Principal {
	r.NotSources = simplifyNotSourceSlice(r.NotSources)

	if len(r.NotSources) == 0 {
		return idPrincipal(r.Source)
	}

	andIDs := make([]*envoyrbac.Principal, 0, len(r.NotSources)+1)
	andIDs = append(andIDs, idPrincipal(r.Source))
	for _, src := range r.NotSources {
		andIDs = append(andIDs, notPrincipal(
			idPrincipal(src),
		))
	}
	return andPrincipals(andIDs)
}

type rbacPermission struct {
	Definition *structs.IntentionPermission

	Action   intentionAction
	Perm     *envoyrbac.Permission
	NotPerms []*envoyrbac.Permission

	// Skip is field used to indicate that this permission can be deleted in
	// the final pass. Items marked as true should generally not escape the
	// method that marked them.
	Skip bool

	ComputedPermission *envoyrbac.Permission
}

func (p *rbacPermission) Flatten() *envoyrbac.Permission {
	if len(p.NotPerms) == 0 {
		return p.Perm
	}

	parts := make([]*envoyrbac.Permission, 0, len(p.NotPerms)+1)
	parts = append(parts, p.Perm)
	for _, notPerm := range p.NotPerms {
		parts = append(parts, notPermission(notPerm))
	}
	return andPermissions(parts)
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
func makeRBACRules(intentions structs.Intentions, intentionDefaultAllow bool, isHTTP bool) (*envoyrbac.RBAC, error) {
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

	// First build up just the basic principal matches.
	rbacIxns := intentionListToIntermediateRBACForm(intentions, isHTTP)

	// Normalize: if we are in default-deny then all intentions must be allows and vice versa
	intentionDefaultAction := intentionActionFromBool(intentionDefaultAllow)

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

	// Remove source and permissions precedence.
	rbacIxns = removeIntentionPrecedence(rbacIxns, intentionDefaultAction)

	// For L4: we should generate one big Policy listing all Principals
	// For L7: we should generate one Policy per Principal and list all of the Permissions
	rbac := &envoyrbac.RBAC{
		Action:   rbacAction,
		Policies: make(map[string]*envoyrbac.Policy),
	}

	var principalsL4 []*envoyrbac.Principal
	for i, rbacIxn := range rbacIxns {
		if len(rbacIxn.Permissions) > 0 {
			if !isHTTP {
				panic("invalid state: L7 permissions present for TCP service")
			}
			// For L7: we should generate one Policy per Principal and list all of the Permissions
			policy := &envoyrbac.Policy{
				Principals:  []*envoyrbac.Principal{rbacIxn.ComputedPrincipal},
				Permissions: make([]*envoyrbac.Permission, 0, len(rbacIxn.Permissions)),
			}
			for _, perm := range rbacIxn.Permissions {
				policy.Permissions = append(policy.Permissions, perm.ComputedPermission)
			}
			rbac.Policies[fmt.Sprintf("consul-intentions-layer7-%d", i)] = policy
		} else {
			// For L4: we should generate one big Policy listing all Principals
			principalsL4 = append(principalsL4, rbacIxn.ComputedPrincipal)
		}
	}
	if len(principalsL4) > 0 {
		rbac.Policies["consul-intentions-layer4"] = &envoyrbac.Policy{
			Principals:  principalsL4,
			Permissions: []*envoyrbac.Permission{anyPermission()},
		}
	}

	if len(rbac.Policies) == 0 {
		rbac.Policies = nil
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

func convertPermission(perm *structs.IntentionPermission) *envoyrbac.Permission {
	// NOTE: this does not do anything with perm.Action
	if perm.HTTP == nil {
		return anyPermission()
	}

	var parts []*envoyrbac.Permission

	switch {
	case perm.HTTP.PathExact != "":
		parts = append(parts, &envoyrbac.Permission{
			Rule: &envoyrbac.Permission_UrlPath{
				UrlPath: &envoymatcher.PathMatcher{
					Rule: &envoymatcher.PathMatcher_Path{
						Path: &envoymatcher.StringMatcher{
							MatchPattern: &envoymatcher.StringMatcher_Exact{
								Exact: perm.HTTP.PathExact,
							},
						},
					},
				},
			},
		})
	case perm.HTTP.PathPrefix != "":
		parts = append(parts, &envoyrbac.Permission{
			Rule: &envoyrbac.Permission_UrlPath{
				UrlPath: &envoymatcher.PathMatcher{
					Rule: &envoymatcher.PathMatcher_Path{
						Path: &envoymatcher.StringMatcher{
							MatchPattern: &envoymatcher.StringMatcher_Prefix{
								Prefix: perm.HTTP.PathPrefix,
							},
						},
					},
				},
			},
		})
	case perm.HTTP.PathRegex != "":
		parts = append(parts, &envoyrbac.Permission{
			Rule: &envoyrbac.Permission_UrlPath{
				UrlPath: &envoymatcher.PathMatcher{
					Rule: &envoymatcher.PathMatcher_Path{
						Path: &envoymatcher.StringMatcher{
							MatchPattern: &envoymatcher.StringMatcher_SafeRegex{
								SafeRegex: makeEnvoyRegexMatch(perm.HTTP.PathRegex),
							},
						},
					},
				},
			},
		})
	}

	for _, hdr := range perm.HTTP.Header {
		eh := &envoyroute.HeaderMatcher{
			Name: hdr.Name,
		}

		switch {
		case hdr.Exact != "":
			eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_ExactMatch{
				ExactMatch: hdr.Exact,
			}
		case hdr.Regex != "":
			eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: makeEnvoyRegexMatch(hdr.Regex),
			}
		case hdr.Prefix != "":
			eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_PrefixMatch{
				PrefixMatch: hdr.Prefix,
			}
		case hdr.Suffix != "":
			eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_SuffixMatch{
				SuffixMatch: hdr.Suffix,
			}
		case hdr.Present:
			eh.HeaderMatchSpecifier = &envoyroute.HeaderMatcher_PresentMatch{
				PresentMatch: true,
			}
		default:
			continue // skip this impossible situation
		}

		if hdr.Invert {
			eh.InvertMatch = true
		}

		parts = append(parts, &envoyrbac.Permission{
			Rule: &envoyrbac.Permission_Header{
				Header: eh,
			},
		})
	}

	if len(perm.HTTP.Methods) > 0 {
		methodHeaderRegex := strings.Join(perm.HTTP.Methods, "|")

		eh := &envoyroute.HeaderMatcher{
			Name: ":method",
			HeaderMatchSpecifier: &envoyroute.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: makeEnvoyRegexMatch(methodHeaderRegex),
			},
		}

		parts = append(parts, &envoyrbac.Permission{
			Rule: &envoyrbac.Permission_Header{
				Header: eh,
			},
		})
	}

	// NOTE: if for some reason we errantly allow a permission to be defined
	// with a body of "http{}" then we'll end up treating that like "ANY" here.
	return andPermissions(parts)
}

func notPermission(perm *envoyrbac.Permission) *envoyrbac.Permission {
	return &envoyrbac.Permission{
		Rule: &envoyrbac.Permission_NotRule{NotRule: perm},
	}
}

func andPermissions(perms []*envoyrbac.Permission) *envoyrbac.Permission {
	switch len(perms) {
	case 0:
		return anyPermission()
	case 1:
		return perms[0]
	default:
		return &envoyrbac.Permission{
			Rule: &envoyrbac.Permission_AndRules{
				AndRules: &envoyrbac.Permission_Set{
					Rules: perms,
				},
			},
		}
	}
}

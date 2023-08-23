// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"sort"
	"strings"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_header_to_meta_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/header_to_metadata/v3"
	envoy_http_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_network_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/rbac/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func makeRBACNetworkFilter(
	intentions structs.SimplifiedIntentions,
	intentionDefaultAllow bool,
	localInfo rbacLocalInfo,
	peerTrustBundles []*pbpeering.PeeringTrustBundle,
) (*envoy_listener_v3.Filter, error) {
	rules, err := makeRBACRules(intentions, intentionDefaultAllow, localInfo, false, peerTrustBundles, nil)
	if err != nil {
		return nil, err
	}

	cfg := &envoy_network_rbac_v3.RBAC{
		StatPrefix: "connect_authz",
		Rules:      rules,
	}
	return makeFilter("envoy.filters.network.rbac", cfg)
}

func makeRBACHTTPFilter(
	intentions structs.SimplifiedIntentions,
	intentionDefaultAllow bool,
	localInfo rbacLocalInfo,
	peerTrustBundles []*pbpeering.PeeringTrustBundle,
	providerMap map[string]*structs.JWTProviderConfigEntry,
) (*envoy_http_v3.HttpFilter, error) {
	rules, err := makeRBACRules(intentions, intentionDefaultAllow, localInfo, true, peerTrustBundles, providerMap)
	if err != nil {
		return nil, err
	}

	cfg := &envoy_http_rbac_v3.RBAC{
		Rules: rules,
	}
	return makeEnvoyHTTPFilter("envoy.filters.http.rbac", cfg)
}

func intentionListToIntermediateRBACForm(
	intentions structs.SimplifiedIntentions,
	localInfo rbacLocalInfo,
	isHTTP bool,
	trustBundlesByPeer map[string]*pbpeering.PeeringTrustBundle,
	providerMap map[string]*structs.JWTProviderConfigEntry,
) ([]*rbacIntention, error) {
	sort.Sort(structs.IntentionPrecedenceSorter(intentions))

	// Omit any lower-precedence intentions that share the same source.
	intentions = removeSameSourceIntentions(intentions)

	rbacIxns := make([]*rbacIntention, 0, len(intentions))
	for _, ixn := range intentions {
		// trustBundle is only applicable to imported services
		trustBundle, ok := trustBundlesByPeer[ixn.SourcePeer]
		if ixn.SourcePeer != "" && !ok {
			// If the intention defines a source peer, we expect to
			// see a trust bundle. Otherwise the config snapshot may
			// not have yet received the bundles and we fail silently
			continue
		}

		rixn, err := intentionToIntermediateRBACForm(ixn, localInfo, isHTTP, trustBundle, providerMap)
		if err != nil {
			return nil, err
		}
		rbacIxns = append(rbacIxns, rixn)
	}
	return rbacIxns, nil
}

func removeSourcePrecedence(rbacIxns []*rbacIntention, intentionDefaultAction intentionAction, localInfo rbacLocalInfo) []*rbacIntention {
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

		rixn.ComputedPrincipal = rixn.FlattenPrincipal(localInfo)
		out = append(out, rixn)
	}

	return out
}

func removeIntentionPrecedence(rbacIxns []*rbacIntention, intentionDefaultAction intentionAction, localInfo rbacLocalInfo) []*rbacIntention {
	// Remove source precedence. After this completes precedence doesn't matter
	// between any two intentions.
	rbacIxns = removeSourcePrecedence(rbacIxns, intentionDefaultAction, localInfo)

	numRetained := 0
	for _, rbacIxn := range rbacIxns {
		// Remove permission precedence. After this completes precedence
		// doesn't matter between any two permissions on this intention.
		rbacIxn.Permissions = removePermissionPrecedence(rbacIxn.Permissions, intentionDefaultAction)
		if rbacIxn.Action == intentionActionLayer7 && len(rbacIxn.Permissions) == 0 {
			// All of the permissions must have had the default action type and
			// were removed. Mark this for removal below.
			rbacIxn.Skip = true
		} else {
			numRetained++
		}
	}

	if numRetained == len(rbacIxns) {
		return rbacIxns
	}

	// We previously used the absence of permissions (above) as a signal to
	// mark the entire intention for removal. Now do the deletions.
	out := make([]*rbacIntention, 0, numRetained)
	for _, rixn := range rbacIxns {
		if !rixn.Skip {
			out = append(out, rixn)
		}
	}

	return out
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

func intentionToIntermediateRBACForm(
	ixn *structs.Intention,
	localInfo rbacLocalInfo,
	isHTTP bool,
	bundle *pbpeering.PeeringTrustBundle,
	providerMap map[string]*structs.JWTProviderConfigEntry,
) (*rbacIntention, error) {
	rixn := &rbacIntention{
		Source: rbacService{
			ServiceName: ixn.SourceServiceName(),
			Peer:        ixn.SourcePeer,
			TrustDomain: localInfo.trustDomain,
		},
		Precedence: ixn.Precedence,
	}

	// imported services will have addition metadata used to override SpiffeID creation
	if bundle != nil {
		rixn.Source.ExportedPartition = bundle.ExportedPartition
		rixn.Source.TrustDomain = bundle.TrustDomain
	}

	if isHTTP && ixn.JWT != nil {
		var jwts []*JWTInfo
		for _, prov := range ixn.JWT.Providers {
			jwtProvider, ok := providerMap[prov.Name]

			if !ok {
				return nil, fmt.Errorf("provider specified in intention does not exist. Provider name: %s", prov.Name)
			}
			jwts = append(jwts, newJWTInfo(prov, jwtProvider))
		}

		rixn.jwtInfos = jwts
	}

	if len(ixn.Permissions) > 0 {
		if isHTTP {
			rixn.Action = intentionActionLayer7
			rixn.Permissions = make([]*rbacPermission, 0, len(ixn.Permissions))
			for _, perm := range ixn.Permissions {
				rbacPerm := rbacPermission{
					Definition: perm,
					Action:     intentionActionFromString(perm.Action),
					Perm:       convertPermission(perm),
				}

				if perm.JWT != nil {
					var jwts []*JWTInfo
					for _, prov := range perm.JWT.Providers {
						jwtProvider, ok := providerMap[prov.Name]
						if !ok {
							return nil, fmt.Errorf("provider specified in intention does not exist. Provider name: %s", prov.Name)
						}
						jwts = append(jwts, newJWTInfo(prov, jwtProvider))
					}
					if len(jwts) > 0 {
						rbacPerm.jwtInfos = jwts
					}
				}
				rixn.Permissions = append(rixn.Permissions, &rbacPerm)
			}
		} else {
			// In case L7 intentions slip through to here, treat them as deny intentions.
			rixn.Action = intentionActionDeny
		}
	} else {
		rixn.Action = intentionActionFromString(ixn.Action)
	}

	return rixn, nil
}

func newJWTInfo(p *structs.IntentionJWTProvider, ce *structs.JWTProviderConfigEntry) *JWTInfo {
	return &JWTInfo{
		Provider: p,
		Issuer:   ce.Issuer,
	}
}

type intentionAction int

type JWTInfo struct {
	// Provider issuer
	// this information is coming from the config entry
	Issuer string
	// Provider is the intention provider
	Provider *structs.IntentionJWTProvider
}

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

type rbacService struct {
	structs.ServiceName

	// Peer, ExportedPartition, and TrustDomain are
	// only applicable to imported services and are
	// used to override SPIFFEID fields.
	Peer              string
	ExportedPartition string
	TrustDomain       string
}

type rbacIntention struct {
	Source      rbacService
	NotSources  []rbacService
	Action      intentionAction
	Permissions []*rbacPermission
	Precedence  int

	// JWTInfo is used to track intentions' JWT information
	// This information is used to update HTTP filters for
	// JWT Payload & claims validation
	jwtInfos []*JWTInfo

	// Skip is field used to indicate that this intention can be deleted in the
	// final pass. Items marked as true should generally not escape the method
	// that marked them.
	Skip bool

	ComputedPrincipal *envoy_rbac_v3.Principal
}

func (r *rbacIntention) FlattenPrincipal(localInfo rbacLocalInfo) *envoy_rbac_v3.Principal {
	var principal *envoy_rbac_v3.Principal
	if !localInfo.expectXFCC {
		principal = r.flattenPrincipalFromCert()
	} else if r.Source.Peer == "" {
		// NOTE: ixnSourceMatches should enforce that all of Source and NotSources
		// are peered or not-peered, so we only need to look at the Source element.
		principal = r.flattenPrincipalFromCert() // intention is not relevant to peering
	} else {
		// If this intention is an L7 peered one, then it is exclusively resolvable
		// using XFCC, rather than the TLS SAN field.
		fromXFCC := r.flattenPrincipalFromXFCC()

		// Use of the XFCC one is gated on coming directly from our own gateways.
		gwIDPattern := makeSpiffeMeshGatewayPattern(localInfo.trustDomain, localInfo.partition)

		principal = andPrincipals([]*envoy_rbac_v3.Principal{
			authenticatedPatternPrincipal(gwIDPattern),
			fromXFCC,
		})
	}

	if len(r.jwtInfos) == 0 {
		return principal
	}

	return addJWTPrincipal(principal, r.jwtInfos)
}

func (r *rbacIntention) flattenPrincipalFromCert() *envoy_rbac_v3.Principal {
	r.NotSources = simplifyNotSourceSlice(r.NotSources)

	if len(r.NotSources) == 0 {
		return idPrincipal(r.Source)
	}

	andIDs := make([]*envoy_rbac_v3.Principal, 0, len(r.NotSources)+1)
	andIDs = append(andIDs, idPrincipal(r.Source))
	for _, src := range r.NotSources {
		andIDs = append(andIDs, notPrincipal(
			idPrincipal(src),
		))
	}
	return andPrincipals(andIDs)
}

func (r *rbacIntention) flattenPrincipalFromXFCC() *envoy_rbac_v3.Principal {
	r.NotSources = simplifyNotSourceSlice(r.NotSources)

	if len(r.NotSources) == 0 {
		return xfccPrincipal(r.Source)
	}

	andIDs := make([]*envoy_rbac_v3.Principal, 0, len(r.NotSources)+1)
	andIDs = append(andIDs, xfccPrincipal(r.Source))
	for _, src := range r.NotSources {
		andIDs = append(andIDs, notPrincipal(
			xfccPrincipal(src),
		))
	}
	return andPrincipals(andIDs)
}

type rbacPermission struct {
	Definition *structs.IntentionPermission

	Action   intentionAction
	Perm     *envoy_rbac_v3.Permission
	NotPerms []*envoy_rbac_v3.Permission

	// JWTInfo is used to track intentions' JWT information
	// This information is used to update HTTP filters for
	// JWT Payload & claims validation
	jwtInfos []*JWTInfo

	// Skip is field used to indicate that this permission can be deleted in
	// the final pass. Items marked as true should generally not escape the
	// method that marked them.
	Skip bool

	ComputedPermission *envoy_rbac_v3.Permission
}

// Flatten ensure the permission rules, not-rules, and jwt validation rules are merged into a single computed permission.
//
// Details on JWTInfo section:
// For each JWTInfo (AKA provider required), this builds 1 single permission that validates that the jwt has
// the right issuer (`iss`) field and validates the claims (if any).
//
// After generating a single permission per info, it combines all the info permissions into a single OrPermission.
// This orPermission is then attached to initial computed permission for jwt payload and claims validation.
func (p *rbacPermission) Flatten() *envoy_rbac_v3.Permission {
	computedPermission := p.Perm
	if len(p.NotPerms) == 0 && len(p.jwtInfos) == 0 {
		return computedPermission
	}

	if len(p.NotPerms) != 0 {
		parts := make([]*envoy_rbac_v3.Permission, 0, len(p.NotPerms)+1)
		parts = append(parts, p.Perm)
		for _, notPerm := range p.NotPerms {
			parts = append(parts, notPermission(notPerm))
		}
		computedPermission = andPermissions(parts)
	}

	if len(p.jwtInfos) == 0 {
		return computedPermission
	}

	var jwtPerms []*envoy_rbac_v3.Permission
	for _, info := range p.jwtInfos {
		payloadKey := buildPayloadInMetadataKey(info.Provider.Name)
		claimsPermission := jwtInfosToPermission(info.Provider.VerifyClaims, payloadKey)
		issuerPermission := segmentToPermission(pathToSegments([]string{"iss"}, payloadKey), info.Issuer)

		perm := andPermissions([]*envoy_rbac_v3.Permission{
			issuerPermission, claimsPermission,
		})
		jwtPerms = append(jwtPerms, perm)
	}

	jwtPerm := orPermissions(jwtPerms)
	return andPermissions([]*envoy_rbac_v3.Permission{computedPermission, jwtPerm})
}

// simplifyNotSourceSlice will collapse NotSources elements together if any element is
// a subset of another.
// For example "default/web" is a subset of "default/*" because it is covered by the wildcard.
func simplifyNotSourceSlice(notSources []rbacService) []rbacService {
	if len(notSources) <= 1 {
		return notSources
	}

	// Sort, keeping the least wildcarded elements first.
	// More specific elements have a higher precedence over more wildcarded elements.
	sort.SliceStable(notSources, func(i, j int) bool {
		return countWild(notSources[i]) < countWild(notSources[j])
	})

	keep := make([]rbacService, 0, len(notSources))
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

type rbacLocalInfo struct {
	trustDomain string
	datacenter  string
	partition   string
	expectXFCC  bool
}

// makeRBACRules translates Consul intentions into RBAC Policies for Envoy.
//
// Consul lets you define up to 9 different kinds of intentions that apply at
// different levels of precedence (this is limited to 4 if not using Consul
// Enterprise). Each intention in this flat list (sorted by precedence) can either
// be an allow rule or a deny rule. Here’s a concrete example of this at work:
//
//	intern/trusted-app => billing/payment-svc : ALLOW (prec=9)
//	intern/*           => billing/payment-svc : DENY  (prec=8)
//	*/*                => billing/payment-svc : ALLOW (prec=7)
//	::: ACL default policy :::                : DENY  (prec=N/A)
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
//	A         : ALLOW
//	B         : DENY
//	C         : ALLOW
//	<default> : DENY
//
//	 1. Given that the overall intention default is set to deny, we start by
//	    choosing to build an allow-list in Envoy (this is also the variant that I find
//	    easier to think about).
//	 2. Next we traverse the list in precedence order (top down) and any DENY
//	    intentions are combined with later intentions using logical operations.
//	 3. Now that all of the intentions result in the same action (allow) we have
//	    successfully removed precedence and we can express this in as a set of Envoy
//	    RBAC policies.
//
// After this the earlier A/B/C/default list becomes:
//
//	A            : ALLOW
//	C AND NOT(B) : ALLOW
//	<default>    : DENY
//
// Which really is just an allow-list of [A, C AND NOT(B)]
func makeRBACRules(
	intentions structs.SimplifiedIntentions,
	intentionDefaultAllow bool,
	localInfo rbacLocalInfo,
	isHTTP bool,
	peerTrustBundles []*pbpeering.PeeringTrustBundle,
	providerMap map[string]*structs.JWTProviderConfigEntry,
) (*envoy_rbac_v3.RBAC, error) {
	// TODO(banks,rb): Implement revocation list checking?

	// TODO(peering): mkeeler asked that these maps come from proxycfg instead of
	// being constructed in xds to save memory allocation and gc pressure. Low priority.
	trustBundlesByPeer := make(map[string]*pbpeering.PeeringTrustBundle, len(peerTrustBundles))
	for _, ptb := range peerTrustBundles {
		trustBundlesByPeer[ptb.PeerName] = ptb
	}

	if isHTTP && len(peerTrustBundles) > 0 {
		for _, ixn := range intentions {
			if ixn.SourcePeer != "" {
				localInfo.expectXFCC = true
				break
			}
		}
	}

	// First build up just the basic principal matches.
	rbacIxns, err := intentionListToIntermediateRBACForm(intentions, localInfo, isHTTP, trustBundlesByPeer, providerMap)
	if err != nil {
		return nil, err
	}

	// Normalize: if we are in default-deny then all intentions must be allows and vice versa
	intentionDefaultAction := intentionActionFromBool(intentionDefaultAllow)

	var rbacAction envoy_rbac_v3.RBAC_Action
	if intentionDefaultAllow {
		// The RBAC policies deny access to principals. The rest is allowed.
		// This is block-list style access control.
		rbacAction = envoy_rbac_v3.RBAC_DENY
	} else {
		// The RBAC policies grant access to principals. The rest is denied.
		// This is safe-list style access control. This is the default type.
		rbacAction = envoy_rbac_v3.RBAC_ALLOW
	}

	// Remove source and permissions precedence.
	rbacIxns = removeIntentionPrecedence(rbacIxns, intentionDefaultAction, localInfo)

	// For L4: we should generate one big Policy listing all Principals
	// For L7: we should generate one Policy per Principal and list all of the Permissions
	rbac := &envoy_rbac_v3.RBAC{
		Action:   rbacAction,
		Policies: make(map[string]*envoy_rbac_v3.Policy),
	}

	var principalsL4 []*envoy_rbac_v3.Principal
	for i, rbacIxn := range rbacIxns {
		if rbacIxn.Action == intentionActionLayer7 {
			if len(rbacIxn.Permissions) == 0 {
				panic("invalid state: L7 intention has no permissions")
			}
			if !isHTTP {
				panic("invalid state: L7 permissions present for TCP service")
			}

			rbacPrincipals := optimizePrincipals([]*envoy_rbac_v3.Principal{rbacIxn.ComputedPrincipal})
			// For L7: we should generate one Policy per Principal and list all of the Permissions
			policy := &envoy_rbac_v3.Policy{
				Principals:  rbacPrincipals,
				Permissions: make([]*envoy_rbac_v3.Permission, 0, len(rbacIxn.Permissions)),
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
		rbac.Policies["consul-intentions-layer4"] = &envoy_rbac_v3.Policy{
			Principals:  optimizePrincipals(principalsL4),
			Permissions: []*envoy_rbac_v3.Permission{anyPermission()},
		}
	}

	if len(rbac.Policies) == 0 {
		rbac.Policies = nil
	}
	return rbac, nil
}

// addJWTPrincipal ensure the passed RBAC/Network principal is associated with
// a JWT principal when JWTs validation is required.
//
// For each jwtInfo, this builds a first principal that validates that the jwt has the right issuer (`iss`).
// It collects all the claims principal and combines them into a single principal using jwtClaimsToPrincipals.
// It then combines the issuer principal and the claims principal into a single principal.
//
// After generating a single principal per info, it combines all the info principals into a single jwt OrPrincipal.
// This orPrincipal is then attached to the RBAC/NETWORK principal for jwt payload validation.
func addJWTPrincipal(principal *envoy_rbac_v3.Principal, infos []*JWTInfo) *envoy_rbac_v3.Principal {
	if len(infos) == 0 {
		return principal
	}
	jwtPrincipals := make([]*envoy_rbac_v3.Principal, 0, len(infos))
	for _, info := range infos {
		payloadKey := buildPayloadInMetadataKey(info.Provider.Name)

		// build jwt provider issuer principal
		segments := pathToSegments([]string{"iss"}, payloadKey)
		p := segmentToPrincipal(segments, info.Issuer)

		// add jwt provider claims principal if any
		if cp := jwtClaimsToPrincipals(info.Provider.VerifyClaims, payloadKey); cp != nil {
			p = andPrincipals([]*envoy_rbac_v3.Principal{p, cp})
		}
		jwtPrincipals = append(jwtPrincipals, p)
	}

	// make jwt principals into 1 single principal
	jwtFinalPrincipal := orPrincipals(jwtPrincipals)

	if principal == nil {
		return jwtFinalPrincipal
	}

	return andPrincipals([]*envoy_rbac_v3.Principal{principal, jwtFinalPrincipal})
}

func jwtClaimsToPrincipals(claims []*structs.IntentionJWTClaimVerification, payloadkey string) *envoy_rbac_v3.Principal {
	ps := make([]*envoy_rbac_v3.Principal, 0)

	for _, claim := range claims {
		ps = append(ps, jwtClaimToPrincipal(claim, payloadkey))
	}
	switch len(ps) {
	case 0:
		return nil
	case 1:
		return ps[0]
	default:
		return andPrincipals(ps)
	}
}

// jwtClaimToPrincipal takes in a payloadkey which is the metadata key. This key is generated by using provider name
// and a jwt_payload prefix. See buildPayloadInMetadataKey in agent/xds/jwt_authn.go
//
// This uniquely generated payloadKey is the first segment in the path to validate the JWT claims. The subsequent segments
// come from the Path included in the IntentionJWTClaimVerification param.
func jwtClaimToPrincipal(c *structs.IntentionJWTClaimVerification, payloadKey string) *envoy_rbac_v3.Principal {
	segments := pathToSegments(c.Path, payloadKey)
	return segmentToPrincipal(segments, c.Value)
}

func segmentToPrincipal(segments []*envoy_matcher_v3.MetadataMatcher_PathSegment, v string) *envoy_rbac_v3.Principal {
	return &envoy_rbac_v3.Principal{
		Identifier: &envoy_rbac_v3.Principal_Metadata{
			Metadata: &envoy_matcher_v3.MetadataMatcher{
				Filter: jwtEnvoyFilter,
				Path:   segments,
				Value: &envoy_matcher_v3.ValueMatcher{
					MatchPattern: &envoy_matcher_v3.ValueMatcher_StringMatch{
						StringMatch: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
								Exact: v,
							},
						},
					},
				},
			},
		},
	}
}

func jwtInfosToPermission(claims []*structs.IntentionJWTClaimVerification, payloadkey string) *envoy_rbac_v3.Permission {
	ps := make([]*envoy_rbac_v3.Permission, 0, len(claims))

	for _, claim := range claims {
		ps = append(ps, jwtClaimToPermission(claim, payloadkey))
	}
	return andPermissions(ps)
}

func jwtClaimToPermission(c *structs.IntentionJWTClaimVerification, payloadKey string) *envoy_rbac_v3.Permission {
	segments := pathToSegments(c.Path, payloadKey)
	return segmentToPermission(segments, c.Value)
}

func segmentToPermission(segments []*envoy_matcher_v3.MetadataMatcher_PathSegment, v string) *envoy_rbac_v3.Permission {
	return &envoy_rbac_v3.Permission{
		Rule: &envoy_rbac_v3.Permission_Metadata{
			Metadata: &envoy_matcher_v3.MetadataMatcher{
				Filter: jwtEnvoyFilter,
				Path:   segments,
				Value: &envoy_matcher_v3.ValueMatcher{
					MatchPattern: &envoy_matcher_v3.ValueMatcher_StringMatch{
						StringMatch: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
								Exact: v,
							},
						},
					},
				},
			},
		},
	}
}

// pathToSegments generates an array of MetadataMatcher_PathSegment that starts with the payloadkey
// and is followed by all existing strings in the path.
//
// eg. calling: pathToSegments([]string{"perms", "roles"}, "jwt_payload_okta") should return the following:
//
//	[]*envoy_matcher_v3.MetadataMatcher_PathSegment{
//		{
//			Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "jwt_payload_okta"},
//		},
//		{
//			Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "perms"},
//		},
//		{
//			Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: "roles"},
//		},
//	},
func pathToSegments(paths []string, payloadKey string) []*envoy_matcher_v3.MetadataMatcher_PathSegment {

	segments := make([]*envoy_matcher_v3.MetadataMatcher_PathSegment, 0, len(paths))
	segments = append(segments, makeSegment(payloadKey))

	for _, p := range paths {
		segments = append(segments, makeSegment(p))
	}

	return segments
}

func makeSegment(key string) *envoy_matcher_v3.MetadataMatcher_PathSegment {
	return &envoy_matcher_v3.MetadataMatcher_PathSegment{
		Segment: &envoy_matcher_v3.MetadataMatcher_PathSegment_Key{Key: key},
	}
}

func optimizePrincipals(orig []*envoy_rbac_v3.Principal) []*envoy_rbac_v3.Principal {
	// If they are all ORs, then OR them together.
	var orIds []*envoy_rbac_v3.Principal
	for _, p := range orig {
		or, ok := p.Identifier.(*envoy_rbac_v3.Principal_OrIds)
		if !ok {
			return orig
		}
		// In practice, you can't hit this
		// Only JWTs (HTTP-only) generate orPrinciples, but optimizePrinciples is only called
		// against the combined list of principles for L4 intentions.
		orIds = append(orIds, or.OrIds.Ids...)
	}

	return []*envoy_rbac_v3.Principal{orPrincipals(orIds)}
}

// removeSameSourceIntentions will iterate over intentions and remove any lower precedence
// intentions that share the same source. Intentions are sorted by descending precedence
// so once a source has been seen, additional intentions with the same source can be dropped.
//
// Example for the default/web service:
// input: [(backend/* -> default/web), (backend/* -> default/*)]
// output: [(backend/* -> default/web)]
//
// (backend/* -> default/*) was dropped because it is already known that any service
// in the backend namespace can target default/web.
func removeSameSourceIntentions(intentions structs.SimplifiedIntentions) structs.SimplifiedIntentions {
	if len(intentions) < 2 {
		return intentions
	}

	var (
		out        = make(structs.SimplifiedIntentions, 0, len(intentions))
		changed    = false
		seenSource = make(map[structs.PeeredServiceName]struct{})
	)
	for _, ixn := range intentions {
		psn := structs.PeeredServiceName{
			ServiceName: ixn.SourceServiceName(),
			Peer:        ixn.SourcePeer,
		}
		if _, ok := seenSource[psn]; ok {
			// A higher precedence intention already used this exact source
			// definition with a different destination.
			changed = true
			continue
		}
		seenSource[psn] = struct{}{}
		out = append(out, ixn)
	}

	if !changed {
		return intentions
	}
	return out
}

// ixnSourceMatches determines if the 'tester' service name is matched by the
// 'against' service name via wildcard rules.
//
// For instance:
// - (web, api)               		=> false, because these have no wildcards
// - (web, *)                 		=> true,  because "all services" includes "web"
// - (default/web, default/*) 		=> true,  because "all services in the default NS" includes "default/web"
// - (default/*, */*)         		=> true,  "any service in any NS" includes "all services in the default NS"
// - (default/default/*, other/*/*) => false, "any service in "other" partition" does NOT include services in the default partition"
//
// Peer and partition must be exact names and cannot be compared with wildcards.
func ixnSourceMatches(tester, against rbacService) bool {
	// We assume that we can't have the same intention twice before arriving
	// here.
	numWildTester := countWild(tester)
	numWildAgainst := countWild(against)

	if numWildTester == numWildAgainst {
		return false
	} else if numWildTester > numWildAgainst {
		return false
	}

	matchesAP := tester.PartitionOrDefault() == against.PartitionOrDefault()
	matchesPeer := tester.Peer == against.Peer
	matchesNS := tester.NamespaceOrDefault() == against.NamespaceOrDefault() || against.NamespaceOrDefault() == structs.WildcardSpecifier
	matchesName := tester.Name == against.Name || against.Name == structs.WildcardSpecifier
	return matchesAP && matchesPeer && matchesNS && matchesName
}

// countWild counts the number of wildcard values in the given namespace and name.
func countWild(src rbacService) int {
	// If Partition is wildcard, panic because it's not supported
	if src.PartitionOrDefault() == structs.WildcardSpecifier {
		panic("invalid state: intention references wildcard partition")
	}
	if src.Peer == structs.WildcardSpecifier {
		panic("invalid state: intention references wildcard peer")
	}

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

func andPrincipals(ids []*envoy_rbac_v3.Principal) *envoy_rbac_v3.Principal {
	switch len(ids) {
	case 1:
		return ids[0]
	default:
		return &envoy_rbac_v3.Principal{
			Identifier: &envoy_rbac_v3.Principal_AndIds{
				AndIds: &envoy_rbac_v3.Principal_Set{
					Ids: ids,
				},
			},
		}
	}
}

func orPrincipals(ids []*envoy_rbac_v3.Principal) *envoy_rbac_v3.Principal {
	switch len(ids) {
	case 1:
		return ids[0]
	default:
		return &envoy_rbac_v3.Principal{
			Identifier: &envoy_rbac_v3.Principal_OrIds{
				OrIds: &envoy_rbac_v3.Principal_Set{
					Ids: ids,
				},
			},
		}
	}
}

func notPrincipal(id *envoy_rbac_v3.Principal) *envoy_rbac_v3.Principal {
	return &envoy_rbac_v3.Principal{
		Identifier: &envoy_rbac_v3.Principal_NotId{
			NotId: id,
		},
	}
}

func idPrincipal(src rbacService) *envoy_rbac_v3.Principal {
	pattern := makeSpiffePattern(src)
	return authenticatedPatternPrincipal(pattern)
}

func authenticatedPatternPrincipal(pattern string) *envoy_rbac_v3.Principal {
	return &envoy_rbac_v3.Principal{
		Identifier: &envoy_rbac_v3.Principal_Authenticated_{
			Authenticated: &envoy_rbac_v3.Principal_Authenticated{
				PrincipalName: &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
						SafeRegex: response.MakeEnvoyRegexMatch(pattern),
					},
				},
			},
		},
	}
}

func xfccPrincipal(src rbacService) *envoy_rbac_v3.Principal {
	// Same match we normally would use.
	idPattern := makeSpiffePattern(src)

	// Remove the leading ^ and trailing $.
	idPattern = idPattern[1 : len(idPattern)-1]

	// Anchor to the first XFCC component
	pattern := `^[^,]+;URI=` + idPattern + `(?:,.*)?$`

	// By=spiffe://8c7db6d3-e4ee-aa8c-488c-dbedd3772b78.consul/gateway/mesh/dc/dc2;
	// Hash=2a2db78ac351a05854a0abd350631bf98cc0eb827d21f4ed5935ccd287779eb6;
	// Cert="-----BEGIN%20CERTIFICATE-----<SNIP>";
	// Chain="-----BEGIN%20CERTIFICATE-----<SNIP>";
	// Subject="";
	// URI=spiffe://5583c38e-c1c0-fd1e-2079-170bb2f396ad.consul/ns/default/dc/dc1/svc/pong,

	return &envoy_rbac_v3.Principal{
		Identifier: &envoy_rbac_v3.Principal_Header{
			Header: &envoy_route_v3.HeaderMatcher{
				Name: "x-forwarded-client-cert",
				HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_StringMatch{
					StringMatch: &envoy_matcher_v3.StringMatcher{
						MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
							SafeRegex: response.MakeEnvoyRegexMatch(pattern),
						},
					},
				},
			},
		},
	}
}

const anyPath = `[^/]+`
const trustDomain = anyPath + "." + anyPath

// downstreamServiceIdentityMatcher needs to match XFCC headers in two cases:
// 1. Requests to cluster peered services through a mesh gateway. In this case, the XFCC header looks like the following (I added a new line after each ; for readability)
// By=spiffe://950df996-caef-ddef-ec5f-8d18a153b7b2.consul/gateway/mesh/dc/alpha;
// Hash=...;
// Cert=...;
// Chain=...;
// Subject="";
// URI=spiffe://c7e1d24a-eed8-10a3-286a-52bdb6b6a6fd.consul/ns/default/dc/primary/svc/s1,By=spiffe://950df996-caef-ddef-ec5f-8d18a153b7b2.consul/ns/default/dc/alpha/svc/s2;
// Hash=...;
// Cert=...;
// Chain=...;
// Subject="";
// URI=spiffe://950df996-caef-ddef-ec5f-8d18a153b7b2.consul/gateway/mesh/dc/alpha
//
// 2. Requests directly to another service
// By=spiffe://ae9dbea8-c1dd-7356-b211-c564f7917100.consul/ns/default/dc/primary/svc/s2;
// Hash=396218588ebc1655d32a49b68cedd6b66b9de7b3d69d0c0451bc5818132377d0;
// Cert=...;
// Chain=...;
// Subject="";
// URI=spiffe://ae9dbea8-c1dd-7356-b211-c564f7917100.consul/ns/default/dc/primary/svc/s1
//
// In either case, the regex matches the downstream service's spiffe id because mesh gateways use a different spiffe id format.
// Envoy requires us to include the trailing and leading .* to properly extract the properly submatch.
const downstreamServiceIdentityMatcher = ".*URI=spiffe://(" + trustDomain +
	")(?:/ap/(" + anyPath +
	"))?/ns/(" + anyPath +
	")/dc/(" + anyPath +
	")/svc/([^/;,]+).*"

func parseXFCCToDynamicMetaHTTPFilter() (*envoy_http_v3.HttpFilter, error) {
	var rules []*envoy_http_header_to_meta_v3.Config_Rule

	fields := []struct {
		name string
		sub  string
	}{
		{
			name: "trust-domain",
			sub:  `\1`,
		},
		{
			name: "partition",
			sub:  `\2`,
		},
		{
			name: "namespace",
			sub:  `\3`,
		},
		{
			name: "datacenter",
			sub:  `\4`,
		},
		{
			name: "service",
			sub:  `\5`,
		},
	}

	for _, f := range fields {
		rules = append(rules, &envoy_http_header_to_meta_v3.Config_Rule{
			Header: "x-forwarded-client-cert",
			OnHeaderPresent: &envoy_http_header_to_meta_v3.Config_KeyValuePair{
				MetadataNamespace: "consul",
				Key:               f.name,
				RegexValueRewrite: &envoy_matcher_v3.RegexMatchAndSubstitute{
					Pattern: &envoy_matcher_v3.RegexMatcher{
						Regex: downstreamServiceIdentityMatcher,
						EngineType: &envoy_matcher_v3.RegexMatcher_GoogleRe2{
							GoogleRe2: &envoy_matcher_v3.RegexMatcher_GoogleRE2{},
						},
					},
					Substitution: f.sub,
				},
			},
		})
	}

	cfg := &envoy_http_header_to_meta_v3.Config{RequestRules: rules}

	return makeEnvoyHTTPFilter("envoy.filters.http.header_to_metadata", cfg)
}

func makeSpiffePattern(src rbacService) string {
	var (
		host = src.TrustDomain
		ap   = src.PartitionOrDefault()
		ns   = src.NamespaceOrDefault()
		svc  = src.Name
	)

	// Validate proper wildcarding
	if ns == structs.WildcardSpecifier && svc != structs.WildcardSpecifier {
		panic(fmt.Sprintf("not possible to have a wildcarded namespace %q but an exact service %q", ns, svc))
	}
	if ap == structs.WildcardSpecifier {
		panic("not possible to have a wildcarded source partition")
	}
	if src.Peer == structs.WildcardSpecifier {
		panic("not possible to have a wildcarded source peer")
	}

	// Match on any namespace or service if it is a wildcard, or on a specific value otherwise.
	if ns == structs.WildcardSpecifier {
		ns = anyPath
	}
	if svc == structs.WildcardSpecifier {
		svc = anyPath
	}

	// If service is imported from a peer, the SpiffeID must
	// refer to its remote partition and trust domain.
	if src.Peer != "" {
		ap = src.ExportedPartition
		host = src.TrustDomain
	}

	id := connect.SpiffeIDService{
		Namespace: ns,
		Service:   svc,
		Host:      host,

		// Datacenter is not verified by RBAC, so we match on any value.
		Datacenter: anyPath,

		// Partition can only ever be an exact value.
		Partition: ap,
	}

	return fmt.Sprintf(`^%s://%s%s$`, id.URI().Scheme, id.Host, id.URI().Path)
}

func makeSpiffeMeshGatewayPattern(gwTrustDomain, gwPartition string) string {
	id := connect.SpiffeIDMeshGateway{
		Host:      gwTrustDomain,
		Partition: gwPartition,
		// Datacenter is not verified by RBAC, so we match on any value.
		Datacenter: anyPath,
	}

	return fmt.Sprintf(`^%s://%s%s$`, id.URI().Scheme, id.Host, id.URI().Path)
}

func anyPermission() *envoy_rbac_v3.Permission {
	return &envoy_rbac_v3.Permission{
		Rule: &envoy_rbac_v3.Permission_Any{Any: true},
	}
}

func convertPermission(perm *structs.IntentionPermission) *envoy_rbac_v3.Permission {
	// NOTE: this does not do anything with perm.Action
	if perm.HTTP == nil {
		return anyPermission()
	}

	var parts []*envoy_rbac_v3.Permission

	switch {
	case perm.HTTP.PathExact != "":
		parts = append(parts, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_UrlPath{
				UrlPath: &envoy_matcher_v3.PathMatcher{
					Rule: &envoy_matcher_v3.PathMatcher_Path{
						Path: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
								Exact: perm.HTTP.PathExact,
							},
						},
					},
				},
			},
		})
	case perm.HTTP.PathPrefix != "":
		parts = append(parts, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_UrlPath{
				UrlPath: &envoy_matcher_v3.PathMatcher{
					Rule: &envoy_matcher_v3.PathMatcher_Path{
						Path: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Prefix{
								Prefix: perm.HTTP.PathPrefix,
							},
						},
					},
				},
			},
		})
	case perm.HTTP.PathRegex != "":
		parts = append(parts, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_UrlPath{
				UrlPath: &envoy_matcher_v3.PathMatcher{
					Rule: &envoy_matcher_v3.PathMatcher_Path{
						Path: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
								SafeRegex: response.MakeEnvoyRegexMatch(perm.HTTP.PathRegex),
							},
						},
					},
				},
			},
		})
	}

	for _, hdr := range perm.HTTP.Header {
		eh := &envoy_route_v3.HeaderMatcher{
			Name: hdr.Name,
		}

		switch {
		case hdr.Exact != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_ExactMatch{
				ExactMatch: hdr.Exact,
			}
		case hdr.Regex != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: response.MakeEnvoyRegexMatch(hdr.Regex),
			}
		case hdr.Prefix != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_PrefixMatch{
				PrefixMatch: hdr.Prefix,
			}
		case hdr.Suffix != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_SuffixMatch{
				SuffixMatch: hdr.Suffix,
			}
		case hdr.Present:
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_PresentMatch{
				PresentMatch: true,
			}
		default:
			continue // skip this impossible situation
		}

		if hdr.Invert {
			eh.InvertMatch = true
		}

		parts = append(parts, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_Header{
				Header: eh,
			},
		})
	}

	if len(perm.HTTP.Methods) > 0 {
		methodHeaderRegex := strings.Join(perm.HTTP.Methods, "|")

		eh := &envoy_route_v3.HeaderMatcher{
			Name: ":method",
			HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_SafeRegexMatch{
				SafeRegexMatch: response.MakeEnvoyRegexMatch(methodHeaderRegex),
			},
		}

		parts = append(parts, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_Header{
				Header: eh,
			},
		})
	}

	// NOTE: if for some reason we errantly allow a permission to be defined
	// with a body of "http{}" then we'll end up treating that like "ANY" here.
	return andPermissions(parts)
}

func notPermission(perm *envoy_rbac_v3.Permission) *envoy_rbac_v3.Permission {
	return &envoy_rbac_v3.Permission{
		Rule: &envoy_rbac_v3.Permission_NotRule{NotRule: perm},
	}
}

func andPermissions(perms []*envoy_rbac_v3.Permission) *envoy_rbac_v3.Permission {
	switch len(perms) {
	case 0:
		return anyPermission()
	case 1:
		return perms[0]
	default:
		return &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_AndRules{
				AndRules: &envoy_rbac_v3.Permission_Set{
					Rules: perms,
				},
			},
		}
	}
}

func orPermissions(perms []*envoy_rbac_v3.Permission) *envoy_rbac_v3.Permission {
	switch len(perms) {
	case 0:
		return anyPermission()
	case 1:
		return perms[0]
	default:
		return &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_OrRules{
				OrRules: &envoy_rbac_v3.Permission_Set{
					Rules: perms,
				},
			},
		}
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_network_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/rbac/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
)

const (
	baseL4PermissionKey = "consul-intentions-layer4"
)

func MakeL4RBAC(trafficPermissions *pbproxystate.L4TrafficPermissions) ([]*envoy_listener_v3.Filter, error) {
	if trafficPermissions == nil {
		return nil, nil
	}

	rbacs, err := makeRBACs(trafficPermissions)
	if err != nil {
		return nil, err
	}

	filters := make([]*envoy_listener_v3.Filter, 0, len(rbacs))
	for _, rbac := range rbacs {
		cfg := &envoy_network_rbac_v3.RBAC{
			StatPrefix: "connect_authz",
			Rules:      rbac,
		}
		filter, err := makeEnvoyFilter("envoy.filters.network.rbac", cfg)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}

	return filters, nil
}

func makeRBACs(trafficPermissions *pbproxystate.L4TrafficPermissions) ([]*envoy_rbac_v3.RBAC, error) {
	allowRBAC := &envoy_rbac_v3.RBAC{
		Action:   envoy_rbac_v3.RBAC_ALLOW,
		Policies: make(map[string]*envoy_rbac_v3.Policy),
	}

	denyRBAC := &envoy_rbac_v3.RBAC{
		Action:   envoy_rbac_v3.RBAC_DENY,
		Policies: make(map[string]*envoy_rbac_v3.Policy),
	}

	policyLabel := func(i int) string {
		if len(trafficPermissions.Permissions) == 1 {
			return baseL4PermissionKey
		}
		return fmt.Sprintf("%s-%d", baseL4PermissionKey, i)
	}

	for i, p := range trafficPermissions.Permissions {
		allowPolicy, err := makeRBACPolicy(allowRBAC.Action, p.AllowPrincipals)
		if err != nil {
			return nil, err
		}

		if allowPolicy != nil {
			allowRBAC.Policies[policyLabel(i)] = allowPolicy
		}

		denyPolicy, err := makeRBACPolicy(denyRBAC.Action, p.DenyPrincipals)
		if err != nil {
			return nil, err
		}

		if denyPolicy != nil {
			denyRBAC.Policies[policyLabel(i)] = denyPolicy
		}
	}

	var rbacs []*envoy_rbac_v3.RBAC
	if rbac := finalizeRBAC(allowRBAC, trafficPermissions.DefaultAction); rbac != nil {
		rbacs = append(rbacs, rbac)
	}

	if rbac := finalizeRBAC(denyRBAC, trafficPermissions.DefaultAction); rbac != nil {
		rbacs = append(rbacs, rbac)
	}

	return rbacs, nil
}

func finalizeRBAC(rbac *envoy_rbac_v3.RBAC, defaultAction pbproxystate.TrafficPermissionAction) *envoy_rbac_v3.RBAC {
	isRBACAllow := rbac.Action == envoy_rbac_v3.RBAC_ALLOW
	isConsulAllow := defaultAction == pbproxystate.TrafficPermissionAction_INTENTION_ACTION_ALLOW
	// Remove allow traffic permissions with default allow. This is required because including an allow RBAC filter enforces default deny.
	// It is safe because deny traffic permissions are applied before allow permissions, so explicit allow is equivalent to default allow. 
	removeAllows := isRBACAllow && isConsulAllow
	if removeAllows {
		return nil
	}

	if len(rbac.Policies) != 0 {
		return rbac
	}

	// Include an empty allow RBAC filter to enforce Consul's default deny.
	includeEmpty := isRBACAllow && !isConsulAllow
	if includeEmpty {
		return rbac
	}

	return nil
}

func makeRBACPolicy(action envoy_rbac_v3.RBAC_Action, l4Principals []*pbproxystate.L4Principal) (*envoy_rbac_v3.Policy, error) {
	if len(l4Principals) == 0 {
		return nil, nil
	}

	principals := make([]*envoy_rbac_v3.Principal, 0, len(l4Principals))
	for _, s := range l4Principals {
		principals = append(principals, toEnvoyPrincipal(s.ToL7Principal()))
	}

	return &envoy_rbac_v3.Policy{
		Principals:  principals,
		Permissions: []*envoy_rbac_v3.Permission{anyPermission()},
	}, nil
}

func toEnvoyPrincipal(p *pbproxystate.L7Principal) *envoy_rbac_v3.Principal {
	orIDs := make([]*envoy_rbac_v3.Principal, 0, len(p.Spiffes))
	for _, regex := range p.Spiffes {
		orIDs = append(orIDs, principal(regex.Regex, false, regex.Xfcc))
	}

	includePrincipal := orPrincipals(orIDs)

	if len(p.ExcludeSpiffes) == 0 {
		return includePrincipal
	}

	principals := make([]*envoy_rbac_v3.Principal, 0, len(p.ExcludeSpiffes)+1)
	principals = append(principals, includePrincipal)
	for _, sid := range p.ExcludeSpiffes {
		principals = append(principals, principal(sid.Regex, true, sid.Xfcc))
	}
	return andPrincipals(principals)
}

func principal(spiffeID string, negate, xfcc bool) *envoy_rbac_v3.Principal {
	var p *envoy_rbac_v3.Principal
	if xfcc {
		p = xfccPrincipal(spiffeID)
	} else {
		p = idPrincipal(spiffeID)
	}

	if !negate {
		return p
	}

	return negatePrincipal(p)
}

func negatePrincipal(p *envoy_rbac_v3.Principal) *envoy_rbac_v3.Principal {
	return &envoy_rbac_v3.Principal{
		Identifier: &envoy_rbac_v3.Principal_NotId{
			NotId: p,
		},
	}
}

func idPrincipal(spiffeID string) *envoy_rbac_v3.Principal {
	return &envoy_rbac_v3.Principal{
		Identifier: &envoy_rbac_v3.Principal_Authenticated_{
			Authenticated: &envoy_rbac_v3.Principal_Authenticated{
				PrincipalName: &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
						SafeRegex: response.MakeEnvoyRegexMatch(spiffeID),
					},
				},
			},
		},
	}
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

func anyPermission() *envoy_rbac_v3.Permission {
	return &envoy_rbac_v3.Permission{
		Rule: &envoy_rbac_v3.Permission_Any{Any: true},
	}
}

func xfccPrincipal(spiffeID string) *envoy_rbac_v3.Principal {
	return &envoy_rbac_v3.Principal{
		Identifier: &envoy_rbac_v3.Principal_Header{
			Header: &envoy_route_v3.HeaderMatcher{
				Name: "x-forwarded-client-cert",
				HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_StringMatch{
					StringMatch: &envoy_matcher_v3.StringMatcher{
						MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
							SafeRegex: response.MakeEnvoyRegexMatch(spiffeID),
						},
					},
				},
			},
		},
	}
}

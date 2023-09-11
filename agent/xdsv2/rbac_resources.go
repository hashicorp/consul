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
	var filters []*envoy_listener_v3.Filter

	if trafficPermissions == nil {
		return nil, nil
	}

	// Only include the allow RBAC when Consul is in default deny.
	if trafficPermissions.DefaultAction != pbproxystate.TrafficPermissionAction_TRAFFIC_PERMISSION_ACTION_ALLOW {
		allowRBAC := &envoy_rbac_v3.RBAC{
			Action:   envoy_rbac_v3.RBAC_ALLOW,
			Policies: make(map[string]*envoy_rbac_v3.Policy),
		}

		allowRBAC.Policies = makeRBACPolicies(trafficPermissions.AllowPermissions)
		filter, err := makeRBACFilter(allowRBAC)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}

	if len(trafficPermissions.DenyPermissions) > 0 {
		denyRBAC := &envoy_rbac_v3.RBAC{
			Action:   envoy_rbac_v3.RBAC_DENY,
			Policies: make(map[string]*envoy_rbac_v3.Policy),
		}
		denyRBAC.Policies = makeRBACPolicies(trafficPermissions.DenyPermissions)
		filter, err := makeRBACFilter(denyRBAC)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}

	return filters, nil
}

func makeRBACFilter(rbac *envoy_rbac_v3.RBAC) (*envoy_listener_v3.Filter, error) {
	cfg := &envoy_network_rbac_v3.RBAC{
		StatPrefix: "connect_authz",
		Rules:      rbac,
	}
	return makeEnvoyFilter("envoy.filters.network.rbac", cfg)
}

func makeRBACPolicies(l4Permissions []*pbproxystate.L4Permission) map[string]*envoy_rbac_v3.Policy {
	policyLabel := func(i int) string {
		if len(l4Permissions) == 1 {
			return baseL4PermissionKey
		}
		return fmt.Sprintf("%s-%d", baseL4PermissionKey, i)
	}

	policies := make(map[string]*envoy_rbac_v3.Policy, len(l4Permissions))

	for i, permission := range l4Permissions {
		policies[policyLabel(i)] = makeRBACPolicy(permission)
	}

	return policies
}

func makeRBACPolicy(p *pbproxystate.L4Permission) *envoy_rbac_v3.Policy {
	var principals []*envoy_rbac_v3.Principal

	for _, l4Principal := range p.Principals {
		principals = append(principals, toEnvoyPrincipal(l4Principal.ToL7Principal()))
	}

	return &envoy_rbac_v3.Policy{
		Principals:  principals,
		Permissions: []*envoy_rbac_v3.Permission{anyPermission()},
	}
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

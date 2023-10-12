// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"

	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/rbac/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_network_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/rbac/v3"
	envoy_matcher_v3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/proto-public/pbmesh/v2beta1/pbproxystate"
)

const (
	baseL4PermissionKey = "consul-intentions-layer4"
)

// MakeL4RBAC returns the envoy deny and allow rules from the traffic permissions. After calling this function these
// rules can be put into a network rbac filter or http rbac filter depending on the local app port protocol.
func MakeL4RBAC(trafficPermissions *pbproxystate.TrafficPermissions) (deny *envoy_rbac_v3.RBAC, allow *envoy_rbac_v3.RBAC, err error) {
	var denyRBAC *envoy_rbac_v3.RBAC
	var allowRBAC *envoy_rbac_v3.RBAC

	if trafficPermissions == nil {
		return nil, nil, nil
	}

	if len(trafficPermissions.DenyPermissions) > 0 {
		denyRBAC = &envoy_rbac_v3.RBAC{
			Action:   envoy_rbac_v3.RBAC_DENY,
			Policies: make(map[string]*envoy_rbac_v3.Policy),
		}
		denyRBAC.Policies = makeRBACPolicies(trafficPermissions.DenyPermissions)
	}

	// Only include the allow RBAC when Consul is in default deny.
	if !trafficPermissions.DefaultAllow {
		allowRBAC = &envoy_rbac_v3.RBAC{
			Action:   envoy_rbac_v3.RBAC_ALLOW,
			Policies: make(map[string]*envoy_rbac_v3.Policy),
		}

		allowRBAC.Policies = makeRBACPolicies(trafficPermissions.AllowPermissions)
	}

	return denyRBAC, allowRBAC, nil
}

// MakeRBACNetworkFilters calls MakeL4RBAC and wraps the result in envoy network filters meant for L4 protocols.
func MakeRBACNetworkFilters(trafficPermissions *pbproxystate.TrafficPermissions) ([]*envoy_listener_v3.Filter, error) {
	var filters []*envoy_listener_v3.Filter

	deny, allow, err := MakeL4RBAC(trafficPermissions)
	if err != nil {
		return nil, err
	}

	if deny != nil {
		filter, err := makeRBACFilter(deny)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}

	if allow != nil {
		filter, err := makeRBACFilter(allow)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)

	}

	return filters, nil
}

// MakeRBACHTTPFilters calls MakeL4RBAC and wraps the result in envoy http filters meant for L7 protocols. Eventually
// this will need to also accumulate any L7 traffic permissions when that is implemented.
func MakeRBACHTTPFilters(trafficPermissions *pbproxystate.TrafficPermissions) ([]*envoy_http_v3.HttpFilter, error) {
	var httpFilters []*envoy_http_v3.HttpFilter

	deny, allow, err := MakeL4RBAC(trafficPermissions)
	if err != nil {
		return nil, err
	}

	if deny != nil {
		filter, err := makeRBACHTTPFilter(deny)
		if err != nil {
			return nil, err
		}
		httpFilters = append(httpFilters, filter)
	}

	if allow != nil {
		filter, err := makeRBACHTTPFilter(allow)
		if err != nil {
			return nil, err
		}
		httpFilters = append(httpFilters, filter)

	}

	return httpFilters, nil
}

const (
	envoyNetworkRBACFilterKey = "envoy.filters.network.rbac"
	envoyHTTPRBACFilterKey    = "envoy.filters.http.rbac"
)

func makeRBACFilter(rbac *envoy_rbac_v3.RBAC) (*envoy_listener_v3.Filter, error) {
	cfg := &envoy_network_rbac_v3.RBAC{
		StatPrefix: "connect_authz",
		Rules:      rbac,
	}
	return makeEnvoyFilter(envoyNetworkRBACFilterKey, cfg)
}

func makeRBACHTTPFilter(rbac *envoy_rbac_v3.RBAC) (*envoy_http_v3.HttpFilter, error) {
	cfg := &envoy_http_rbac_v3.RBAC{
		Rules: rbac,
	}
	return makeEnvoyHTTPFilter(envoyHTTPRBACFilterKey, cfg)
}

func makeRBACPolicies(l4Permissions []*pbproxystate.Permission) map[string]*envoy_rbac_v3.Policy {
	policyLabel := func(i int) string {
		if len(l4Permissions) == 1 {
			return baseL4PermissionKey
		}
		return fmt.Sprintf("%s-%d", baseL4PermissionKey, i)
	}

	policies := make(map[string]*envoy_rbac_v3.Policy, len(l4Permissions))

	for i, permission := range l4Permissions {
		policy := makeRBACPolicy(permission)
		if policy != nil {
			policies[policyLabel(i)] = policy
		}
	}

	return policies
}

func makeRBACPolicy(p *pbproxystate.Permission) *envoy_rbac_v3.Policy {
	if len(p.Principals) == 0 {
		return nil
	}

	var principals []*envoy_rbac_v3.Principal

	for _, p := range p.Principals {
		principals = append(principals, toEnvoyPrincipal(p))
	}

	return &envoy_rbac_v3.Policy{
		Principals:  principals,
		Permissions: []*envoy_rbac_v3.Permission{anyPermission()},
	}
}

func toEnvoyPrincipal(p *pbproxystate.Principal) *envoy_rbac_v3.Principal {
	includePrincipal := principal(p.Spiffe)

	if len(p.ExcludeSpiffes) == 0 {
		return includePrincipal
	}

	principals := make([]*envoy_rbac_v3.Principal, 0, len(p.ExcludeSpiffes)+1)
	principals = append(principals, includePrincipal)
	for _, s := range p.ExcludeSpiffes {
		principals = append(principals, negatePrincipal(principal(s)))
	}
	return andPrincipals(principals)
}

func principal(spiffe *pbproxystate.Spiffe) *envoy_rbac_v3.Principal {
	var andIDs []*envoy_rbac_v3.Principal
	andIDs = append(andIDs, idPrincipal(spiffe.Regex))

	if len(spiffe.XfccRegex) > 0 {
		andIDs = append(andIDs, xfccPrincipal(spiffe.XfccRegex))
	}

	return andPrincipals(andIDs)
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

func anyPermission() *envoy_rbac_v3.Permission {
	return &envoy_rbac_v3.Permission{
		Rule: &envoy_rbac_v3.Permission_Any{Any: true},
	}
}

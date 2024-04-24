// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdsv2

import (
	"fmt"
	"strings"

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
	baseL7PermissionKey = "consul-intentions-layer7"
)

// MakeRBAC returns the envoy deny and allow rules from the traffic permissions. After calling this function these
// rules can be put into a network rbac filter or http rbac filter depending on the local app port protocol.
func MakeRBAC(trafficPermissions *pbproxystate.TrafficPermissions, makePolicies func([]*pbproxystate.Permission) map[string]*envoy_rbac_v3.Policy) (deny *envoy_rbac_v3.RBAC, allow *envoy_rbac_v3.RBAC, err error) {
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
		denyRBAC.Policies = makePolicies(trafficPermissions.DenyPermissions)
	}

	// Only include the allow RBAC when Consul is in default deny.
	if !trafficPermissions.DefaultAllow {
		allowRBAC = &envoy_rbac_v3.RBAC{
			Action:   envoy_rbac_v3.RBAC_ALLOW,
			Policies: make(map[string]*envoy_rbac_v3.Policy),
		}

		allowRBAC.Policies = makePolicies(trafficPermissions.AllowPermissions)
	}

	return denyRBAC, allowRBAC, nil
}

// MakeRBACNetworkFilters calls MakeL4RBAC and wraps the result in envoy network filters meant for L4 protocols.
func MakeRBACNetworkFilters(trafficPermissions *pbproxystate.TrafficPermissions) ([]*envoy_listener_v3.Filter, error) {
	var filters []*envoy_listener_v3.Filter

	deny, allow, err := MakeRBAC(trafficPermissions, makeL4RBACPolicies)
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

	deny, allow, err := MakeRBAC(trafficPermissions, makeL7RBACPolicies)
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

func makeL4RBACPolicies(l4Permissions []*pbproxystate.Permission) map[string]*envoy_rbac_v3.Policy {
	policies := make(map[string]*envoy_rbac_v3.Policy, len(l4Permissions))

	for i, permission := range l4Permissions {
		if len(permission.DestinationRules) != 0 {
			// This is an L7-only permission
			// ports are split out for separate configuration before this point and L7 filters are configured separately
			continue
		}
		policy := makeL4RBACPolicy(permission)
		if policy != nil {
			policies[l4PolicyLabel(l4Permissions, i)] = policy
		}
	}

	return policies
}

func makeL4RBACPolicy(p *pbproxystate.Permission) *envoy_rbac_v3.Policy {
	if p == nil || len(p.Principals) == 0 {
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

func l4PolicyLabel(perms []*pbproxystate.Permission, i int) string {
	if len(perms) == 1 {
		return baseL4PermissionKey
	}
	return fmt.Sprintf("%s-%d", baseL4PermissionKey, i)
}

func makeL7RBACPolicies(l7Permissions []*pbproxystate.Permission) map[string]*envoy_rbac_v3.Policy {
	// sort permissions into those with L7-specific features and those without, to match labeling and behavior
	// conventions in V1: https://github.com/hashicorp/consul/blob/4e451f23584473a7eaf7f123145ca85e0a31783a/agent/xds/rbac.go#L647
	// this is a somewhat unfortunate carry-over needed for testing v1 vs v2 final config
	// and this will break with v1 intentions when multiple L4 permissions are used
	var l4Perms []*pbproxystate.Permission
	var l7Perms []*pbproxystate.Permission
	for _, p := range l7Permissions {
		if len(p.DestinationRules) > 0 {
			l7Perms = append(l7Perms, p)
		} else {
			l4Perms = append(l4Perms, p)
		}
	}

	policies := make(map[string]*envoy_rbac_v3.Policy, len(l7Permissions))

	// L7 policies first, then L4 per: https://github.com/hashicorp/consul/blob/4e451f23584473a7eaf7f123145ca85e0a31783a/agent/xds/rbac.go#L664
	for i, permission := range l7Perms {
		policy := makeL7RBACPolicy(permission)
		if policy != nil {
			policies[fmt.Sprintf("%s-%d", baseL7PermissionKey, i)] = policy
		}
	}
	for i, permission := range l4Perms {
		policy := makeL4RBACPolicy(permission)
		if policy != nil {
			policies[l4PolicyLabel(l4Perms, i)] = policy
		}
	}

	return policies
}

func makeL7RBACPolicy(p *pbproxystate.Permission) *envoy_rbac_v3.Policy {
	if p == nil || len(p.Principals) == 0 {
		return nil
	}

	var principals []*envoy_rbac_v3.Principal

	for _, p := range p.Principals {
		principals = append(principals, toEnvoyPrincipal(p))
	}
	permissions := permissionsFromDestinationRules(p.DestinationRules)
	return &envoy_rbac_v3.Policy{
		Principals:  principals,
		Permissions: permissions,
	}
}

func translateRule(dr *pbproxystate.DestinationRule) *envoy_rbac_v3.Permission {
	var perms []*envoy_rbac_v3.Permission
	// paths
	switch {
	case dr.PathExact != "":
		perms = append(perms, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_UrlPath{
				UrlPath: &envoy_matcher_v3.PathMatcher{
					Rule: &envoy_matcher_v3.PathMatcher_Path{
						Path: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
								Exact: dr.PathExact,
							},
						},
					},
				},
			},
		})
	case dr.PathPrefix != "":
		perms = append(perms, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_UrlPath{
				UrlPath: &envoy_matcher_v3.PathMatcher{
					Rule: &envoy_matcher_v3.PathMatcher_Path{
						Path: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_Prefix{
								Prefix: dr.PathPrefix,
							},
						},
					},
				},
			},
		})
	case dr.PathRegex != "":
		perms = append(perms, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_UrlPath{
				UrlPath: &envoy_matcher_v3.PathMatcher{
					Rule: &envoy_matcher_v3.PathMatcher_Path{
						Path: &envoy_matcher_v3.StringMatcher{
							MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
								SafeRegex: response.MakeEnvoyRegexMatch(dr.PathRegex),
							},
						},
					},
				},
			},
		})
	}

	// methods
	if len(dr.Methods) > 0 {
		methodHeaderRegex := strings.Join(dr.Methods, "|")
		eh := &envoy_route_v3.HeaderMatcher{
			Name: ":method",
			HeaderMatchSpecifier: &envoy_route_v3.HeaderMatcher_StringMatch{
				StringMatch: &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
						SafeRegex: response.MakeEnvoyRegexMatch(methodHeaderRegex),
					},
				},
			},
		}
		perms = append(perms, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_Header{
				Header: eh,
			}})
	}

	// headers
	for _, hdr := range dr.DestinationRuleHeader {
		eh := &envoy_route_v3.HeaderMatcher{
			Name: hdr.Name,
		}

		switch {
		case hdr.Exact != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_StringMatch{
				StringMatch: &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_Exact{
						Exact: hdr.Exact,
					},
					IgnoreCase: false,
				},
			}
		case hdr.Regex != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_StringMatch{
				StringMatch: &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_SafeRegex{
						SafeRegex: response.MakeEnvoyRegexMatch(hdr.Regex),
					},
					IgnoreCase: false,
				},
			}

		case hdr.Prefix != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_StringMatch{
				StringMatch: &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_Prefix{
						Prefix: hdr.Prefix,
					},
					IgnoreCase: false,
				},
			}

		case hdr.Suffix != "":
			eh.HeaderMatchSpecifier = &envoy_route_v3.HeaderMatcher_StringMatch{
				StringMatch: &envoy_matcher_v3.StringMatcher{
					MatchPattern: &envoy_matcher_v3.StringMatcher_Suffix{
						Suffix: hdr.Suffix,
					},
					IgnoreCase: false,
				},
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

		perms = append(perms, &envoy_rbac_v3.Permission{
			Rule: &envoy_rbac_v3.Permission_Header{
				Header: eh,
			},
		})
	}
	return combineAndPermissions(perms)
}

func permissionsFromDestinationRules(drs []*pbproxystate.DestinationRule) []*envoy_rbac_v3.Permission {
	var perms []*envoy_rbac_v3.Permission
	for _, dr := range drs {
		subPerms := make([]*envoy_rbac_v3.Permission, len(dr.Exclude))
		for i, er := range dr.Exclude {
			translated := translateRule(&pbproxystate.DestinationRule{
				PathExact:             er.PathExact,
				PathPrefix:            er.PathPrefix,
				PathRegex:             er.PathRegex,
				Methods:               er.Methods,
				DestinationRuleHeader: er.Headers,
			})
			subPerms[i] = &envoy_rbac_v3.Permission{
				Rule: &envoy_rbac_v3.Permission_NotRule{NotRule: translated},
			}
		}
		subPerms = append([]*envoy_rbac_v3.Permission{translateRule(dr)}, subPerms...)
		perms = append(perms, combineAndPermissions(subPerms))
	}
	return perms
}

func combineAndPermissions(perms []*envoy_rbac_v3.Permission) *envoy_rbac_v3.Permission {
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

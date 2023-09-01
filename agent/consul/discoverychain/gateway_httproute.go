// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discoverychain

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

// compareHTTPRules implements the non-hostname order of precedence for routes specified by the K8s Gateway API spec.
// https://gateway-api.sigs.k8s.io/v1alpha2/references/spec/#gateway.networking.k8s.io/v1alpha2.HTTPRouteRule
//
// Ordering prefers matches based on the largest number of:
//
//  1. characters in a matching non-wildcard hostname
//  2. characters in a matching hostname
//  3. characters in a matching path
//  4. header matches
//  5. query param matches
//
// The hostname-specific comparison (1+2) occur in Envoy outside of our control:
// https://github.com/envoyproxy/envoy/blob/5c4d4bd957f9402eca80bef82e7cc3ae714e04b4/source/common/router/config_impl.cc#L1645-L1682
func compareHTTPRules(ruleA, ruleB structs.HTTPMatch) bool {
	if len(ruleA.Path.Value) != len(ruleB.Path.Value) {
		return len(ruleA.Path.Value) > len(ruleB.Path.Value)
	}
	if len(ruleA.Headers) != len(ruleB.Headers) {
		return len(ruleA.Headers) > len(ruleB.Headers)
	}
	return len(ruleA.Query) > len(ruleB.Query)
}

func httpServiceDefault(entry structs.ConfigEntry, meta map[string]string) *structs.ServiceConfigEntry {
	return &structs.ServiceConfigEntry{
		Kind:           structs.ServiceDefaults,
		Name:           entry.GetName(),
		Protocol:       "http",
		Meta:           meta,
		EnterpriseMeta: *entry.GetEnterpriseMeta(),
	}
}

func synthesizeHTTPRouteDiscoveryChain(route structs.HTTPRouteConfigEntry) (structs.IngressService, *structs.ServiceRouterConfigEntry, []*structs.ServiceSplitterConfigEntry, []*structs.ServiceConfigEntry) {
	meta := route.GetMeta()
	splitters := []*structs.ServiceSplitterConfigEntry{}
	defaults := []*structs.ServiceConfigEntry{}

	router, splits, upstreamDefaults := httpRouteToDiscoveryChain(route)
	serviceDefault := httpServiceDefault(router, meta)
	defaults = append(defaults, serviceDefault)
	for _, split := range splits {
		splitters = append(splitters, split)
		if split.Name != serviceDefault.Name {
			defaults = append(defaults, httpServiceDefault(split, meta))
		}
	}
	defaults = append(defaults, upstreamDefaults...)

	ingress := structs.IngressService{
		Name:           router.Name,
		Hosts:          route.Hostnames,
		Meta:           route.Meta,
		EnterpriseMeta: route.EnterpriseMeta,
	}

	return ingress, router, splitters, defaults
}

func httpRouteToDiscoveryChain(route structs.HTTPRouteConfigEntry) (*structs.ServiceRouterConfigEntry, []*structs.ServiceSplitterConfigEntry, []*structs.ServiceConfigEntry) {
	router := &structs.ServiceRouterConfigEntry{
		Kind:           structs.ServiceRouter,
		Name:           route.GetName(),
		Meta:           route.GetMeta(),
		EnterpriseMeta: route.EnterpriseMeta,
	}
	var splitters []*structs.ServiceSplitterConfigEntry
	var defaults []*structs.ServiceConfigEntry

	for idx, rule := range route.Rules {
		requestModifier := httpRouteFiltersToServiceRouteHeaderModifier(rule.Filters.Headers)
		responseModifier := httpRouteFiltersToServiceRouteHeaderModifier(rule.ResponseFilters.Headers)
		prefixRewrite := httpRouteFiltersToDestinationPrefixRewrite(rule.Filters.URLRewrite)

		var destination structs.ServiceRouteDestination
		if len(rule.Services) == 1 {
			service := rule.Services[0]

			servicePrefixRewrite := httpRouteFiltersToDestinationPrefixRewrite(service.Filters.URLRewrite)
			if service.Filters.URLRewrite == nil {
				servicePrefixRewrite = prefixRewrite
			}
			serviceModifier := httpRouteFiltersToServiceRouteHeaderModifier(service.Filters.Headers)
			requestModifier.Add = mergeMaps(requestModifier.Add, serviceModifier.Add)
			requestModifier.Set = mergeMaps(requestModifier.Set, serviceModifier.Set)
			requestModifier.Remove = append(requestModifier.Remove, serviceModifier.Remove...)

			destination.Service = service.Name
			destination.Namespace = service.NamespaceOrDefault()
			destination.Partition = service.PartitionOrDefault()
			destination.PrefixRewrite = servicePrefixRewrite
			destination.RequestHeaders = requestModifier
			destination.ResponseHeaders = responseModifier

			// since we have already validated the protocol elsewhere, we
			// create a new service defaults here to make sure we pass validation
			defaults = append(defaults, &structs.ServiceConfigEntry{
				Kind:           structs.ServiceDefaults,
				Name:           service.Name,
				Protocol:       "http",
				EnterpriseMeta: service.EnterpriseMeta,
			})
		} else {
			// create a virtual service to split
			destination.Service = fmt.Sprintf("%s-%d", route.GetName(), idx)
			destination.Namespace = route.NamespaceOrDefault()
			destination.Partition = route.PartitionOrDefault()
			destination.PrefixRewrite = prefixRewrite
			destination.RequestHeaders = requestModifier
			destination.ResponseHeaders = responseModifier

			splitter := &structs.ServiceSplitterConfigEntry{
				Kind:           structs.ServiceSplitter,
				Name:           destination.Service,
				Splits:         []structs.ServiceSplit{},
				Meta:           route.GetMeta(),
				EnterpriseMeta: route.EnterpriseMeta,
			}

			totalWeight := 0
			for _, service := range rule.Services {
				totalWeight += service.Weight
			}

			for _, service := range rule.Services {
				if service.Weight == 0 {
					continue
				}

				modifier := httpRouteFiltersToServiceRouteHeaderModifier(service.Filters.Headers)

				weightPercentage := float32(service.Weight) / float32(totalWeight)
				split := structs.ServiceSplit{
					RequestHeaders: modifier,
					Weight:         weightPercentage * 100,
				}
				split.Service = service.Name
				split.Namespace = service.NamespaceOrDefault()
				split.Partition = service.PartitionOrDefault()
				splitter.Splits = append(splitter.Splits, split)

				// since we have already validated the protocol elsewhere, we
				// create a new service defaults here to make sure we pass validation
				defaults = append(defaults, &structs.ServiceConfigEntry{
					Kind:           structs.ServiceDefaults,
					Name:           service.Name,
					Protocol:       "http",
					EnterpriseMeta: service.EnterpriseMeta,
				})
			}
			if len(splitter.Splits) > 0 {
				splitters = append(splitters, splitter)
			}
		}

		if rule.Filters.RetryFilter != nil {
			if rule.Filters.RetryFilter.NumRetries != nil {
				destination.NumRetries = *rule.Filters.RetryFilter.NumRetries
			}
			if rule.Filters.RetryFilter.RetryOnConnectFailure != nil {
				destination.RetryOnConnectFailure = *rule.Filters.RetryFilter.RetryOnConnectFailure
			}

			if len(rule.Filters.RetryFilter.RetryOn) > 0 {
				destination.RetryOn = rule.Filters.RetryFilter.RetryOn
			}

			if len(rule.Filters.RetryFilter.RetryOnStatusCodes) > 0 {
				destination.RetryOnStatusCodes = rule.Filters.RetryFilter.RetryOnStatusCodes
			}
		}

		if rule.Filters.TimeoutFilter != nil {
			destination.IdleTimeout = rule.Filters.TimeoutFilter.IdleTimeout
			destination.RequestTimeout = rule.Filters.TimeoutFilter.RequestTimeout
		}

		// for each match rule a ServiceRoute is created for the service-router
		// if there are no rules a single route with the destination is set
		if len(rule.Matches) == 0 {
			router.Routes = append(router.Routes, structs.ServiceRoute{Destination: &destination})
		}

		for _, match := range rule.Matches {
			router.Routes = append(router.Routes, structs.ServiceRoute{
				Match:       &structs.ServiceRouteMatch{HTTP: httpRouteMatchToServiceRouteHTTPMatch(match)},
				Destination: &destination,
			})
		}

	}

	return router, splitters, defaults
}

func httpRouteFiltersToDestinationPrefixRewrite(rewrite *structs.URLRewrite) string {
	if rewrite == nil {
		return ""
	}
	return rewrite.Path
}

// httpRouteFiltersToServiceRouteHeaderModifier will consolidate a list of HTTP filters
// into a single set of header modifications for Consul to make as a request passes through.
func httpRouteFiltersToServiceRouteHeaderModifier(filters []structs.HTTPHeaderFilter) *structs.HTTPHeaderModifiers {
	modifier := &structs.HTTPHeaderModifiers{
		Add: make(map[string]string),
		Set: make(map[string]string),
	}
	for _, filter := range filters {
		// If we have multiple filters specified, then we can potentially clobber
		// "Add" and "Set" here -- as far as K8S gateway spec is concerned, this
		// is all implementation-specific behavior and undefined by the spec.
		modifier.Add = mergeMaps(modifier.Add, filter.Add)
		modifier.Set = mergeMaps(modifier.Set, filter.Set)
		modifier.Remove = append(modifier.Remove, filter.Remove...)
	}
	return modifier
}

func mergeMaps(a, b map[string]string) map[string]string {
	for k, v := range b {
		a[k] = v
	}
	return a
}

func httpRouteMatchToServiceRouteHTTPMatch(match structs.HTTPMatch) *structs.ServiceRouteHTTPMatch {
	var consulMatch structs.ServiceRouteHTTPMatch
	switch match.Path.Match {
	case structs.HTTPPathMatchExact:
		consulMatch.PathExact = match.Path.Value
	case structs.HTTPPathMatchPrefix:
		consulMatch.PathPrefix = match.Path.Value
	case structs.HTTPPathMatchRegularExpression:
		consulMatch.PathRegex = match.Path.Value
	}

	for _, header := range match.Headers {
		switch header.Match {
		case structs.HTTPHeaderMatchExact:
			consulMatch.Header = append(consulMatch.Header, structs.ServiceRouteHTTPMatchHeader{
				Name:  header.Name,
				Exact: header.Value,
			})
		case structs.HTTPHeaderMatchPrefix:
			consulMatch.Header = append(consulMatch.Header, structs.ServiceRouteHTTPMatchHeader{
				Name:   header.Name,
				Prefix: header.Value,
			})
		case structs.HTTPHeaderMatchSuffix:
			consulMatch.Header = append(consulMatch.Header, structs.ServiceRouteHTTPMatchHeader{
				Name:   header.Name,
				Suffix: header.Value,
			})
		case structs.HTTPHeaderMatchPresent:
			consulMatch.Header = append(consulMatch.Header, structs.ServiceRouteHTTPMatchHeader{
				Name:    header.Name,
				Present: true,
			})
		case structs.HTTPHeaderMatchRegularExpression:
			consulMatch.Header = append(consulMatch.Header, structs.ServiceRouteHTTPMatchHeader{
				Name:  header.Name,
				Regex: header.Value,
			})
		}
	}

	for _, query := range match.Query {
		switch query.Match {
		case structs.HTTPQueryMatchExact:
			consulMatch.QueryParam = append(consulMatch.QueryParam, structs.ServiceRouteHTTPMatchQueryParam{
				Name:  query.Name,
				Exact: query.Value,
			})
		case structs.HTTPQueryMatchPresent:
			consulMatch.QueryParam = append(consulMatch.QueryParam, structs.ServiceRouteHTTPMatchQueryParam{
				Name:    query.Name,
				Present: true,
			})
		case structs.HTTPQueryMatchRegularExpression:
			consulMatch.QueryParam = append(consulMatch.QueryParam, structs.ServiceRouteHTTPMatchQueryParam{
				Name:  query.Name,
				Regex: query.Value,
			})
		}
	}

	switch match.Method {
	case structs.HTTPMatchMethodConnect:
		consulMatch.Methods = append(consulMatch.Methods, "CONNECT")
	case structs.HTTPMatchMethodDelete:
		consulMatch.Methods = append(consulMatch.Methods, "DELETE")
	case structs.HTTPMatchMethodGet:
		consulMatch.Methods = append(consulMatch.Methods, "GET")
	case structs.HTTPMatchMethodHead:
		consulMatch.Methods = append(consulMatch.Methods, "HEAD")
	case structs.HTTPMatchMethodOptions:
		consulMatch.Methods = append(consulMatch.Methods, "OPTIONS")
	case structs.HTTPMatchMethodPatch:
		consulMatch.Methods = append(consulMatch.Methods, "PATCH")
	case structs.HTTPMatchMethodPost:
		consulMatch.Methods = append(consulMatch.Methods, "POST")
	case structs.HTTPMatchMethodPut:
		consulMatch.Methods = append(consulMatch.Methods, "PUT")
	case structs.HTTPMatchMethodTrace:
		consulMatch.Methods = append(consulMatch.Methods, "TRACE")
	}

	return &consulMatch
}

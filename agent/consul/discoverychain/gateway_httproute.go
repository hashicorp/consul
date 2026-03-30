// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discoverychain

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

const maxComposedRoutes = 256

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

func synthesizeHTTPRouteDiscoveryChain(route structs.HTTPRouteConfigEntry, serviceRouters map[structs.ServiceName][]*structs.ServiceRoute) (structs.IngressService, *structs.ServiceRouterConfigEntry, []*structs.ServiceSplitterConfigEntry, []*structs.ServiceConfigEntry) {
	meta := route.GetMeta()
	splitters := []*structs.ServiceSplitterConfigEntry{}
	defaults := []*structs.ServiceConfigEntry{}

	router, splits, upstreamDefaults := httpRouteToDiscoveryChain(route, serviceRouters)
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

func httpRouteToDiscoveryChain(route structs.HTTPRouteConfigEntry, serviceRouters map[structs.ServiceName][]*structs.ServiceRoute) (*structs.ServiceRouterConfigEntry, []*structs.ServiceSplitterConfigEntry, []*structs.ServiceConfigEntry) {
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

			// Merge service request header modifier(s) onto route rule modifiers
			// Note: Removals for the same header may exist on the rule + the service and
			//   will result in idempotent duplicate values in the modifier w/ service coming last
			serviceRequestModifier := httpRouteFiltersToServiceRouteHeaderModifier(service.Filters.Headers)
			requestModifier.Add = mergeMaps(requestModifier.Add, serviceRequestModifier.Add)
			requestModifier.Set = mergeMaps(requestModifier.Set, serviceRequestModifier.Set)
			requestModifier.Remove = append(requestModifier.Remove, serviceRequestModifier.Remove...)

			// Merge service response header modifier(s) onto route rule modifiers
			// Note: Removals for the same header may exist on the rule + the service and
			//   will result in idempotent duplicate values in the modifier w/ service coming last
			serviceResponseModifier := httpRouteFiltersToServiceRouteHeaderModifier(service.ResponseFilters.Headers)
			responseModifier.Add = mergeMaps(responseModifier.Add, serviceResponseModifier.Add)
			responseModifier.Set = mergeMaps(responseModifier.Set, serviceResponseModifier.Set)
			responseModifier.Remove = append(responseModifier.Remove, serviceResponseModifier.Remove...)

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

			applyHTTPRouteFilters(&destination, rule)

			httpMatches := rule.Matches
			if len(httpMatches) == 0 {
				httpMatches = []structs.HTTPMatch{{
					Path: structs.HTTPPathMatch{
						Match: structs.HTTPPathMatchPrefix,
						Value: "/",
					},
				}}
			}

			serviceRouterRoutes := lookupServiceRouterRules(serviceRouters, service)
			if shouldComposeServiceRouter(httpMatches, serviceRouterRoutes) {
				for _, match := range httpMatches {
					httpMatch := &structs.ServiceRouteMatch{HTTP: httpRouteMatchToServiceRouteHTTPMatch(match)}
					composed := false

					for _, svcRoute := range serviceRouterRoutes {
						mergedMatch, ok := mergeServiceRouteMatch(httpMatch, svcRoute.Match)
						if !ok {
							continue
						}
						mergedDest := mergeServiceRouteDestination(&destination, svcRoute.Destination)
						router.Routes = append(router.Routes, structs.ServiceRoute{
							Match:       mergedMatch,
							Destination: mergedDest,
						})
						composed = true
					}

					if !composed {
						router.Routes = append(router.Routes, structs.ServiceRoute{
							Match:       httpMatch,
							Destination: &destination,
						})
					}
				}
			} else {
				for _, match := range httpMatches {
					router.Routes = append(router.Routes, structs.ServiceRoute{
						Match:       &structs.ServiceRouteMatch{HTTP: httpRouteMatchToServiceRouteHTTPMatch(match)},
						Destination: &destination,
					})
				}
			}

			continue
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

		applyHTTPRouteFilters(&destination, rule)

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

func applyHTTPRouteFilters(destination *structs.ServiceRouteDestination, rule structs.HTTPRouteRule) {
	if rule.Filters.RetryFilter != nil {
		destination.NumRetries = rule.Filters.RetryFilter.NumRetries
		destination.RetryOnConnectFailure = rule.Filters.RetryFilter.RetryOnConnectFailure

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
}

func lookupServiceRouterRules(serviceRouters map[structs.ServiceName][]*structs.ServiceRoute, service structs.HTTPService) []*structs.ServiceRoute {
	if len(serviceRouters) == 0 {
		return nil
	}
	return serviceRouters[service.ServiceName()]
}

func shouldComposeServiceRouter(httpMatches []structs.HTTPMatch, serviceRoutes []*structs.ServiceRoute) bool {
	if len(serviceRoutes) == 0 {
		return false
	}
	if len(httpMatches) == 0 {
		return true
	}
	if len(serviceRoutes) > maxComposedRoutes/len(httpMatches) {
		return false
	}
	return true
}

func mergeServiceRouteMatch(httpMatch, svcMatch *structs.ServiceRouteMatch) (*structs.ServiceRouteMatch, bool) {
	if httpMatch == nil || httpMatch.IsEmpty() {
		return cloneServiceRouteMatch(svcMatch), true
	}
	if svcMatch == nil || svcMatch.IsEmpty() {
		return cloneServiceRouteMatch(httpMatch), true
	}

	mergedHTTP, ok := mergeServiceRouteHTTPMatch(httpMatch.HTTP, svcMatch.HTTP)
	if !ok {
		return nil, false
	}
	return &structs.ServiceRouteMatch{HTTP: mergedHTTP}, true
}

func mergeServiceRouteHTTPMatch(a, b *structs.ServiceRouteHTTPMatch) (*structs.ServiceRouteHTTPMatch, bool) {
	if a == nil || a.IsEmpty() {
		return cloneServiceRouteHTTPMatch(b), true
	}
	if b == nil || b.IsEmpty() {
		return cloneServiceRouteHTTPMatch(a), true
	}

	merged := cloneServiceRouteHTTPMatch(a)
	if merged == nil {
		merged = &structs.ServiceRouteHTTPMatch{}
	}

	path, ok := mergePathMatch(a, b)
	if !ok {
		return nil, false
	}
	merged.PathExact = path.pathExact
	merged.PathPrefix = path.pathPrefix
	merged.PathRegex = path.pathRegex
	merged.CaseInsensitive = a.CaseInsensitive && b.CaseInsensitive

	merged.Header = append(append([]structs.ServiceRouteHTTPMatchHeader{}, a.Header...), b.Header...)
	merged.QueryParam = append(append([]structs.ServiceRouteHTTPMatchQueryParam{}, a.QueryParam...), b.QueryParam...)

	merged.Methods = mergeHTTPMethods(a.Methods, b.Methods)
	if len(merged.Methods) == 0 && len(a.Methods) > 0 && len(b.Methods) > 0 {
		return nil, false
	}

	return merged, true
}

type mergedPath struct {
	pathExact  string
	pathPrefix string
	pathRegex  string
}

func mergePathMatch(a, b *structs.ServiceRouteHTTPMatch) (mergedPath, bool) {
	aPath, aOK := extractPathMatch(a)
	bPath, bOK := extractPathMatch(b)

	if !aOK && !bOK {
		return mergedPath{}, true
	}
	if !aOK {
		return mergedPath{pathExact: b.PathExact, pathPrefix: b.PathPrefix, pathRegex: b.PathRegex}, true
	}
	if !bOK {
		return mergedPath{pathExact: a.PathExact, pathPrefix: a.PathPrefix, pathRegex: a.PathRegex}, true
	}

	switch aPath.kind {
	case "exact":
		switch bPath.kind {
		case "exact":
			if aPath.value != bPath.value {
				return mergedPath{}, false
			}
			return mergedPath{pathExact: aPath.value}, true
		case "prefix":
			if strings.HasPrefix(aPath.value, bPath.value) {
				return mergedPath{pathExact: aPath.value}, true
			}
			return mergedPath{}, false
		case "regex":
			if aPath.value == bPath.value {
				return mergedPath{pathExact: aPath.value}, true
			}
			return mergedPath{}, false
		}
	case "prefix":
		switch bPath.kind {
		case "exact":
			if strings.HasPrefix(bPath.value, aPath.value) {
				return mergedPath{pathExact: bPath.value}, true
			}
			return mergedPath{}, false
		case "prefix":
			if strings.HasPrefix(aPath.value, bPath.value) {
				return mergedPath{pathPrefix: aPath.value}, true
			}
			if strings.HasPrefix(bPath.value, aPath.value) {
				return mergedPath{pathPrefix: bPath.value}, true
			}
			return mergedPath{}, false
		case "regex":
			if aPath.value == bPath.value {
				return mergedPath{pathPrefix: aPath.value}, true
			}
			return mergedPath{}, false
		}
	case "regex":
		switch bPath.kind {
		case "regex":
			if aPath.value != bPath.value {
				return mergedPath{}, false
			}
			return mergedPath{pathRegex: aPath.value}, true
		case "exact":
			if aPath.value == bPath.value {
				return mergedPath{pathExact: bPath.value}, true
			}
			return mergedPath{}, false
		case "prefix":
			if aPath.value == bPath.value {
				return mergedPath{pathPrefix: bPath.value}, true
			}
			return mergedPath{}, false
		}
	}

	return mergedPath{}, false
}

type pathMatch struct {
	kind  string
	value string
}

func extractPathMatch(m *structs.ServiceRouteHTTPMatch) (pathMatch, bool) {
	if m == nil {
		return pathMatch{}, false
	}
	switch {
	case m.PathExact != "":
		return pathMatch{kind: "exact", value: m.PathExact}, true
	case m.PathPrefix != "":
		return pathMatch{kind: "prefix", value: m.PathPrefix}, true
	case m.PathRegex != "":
		return pathMatch{kind: "regex", value: m.PathRegex}, true
	default:
		return pathMatch{}, false
	}
}

func mergeHTTPMethods(a, b []string) []string {
	if len(a) == 0 {
		return append([]string(nil), b...)
	}
	if len(b) == 0 {
		return append([]string(nil), a...)
	}
	set := make(map[string]struct{}, len(a))
	for _, method := range a {
		set[method] = struct{}{}
	}
	var out []string
	for _, method := range b {
		if _, ok := set[method]; ok {
			out = append(out, method)
		}
	}
	return out
}

func mergeServiceRouteDestination(httpDest, svcDest *structs.ServiceRouteDestination) *structs.ServiceRouteDestination {
	if svcDest == nil {
		return cloneServiceRouteDestination(httpDest)
	}
	merged := cloneServiceRouteDestination(svcDest)
	if httpDest == nil {
		return merged
	}

	if merged.Service == "" {
		merged.Service = httpDest.Service
	}
	if merged.ServiceSubset == "" {
		merged.ServiceSubset = httpDest.ServiceSubset
	}
	if merged.Namespace == "" {
		merged.Namespace = httpDest.Namespace
	}
	if merged.Partition == "" {
		merged.Partition = httpDest.Partition
	}

	if httpDest.PrefixRewrite != "" {
		merged.PrefixRewrite = httpDest.PrefixRewrite
	}
	if httpDest.RequestTimeout != 0 {
		merged.RequestTimeout = httpDest.RequestTimeout
	}
	if httpDest.IdleTimeout != 0 {
		merged.IdleTimeout = httpDest.IdleTimeout
	}
	if httpDest.NumRetries != 0 {
		merged.NumRetries = httpDest.NumRetries
	}
	if httpDest.RetryOnConnectFailure {
		merged.RetryOnConnectFailure = true
	}
	if len(httpDest.RetryOn) > 0 {
		merged.RetryOn = httpDest.RetryOn
	}
	if len(httpDest.RetryOnStatusCodes) > 0 {
		merged.RetryOnStatusCodes = httpDest.RetryOnStatusCodes
	}

	merged.RequestHeaders = mergeHeaderModifiers(merged.RequestHeaders, httpDest.RequestHeaders)
	merged.ResponseHeaders = mergeHeaderModifiers(merged.ResponseHeaders, httpDest.ResponseHeaders)

	return merged
}

func mergeHeaderModifiers(base, overlay *structs.HTTPHeaderModifiers) *structs.HTTPHeaderModifiers {
	if base == nil && overlay == nil {
		return nil
	}
	if base == nil {
		return cloneHeaderModifiers(overlay)
	}
	if overlay == nil {
		return cloneHeaderModifiers(base)
	}

	merged := &structs.HTTPHeaderModifiers{
		Add:    make(map[string]string),
		Set:    make(map[string]string),
		Remove: []string{},
	}

	for k, v := range base.Add {
		merged.Add[k] = v
	}
	for k, v := range base.Set {
		merged.Set[k] = v
	}
	merged.Remove = append(merged.Remove, base.Remove...)

	for k, v := range overlay.Add {
		merged.Add[k] = v
	}
	for k, v := range overlay.Set {
		merged.Set[k] = v
	}
	merged.Remove = append(merged.Remove, overlay.Remove...)

	return merged
}

func cloneServiceRouteMatch(in *structs.ServiceRouteMatch) *structs.ServiceRouteMatch {
	if in == nil {
		return nil
	}
	return &structs.ServiceRouteMatch{HTTP: cloneServiceRouteHTTPMatch(in.HTTP)}
}

func cloneServiceRouteHTTPMatch(in *structs.ServiceRouteHTTPMatch) *structs.ServiceRouteHTTPMatch {
	if in == nil {
		return nil
	}
	out := *in
	out.Header = append([]structs.ServiceRouteHTTPMatchHeader(nil), in.Header...)
	out.QueryParam = append([]structs.ServiceRouteHTTPMatchQueryParam(nil), in.QueryParam...)
	out.Methods = append([]string(nil), in.Methods...)
	return &out
}

func cloneServiceRouteDestination(in *structs.ServiceRouteDestination) *structs.ServiceRouteDestination {
	if in == nil {
		return nil
	}
	out := *in
	out.RequestHeaders = cloneHeaderModifiers(in.RequestHeaders)
	out.ResponseHeaders = cloneHeaderModifiers(in.ResponseHeaders)
	return &out
}

func cloneHeaderModifiers(in *structs.HTTPHeaderModifiers) *structs.HTTPHeaderModifiers {
	if in == nil {
		return nil
	}
	out := &structs.HTTPHeaderModifiers{
		Add:    make(map[string]string),
		Set:    make(map[string]string),
		Remove: append([]string(nil), in.Remove...),
	}
	for k, v := range in.Add {
		out.Add[k] = v
	}
	for k, v := range in.Set {
		out.Set[k] = v
	}
	return out
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

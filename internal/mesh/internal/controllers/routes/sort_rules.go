// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"sort"

	"github.com/oklog/ulid"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func gammaSortRouteRules(node *inputRouteNode) {
	switch {
	case resource.EqualType(node.RouteType, pbmesh.HTTPRouteType):
		gammaSortHTTPRouteRules(node.HTTPRules)
	case resource.EqualType(node.RouteType, pbmesh.GRPCRouteType):
		// TODO(rb): do a determinstic sort of something
	case resource.EqualType(node.RouteType, pbmesh.TCPRouteType):
		// TODO(rb): do a determinstic sort of something
	default:
		panic("impossible")
	}
}

func gammaSortHTTPRouteRules(rules []*pbmesh.ComputedHTTPRouteRule) {
	// First generate a parallel slice.
	sortable := &sortableHTTPRouteRules{
		rules:       rules,
		derivedInfo: make([]*derivedHTTPRouteRuleInfo, 0, len(rules)),
	}

	for _, rule := range rules {
		var sr derivedHTTPRouteRuleInfo
		for _, m := range rule.Matches {
			if m.Path != nil {
				switch m.Path.Type {
				case pbmesh.PathMatchType_PATH_MATCH_TYPE_EXACT:
					sr.hasPathExact = true
				case pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX:
					sr.hasPathPrefix = true
					v := len(m.Path.Value)
					if v > sr.pathPrefixLength {
						sr.pathPrefixLength = v
					}
				}
			}

			if m.Method != "" {
				sr.hasMethod = true
			}
			if v := len(m.Headers); v > sr.numHeaders {
				sr.numHeaders = v
			}
			if v := len(m.QueryParams); v > sr.numQueryParams {
				sr.numQueryParams = v
			}
		}
		sortable.derivedInfo = append(sortable.derivedInfo, &sr)
	}

	// Similar to
	// "agent/consul/discoverychain/gateway_httproute.go"
	// compareHTTPRules

	// Sort this by the GAMMA spec. We assume the caller has pre-sorted this
	// for tiebreakers based on resource metadata.

	sort.Stable(sortable)
}

type derivedHTTPRouteRuleInfo struct {
	// sortable fields extracted from route
	hasPathExact     bool
	hasPathPrefix    bool
	pathPrefixLength int
	hasMethod        bool
	numHeaders       int
	numQueryParams   int
}

type sortableHTTPRouteRules struct {
	rules       []*pbmesh.ComputedHTTPRouteRule
	derivedInfo []*derivedHTTPRouteRuleInfo
}

var _ sort.Interface = (*sortableHTTPRouteRules)(nil)

func (r *sortableHTTPRouteRules) Len() int { return len(r.rules) }

func (r *sortableHTTPRouteRules) Swap(i, j int) {
	r.rules[i], r.rules[j] = r.rules[j], r.rules[i]
	r.derivedInfo[i], r.derivedInfo[j] = r.derivedInfo[j], r.derivedInfo[i]
}

func (r *sortableHTTPRouteRules) Less(i, j int) bool {
	a := r.derivedInfo[i]
	b := r.derivedInfo[j]

	// (1) “Exact” path match.
	switch {
	case a.hasPathExact && b.hasPathExact:
		// NEXT TIE BREAK
	case a.hasPathExact && !b.hasPathExact:
		return true
	case !a.hasPathExact && b.hasPathExact:
		return false
	}

	// (2) “Prefix” path match with largest number of characters.
	switch {
	case a.hasPathPrefix && b.hasPathPrefix:
		if a.pathPrefixLength != b.pathPrefixLength {
			return a.pathPrefixLength > b.pathPrefixLength
		}
		// NEXT TIE BREAK
	case a.hasPathPrefix && !b.hasPathPrefix:
		return true
	case !a.hasPathPrefix && b.hasPathPrefix:
		return false
	}

	// (3) Method match.
	switch {
	case a.hasMethod && b.hasMethod:
		// NEXT TIE BREAK
	case a.hasMethod && !b.hasMethod:
		return true
	case !a.hasMethod && b.hasMethod:
		return false
	}

	// (4) Largest number of header matches.
	switch {
	case a.numHeaders == b.numHeaders:
		// NEXT TIE BREAK
	case a.numHeaders > b.numHeaders:
		return true
	case a.numHeaders < b.numHeaders:
		return false
	}

	// (5) Largest number of query param matches.
	return a.numQueryParams > b.numQueryParams
}

// gammaInitialSortWrappedRoutes will sort the original inputs by the
// resource-envelope-only fields before we further stable sort by type-specific
// fields.
//
// If more than 1 route is provided the OriginalResource field must be set on
// all inputs (i.e. no synthetic default routes should be processed here).
func gammaInitialSortWrappedRoutes(routes []*inputRouteNode) {
	if len(routes) < 2 {
		return
	}

	// First sort the input routes by the final criteria, so we can let the
	// stable sort take care of the ultimate tiebreakers.
	sort.Slice(routes, func(i, j int) bool {
		var (
			resA = routes[i].OriginalResource
			resB = routes[j].OriginalResource
		)

		if resA == nil || resB == nil {
			panic("some provided nodes lacked original resources")
		}

		var (
			genA = resA.Generation
			genB = resB.Generation
		)

		// (END-1) The oldest Route based on creation timestamp.
		//
		// Because these are ULIDs, we should be able to lexicographically sort
		// them to determine the oldest, but we also need to have a further
		// tiebreaker AFTER per-gamma so we cannot.
		aULID, aErr := ulid.Parse(genA)
		bULID, bErr := ulid.Parse(genB)
		if aErr == nil && bErr == nil {
			aTime := aULID.Time()
			bTime := bULID.Time()

			switch {
			case aTime < bTime:
				return true
			case aTime > bTime:
				return false
			default:
				// NEXT TIE BREAK
			}
		}

		// (END-2) The Route appearing first in alphabetical order by “{namespace}/{name}”.
		var (
			tenancyA = resA.Id.Tenancy
			tenancyB = resB.Id.Tenancy

			nsA = tenancyA.Namespace
			nsB = tenancyB.Namespace
		)

		if nsA == "" {
			nsA = "default"
		}
		if nsB == "" {
			nsB = "default"
		}
		switch {
		case nsA < nsB:
			return true
		case nsA > nsB:
			return false
		default:
			// NEXT TIE BREAK
		}

		return resA.Id.Name < resB.Id.Name

		// We get this for free b/c of the stable sort.
		//
		// If ties still exist within an HTTPRoute, matching precedence MUST
		// be granted to the FIRST matching rule (in list order) with a match
		// meeting the above criteria.
	})
}

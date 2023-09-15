// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"fmt"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// GenerateComputedRoutes walks a set of related resources and assembles them
// into one effective easy-to-consume ComputedRoutes result.
//
// Any status conditions generated during traversal can be queued for
// persistence using the PendingStatuses map.
//
// This should not internally generate, nor return any errors.
func GenerateComputedRoutes(
	related *loader.RelatedResources,
	pending PendingStatuses,
) []*ComputedRoutesResult {
	out := make([]*ComputedRoutesResult, 0, len(related.ComputedRoutesList))
	for _, computedRoutesID := range related.ComputedRoutesList {
		out = append(out, compile(related, computedRoutesID, pending))
	}
	return out
}

type ComputedRoutesResult struct {
	// ID is always required.
	ID *pbresource.ID
	// OwnerID is only required on upserts.
	OwnerID *pbresource.ID
	// Data being empty means delete if exists.
	Data *pbmesh.ComputedRoutes
}

func compile(
	related *loader.RelatedResources,
	computedRoutesID *pbresource.ID,
	pending PendingStatuses,
) *ComputedRoutesResult {
	// There is one computed routes resource for the entire service (perfect name alignment).
	//
	// All ports are embedded within.

	parentServiceID := &pbresource.ID{
		Type:    catalog.ServiceType,
		Tenancy: computedRoutesID.Tenancy,
		Name:    computedRoutesID.Name,
	}

	parentServiceRef := resource.Reference(parentServiceID, "")

	parentServiceDec := related.GetService(parentServiceID)
	if parentServiceDec == nil {
		return &ComputedRoutesResult{
			ID:   computedRoutesID,
			Data: nil, // returning nil signals a delete is requested
		}
	}
	parentServiceID = parentServiceDec.Resource.Id // get ULID out of it

	var (
		inMesh               = false
		allowedPortProtocols = make(map[string]pbcatalog.Protocol)
	)
	for _, port := range parentServiceDec.Data.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			inMesh = true
			continue // skip
		}
		allowedPortProtocols[port.TargetPort] = port.Protocol
	}

	if !inMesh {
		return &ComputedRoutesResult{
			ID:   computedRoutesID,
			Data: nil, // returning nil signals a delete is requested
		}
	}

	computedRoutes := &pbmesh.ComputedRoutes{
		PortedConfigs: make(map[string]*pbmesh.ComputedPortRoutes),
	}

	// Visit all of the routes relevant to this computed routes.
	routeNodesByPort := make(map[string][]*inputRouteNode)
	related.WalkRoutesForParentRef(parentServiceRef, func(
		rk resource.ReferenceKey,
		res *pbresource.Resource,
		xroute types.XRouteData,
	) {
		var (
			ports          []string
			wildcardedPort bool
		)
		for _, ref := range xroute.GetParentRefs() {
			if resource.ReferenceOrIDMatch(ref.Ref, parentServiceRef) {
				if ref.Port == "" {
					wildcardedPort = true
					break
				}
				if _, ok := allowedPortProtocols[ref.Port]; ok {
					ports = append(ports, ref.Port)
				}
			}
		}

		// Do a port explosion.
		if wildcardedPort {
			ports = nil
			for port := range allowedPortProtocols {
				ports = append(ports, port)
			}
		}

		if len(ports) == 0 {
			return // not relevant to this computed routes
		}

		for _, port := range ports {
			if port == "" {
				panic("impossible to have an empty port here")
			}

			var node *inputRouteNode
			switch route := xroute.(type) {
			case *pbmesh.HTTPRoute:
				node = compileHTTPRouteNode(port, res, route, related)
			case *pbmesh.GRPCRoute:
				node = compileGRPCRouteNode(port, res, route, related)
			case *pbmesh.TCPRoute:
				node = compileTCPRouteNode(port, res, route, related)
			default:
				panic(fmt.Sprintf("unexpected xroute type: %T", xroute))
			}

			routeNodesByPort[node.ParentPort] = append(routeNodesByPort[node.ParentPort], node)
		}
	})

	// Fill in defaults where there was no xroute defined at all.
	for port, protocol := range allowedPortProtocols {
		if _, ok := routeNodesByPort[port]; !ok {
			var typ *pbresource.Type

			// enumcover:pbcatalog.Protocol
			switch protocol {
			case pbcatalog.Protocol_PROTOCOL_HTTP2:
				typ = types.HTTPRouteType
			case pbcatalog.Protocol_PROTOCOL_HTTP:
				typ = types.HTTPRouteType
			case pbcatalog.Protocol_PROTOCOL_GRPC:
				typ = types.GRPCRouteType
			case pbcatalog.Protocol_PROTOCOL_TCP:
				typ = types.TCPRouteType
			case pbcatalog.Protocol_PROTOCOL_MESH:
				fallthrough // to default
			case pbcatalog.Protocol_PROTOCOL_UNSPECIFIED:
				fallthrough // to default
			default:
				continue // not possible
			}

			routeNode := createDefaultRouteNode(parentServiceRef, port, typ)

			routeNodesByPort[port] = append(routeNodesByPort[port], routeNode)
		}
	}

	// First sort the input routes by the final criteria, so we can let the
	// stable sort take care of the ultimate tiebreakers.
	for port, routeNodes := range routeNodesByPort {
		gammaInitialSortWrappedRoutes(routeNodes)

		// Now that they are sorted by age and name, we can figure out which
		// xRoute should apply to each port (the first).
		var top *inputRouteNode
		for i, routeNode := range routeNodes {
			if i == 0 {
				top = routeNode
				continue
			}
			if top.RouteType != routeNode.RouteType {
				// This should only happen with user-provided ones, since we
				// would never have two synthetic default routes at once.
				res := routeNode.OriginalResource
				if res != nil {
					pending.AddConditions(resource.NewReferenceKey(res.Id), res, []*pbresource.Condition{
						ConditionConflictNotBoundToParentRef(
							parentServiceRef,
							port,
							top.RouteType,
						),
					})
				}
				continue
			}
			top.AppendRulesFrom(routeNode)
			top.AddTargetsFrom(routeNode)
		}

		// Clear this field as it's no longer used and doesn't make sense once
		// this represents multiple xRoutes or defaults.
		top.OriginalResource = nil

		// Now we can do the big sort.
		gammaSortRouteRules(top)

		// Inject catch-all rules to ensure stray requests will explicitly be blackholed.
		if !top.Default {
			if types.IsTCPRouteType(top.RouteType) {
				if len(top.TCPRules) == 0 {
					// User requests traffic blackhole.
					appendDefaultRouteNode(top, types.NullRouteBackend)
				}
			} else {
				// There are no match criteria on a TCPRoute, so never a need
				// to add a catch-all.
				appendDefaultRouteNode(top, types.NullRouteBackend)
			}
		}

		parentRef := &pbmesh.ParentReference{
			Ref:  parentServiceRef,
			Port: port,
		}

		mc := &pbmesh.ComputedPortRoutes{
			UsingDefaultConfig: top.Default,
			ParentRef:          parentRef,
			Protocol:           allowedPortProtocols[port],
			Targets:            top.NewTargets,
		}

		switch {
		case resource.EqualType(top.RouteType, types.HTTPRouteType):
			if mc.Protocol == pbcatalog.Protocol_PROTOCOL_TCP {
				// The rest are HTTP-like
				mc.Protocol = pbcatalog.Protocol_PROTOCOL_HTTP
			}
			mc.Config = &pbmesh.ComputedPortRoutes_Http{
				Http: &pbmesh.ComputedHTTPRoute{
					Rules: top.HTTPRules,
				},
			}
		case resource.EqualType(top.RouteType, types.GRPCRouteType):
			mc.Protocol = pbcatalog.Protocol_PROTOCOL_GRPC
			mc.Config = &pbmesh.ComputedPortRoutes_Grpc{
				Grpc: &pbmesh.ComputedGRPCRoute{
					Rules: top.GRPCRules,
				},
			}
		case resource.EqualType(top.RouteType, types.TCPRouteType):
			mc.Protocol = pbcatalog.Protocol_PROTOCOL_TCP
			mc.Config = &pbmesh.ComputedPortRoutes_Tcp{
				Tcp: &pbmesh.ComputedTCPRoute{
					Rules: top.TCPRules,
				},
			}
		default:
			panic("impossible")
		}

		computedRoutes.PortedConfigs[port] = mc

		for _, details := range mc.Targets {
			svcRef := details.BackendRef.Ref

			svc := related.GetService(svcRef)
			failoverPolicy := related.GetFailoverPolicyForService(svcRef)
			destPolicy := related.GetDestinationPolicyForService(svcRef)

			if svc == nil {
				panic("impossible at this point; should already have been handled before getting here")
			}

			// Find the destination proxy's port.
			//
			// Endpoints refs will need to route to mesh port instead of the
			// destination port as that is the port of the destination's proxy.
			//
			// Note: we will always find a port here because we only add targets that have
			// mesh ports above in shouldRouteTrafficToBackend().
			for _, port := range svc.Data.Ports {
				if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
					details.MeshPort = port.TargetPort
				}
			}

			if failoverPolicy != nil {
				simpleFailoverPolicy := catalog.SimplifyFailoverPolicy(svc.Data, failoverPolicy.Data)
				portFailoverConfig, ok := simpleFailoverPolicy.PortConfigs[details.BackendRef.Port]
				if ok {
					details.FailoverConfig = portFailoverConfig
				}
			}
			if destPolicy != nil {
				portDestConfig, ok := destPolicy.Data.PortConfigs[details.BackendRef.Port]
				if ok {
					details.DestinationConfig = portDestConfig
				}
			}
		}

		computedRoutes.PortedConfigs[port] = mc
	}

	return &ComputedRoutesResult{
		ID:      computedRoutesID,
		OwnerID: parentServiceID,
		Data:    computedRoutes,
	}
}

func compileHTTPRouteNode(
	port string,
	res *pbresource.Resource,
	route *pbmesh.HTTPRoute,
	serviceGetter serviceGetter,
) *inputRouteNode {
	route = protoClone(route)
	node := newInputRouteNode(port)

	node.RouteType = types.HTTPRouteType
	node.OriginalResource = res
	node.HTTPRules = make([]*pbmesh.ComputedHTTPRouteRule, 0, len(route.Rules))
	for _, rule := range route.Rules {
		irule := &pbmesh.ComputedHTTPRouteRule{
			Matches:     protoSliceClone(rule.Matches),
			Filters:     protoSliceClone(rule.Filters),
			BackendRefs: make([]*pbmesh.ComputedHTTPBackendRef, 0, len(rule.BackendRefs)),
			Timeouts:    rule.Timeouts,
			Retries:     rule.Retries,
		}

		// https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1beta1.HTTPRouteRule
		//
		// If no matches are specified, the default is a prefix path match
		// on “/”, which has the effect of matching every HTTP request.
		if len(irule.Matches) == 0 {
			irule.Matches = defaultHTTPRouteMatches()
		}
		for _, match := range irule.Matches {
			if match.Path == nil {
				// Path specifies a HTTP request path matcher. If this
				// field is not specified, a default prefix match on
				// the “/” path is provided.
				match.Path = defaultHTTPPathMatch()
			}
		}

		for _, backendRef := range rule.BackendRefs {
			// Infer port name from parent ref.
			if backendRef.BackendRef.Port == "" {
				backendRef.BackendRef.Port = port
			}

			var (
				backendTarget string
				backendSvc    = serviceGetter.GetService(backendRef.BackendRef.Ref)
			)
			if shouldRouteTrafficToBackend(backendSvc, backendRef.BackendRef) {
				details := &pbmesh.BackendTargetDetails{
					BackendRef: backendRef.BackendRef,
				}
				backendTarget = node.AddTarget(backendRef.BackendRef, details)
			} else {
				backendTarget = types.NullRouteBackend
			}
			ibr := &pbmesh.ComputedHTTPBackendRef{
				BackendTarget: backendTarget,
				Weight:        backendRef.Weight,
				Filters:       backendRef.Filters,
			}
			irule.BackendRefs = append(irule.BackendRefs, ibr)
		}

		node.HTTPRules = append(node.HTTPRules, irule)
	}

	return node
}

func compileGRPCRouteNode(
	port string,
	res *pbresource.Resource,
	route *pbmesh.GRPCRoute,
	serviceGetter serviceGetter,
) *inputRouteNode {
	route = protoClone(route)

	node := newInputRouteNode(port)

	node.RouteType = types.GRPCRouteType
	node.OriginalResource = res
	node.GRPCRules = make([]*pbmesh.ComputedGRPCRouteRule, 0, len(route.Rules))
	for _, rule := range route.Rules {
		irule := &pbmesh.ComputedGRPCRouteRule{
			Matches:     protoSliceClone(rule.Matches),
			Filters:     protoSliceClone(rule.Filters),
			BackendRefs: make([]*pbmesh.ComputedGRPCBackendRef, 0, len(rule.BackendRefs)),
			Timeouts:    rule.Timeouts,
			Retries:     rule.Retries,
		}

		// https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1alpha2.GRPCRouteRule
		//
		// If no matches are specified, the implementation MUST match every gRPC request.
		if len(irule.Matches) == 0 {
			irule.Matches = defaultGRPCRouteMatches()
		}

		for _, backendRef := range rule.BackendRefs {
			// Infer port name from parent ref.
			if backendRef.BackendRef.Port == "" {
				backendRef.BackendRef.Port = port
			}

			var (
				backendTarget string
				backendSvc    = serviceGetter.GetService(backendRef.BackendRef.Ref)
			)
			if shouldRouteTrafficToBackend(backendSvc, backendRef.BackendRef) {
				details := &pbmesh.BackendTargetDetails{
					BackendRef: backendRef.BackendRef,
				}
				backendTarget = node.AddTarget(backendRef.BackendRef, details)
			} else {
				backendTarget = types.NullRouteBackend
			}

			ibr := &pbmesh.ComputedGRPCBackendRef{
				BackendTarget: backendTarget,
				Weight:        backendRef.Weight,
				Filters:       backendRef.Filters,
			}
			irule.BackendRefs = append(irule.BackendRefs, ibr)
		}

		node.GRPCRules = append(node.GRPCRules, irule)
	}

	return node
}

func compileTCPRouteNode(
	port string,
	res *pbresource.Resource,
	route *pbmesh.TCPRoute,
	serviceGetter serviceGetter,
) *inputRouteNode {
	route = protoClone(route)

	node := newInputRouteNode(port)

	node.RouteType = types.TCPRouteType
	node.OriginalResource = res
	node.TCPRules = make([]*pbmesh.ComputedTCPRouteRule, 0, len(route.Rules))
	for _, rule := range route.Rules {
		irule := &pbmesh.ComputedTCPRouteRule{
			BackendRefs: make([]*pbmesh.ComputedTCPBackendRef, 0, len(rule.BackendRefs)),
		}

		// https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1alpha2.TCPRoute

		for _, backendRef := range rule.BackendRefs {
			// Infer port name from parent ref.
			if backendRef.BackendRef.Port == "" {
				backendRef.BackendRef.Port = port
			}

			var (
				backendTarget string
				backendSvc    = serviceGetter.GetService(backendRef.BackendRef.Ref)
			)
			if shouldRouteTrafficToBackend(backendSvc, backendRef.BackendRef) {
				details := &pbmesh.BackendTargetDetails{
					BackendRef: backendRef.BackendRef,
				}
				backendTarget = node.AddTarget(backendRef.BackendRef, details)
			} else {
				backendTarget = types.NullRouteBackend
			}

			ibr := &pbmesh.ComputedTCPBackendRef{
				BackendTarget: backendTarget,
				Weight:        backendRef.Weight,
			}
			irule.BackendRefs = append(irule.BackendRefs, ibr)
		}

		node.TCPRules = append(node.TCPRules, irule)
	}

	return node
}

func shouldRouteTrafficToBackend(backendSvc *types.DecodedService, backendRef *pbmesh.BackendReference) bool {
	if backendSvc == nil {
		return false
	}

	var (
		found  = false
		inMesh = false
	)
	for _, port := range backendSvc.Data.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			inMesh = true
			continue
		}
		if port.TargetPort == backendRef.Port {
			found = true
		}
	}

	return inMesh && found
}

func createDefaultRouteNode(
	parentServiceRef *pbresource.Reference,
	port string,
	typ *pbresource.Type,
) *inputRouteNode {
	// always add the parent as a possible target due to catch-all
	defaultBackendRef := &pbmesh.BackendReference{
		Ref:  parentServiceRef,
		Port: port,
	}

	routeNode := newInputRouteNode(port)

	defaultBackendTarget := routeNode.AddTarget(defaultBackendRef, &pbmesh.BackendTargetDetails{
		BackendRef: defaultBackendRef,
	})
	switch {
	case resource.EqualType(types.HTTPRouteType, typ):
		routeNode.RouteType = types.HTTPRouteType
		appendDefaultHTTPRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(types.GRPCRouteType, typ):
		routeNode.RouteType = types.GRPCRouteType
		appendDefaultGRPCRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(types.TCPRouteType, typ):
		fallthrough
	default:
		routeNode.RouteType = types.TCPRouteType
		appendDefaultTCPRouteRule(routeNode, defaultBackendTarget)
	}

	routeNode.Default = true
	return routeNode
}

func appendDefaultRouteNode(
	routeNode *inputRouteNode,
	defaultBackendTarget string,
) {
	switch {
	case resource.EqualType(types.HTTPRouteType, routeNode.RouteType):
		appendDefaultHTTPRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(types.GRPCRouteType, routeNode.RouteType):
		appendDefaultGRPCRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(types.TCPRouteType, routeNode.RouteType):
		fallthrough
	default:
		appendDefaultTCPRouteRule(routeNode, defaultBackendTarget)
	}
}

func appendDefaultHTTPRouteRule(
	routeNode *inputRouteNode,
	backendTarget string,
) {
	routeNode.HTTPRules = append(routeNode.HTTPRules, &pbmesh.ComputedHTTPRouteRule{
		Matches: defaultHTTPRouteMatches(),
		BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
			BackendTarget: backendTarget,
		}},
	})
}

func appendDefaultGRPCRouteRule(
	routeNode *inputRouteNode,
	backendTarget string,
) {
	routeNode.GRPCRules = append(routeNode.GRPCRules, &pbmesh.ComputedGRPCRouteRule{
		Matches: defaultGRPCRouteMatches(),
		BackendRefs: []*pbmesh.ComputedGRPCBackendRef{{
			BackendTarget: backendTarget,
		}},
	})
}

func appendDefaultTCPRouteRule(
	routeNode *inputRouteNode,
	backendTarget string,
) {
	routeNode.TCPRules = append(routeNode.TCPRules, &pbmesh.ComputedTCPRouteRule{
		BackendRefs: []*pbmesh.ComputedTCPBackendRef{{
			BackendTarget: backendTarget,
		}},
	})
}

func defaultHTTPRouteMatches() []*pbmesh.HTTPRouteMatch {
	return []*pbmesh.HTTPRouteMatch{{
		Path: defaultHTTPPathMatch(),
	}}
}

func defaultHTTPPathMatch() *pbmesh.HTTPPathMatch {
	return &pbmesh.HTTPPathMatch{
		Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
		Value: "/",
	}
}

func defaultGRPCRouteMatches() []*pbmesh.GRPCRouteMatch {
	return []*pbmesh.GRPCRouteMatch{{}}
}

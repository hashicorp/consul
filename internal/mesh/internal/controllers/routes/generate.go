// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"

	"github.com/hashicorp/go-hclog"

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
	ctx context.Context,
	loggerFor func(*pbresource.ID) hclog.Logger,
	related *loader.RelatedResources,
	pending PendingStatuses,
) []*ComputedRoutesResult {
	if loggerFor == nil {
		loggerFor = func(_ *pbresource.ID) hclog.Logger {
			return hclog.NewNullLogger()
		}
	}
	out := make([]*ComputedRoutesResult, 0, len(related.ComputedRoutesList))
	for _, computedRoutesID := range related.ComputedRoutesList {
		out = append(out, compile(ctx, loggerFor, related, computedRoutesID, pending))
	}
	return out
}

type ComputedRoutesResult struct {
	ID      *pbresource.ID
	OwnerID *pbresource.ID
	// If Data is empty it means delete if exists.
	Data *pbmesh.ComputedRoutes
}

func compile(
	ctx context.Context,
	loggerFor func(*pbresource.ID) hclog.Logger,
	related *loader.RelatedResources,
	computedRoutesID *pbresource.ID,
	pending PendingStatuses,
) *ComputedRoutesResult {
	logger := loggerFor(computedRoutesID)

	// There is one mesh config for the entire service (perfect name alignment).
	//
	// All ports are embedded within.

	parentServiceID := &pbresource.ID{
		Type:    catalog.ServiceType,
		Tenancy: computedRoutesID.Tenancy,
		Name:    computedRoutesID.Name,
	}

	parentServiceRef := resource.Reference(parentServiceID, "")

	parentServiceDec := related.GetService(parentServiceID)

	allowedPortProtocols := make(map[string]pbcatalog.Protocol)
	inMesh := false
	if parentServiceDec != nil {
		parentServiceID = parentServiceDec.Resource.Id // get ULID out of it

		for _, port := range parentServiceDec.Data.Ports {
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				inMesh = true
				continue // skip
			}
			allowedPortProtocols[port.TargetPort] = port.Protocol
		}
	}

	meshConfig := &pbmesh.ComputedRoutes{
		PortedConfigs: make(map[string]*pbmesh.ComputedPortRoutes),
	}

	// Visit all of the routes relevant to this mesh config.
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
				ports = append(ports, ref.Port)
				if ref.Port == "" {
					wildcardedPort = true
					break
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
			return // not relevant to this mesh config
		}

		for _, port := range ports {
			if port == "" {
				panic("impossible to have an empty port here")
			}

			// Check if the user provided port is actually valid.
			nullRouteTraffic := (parentServiceDec == nil)
			if _, ok := allowedPortProtocols[port]; !ok {
				nullRouteTraffic = true
			}

			var node *inputRouteNode
			switch route := xroute.(type) {
			case *pbmesh.HTTPRoute:
				if nullRouteTraffic {
					node = newInputRouteNode(port)
					setupDefaultHTTPRouteNode(node, types.NullRouteBackend)
				} else {
					node = compileHTTPRouteNode(parentServiceRef, port, res, route, related)
				}
			case *pbmesh.GRPCRoute:
				if nullRouteTraffic {
					node = newInputRouteNode(port)
					setupDefaultGRPCRouteNode(node, types.NullRouteBackend)
				} else {
					node = compileGRPCRouteNode(parentServiceRef, port, res, route, related)
				}
			case *pbmesh.TCPRoute:
				if nullRouteTraffic {
					node = newInputRouteNode(port)
					setupDefaultTCPRouteNode(node, types.NullRouteBackend)
				} else {
					node = compileTCPRouteNode(parentServiceRef, port, res, route, related)
				}
			default:
				return // unknown xroute type (impossible)
			}

			routeNodesByPort[node.ParentPort] = append(routeNodesByPort[node.ParentPort], node)
		}
	})

	// Fill in defaults where there was no xroute defined at all.
	for port, protocol := range allowedPortProtocols {
		if _, ok := routeNodesByPort[port]; !ok {
			var typ *pbresource.Type
			switch protocol {
			case pbcatalog.Protocol_PROTOCOL_HTTP2:
				typ = types.HTTPRouteType
			case pbcatalog.Protocol_PROTOCOL_HTTP:
				typ = types.HTTPRouteType
			case pbcatalog.Protocol_PROTOCOL_GRPC:
				typ = types.GRPCRouteType
			case pbcatalog.Protocol_PROTOCOL_TCP:
				typ = types.TCPRouteType
			default:
				typ = types.TCPRouteType
			}

			routeNode := createDefaultRouteNode(parentServiceRef, port, typ)

			routeNodesByPort[port] = append(routeNodesByPort[port], routeNode)
		}
	}

	// First sort the input routes by the final criteria, so we can let the
	// stable sort take care of the ultimate tiebreakers.
	for port, routeNodes := range routeNodesByPort {
		if port == "grpc" {
			logger.Info("RBOYER - print routes", "N", len(routeNodes), "r", routeNodes)
		}
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
				res := routeNode.OriginalResource()
				pending.AddConditions(resource.NewReferenceKey(res.Id), res, []*pbresource.Condition{
					ConditionConflictNotBoundToParentRef(
						parentServiceRef,
						port,
						top.RouteType,
					),
				})
				continue
			}
			top.AppendRulesFrom(routeNode)
			top.AddTargetsFrom(routeNode)
		}

		// TODO: default route values by port by protocol?

		// Now we can do the big sort.
		gammaSortRouteRules(top)

		// Inject catch-all rules to ensure stray requests will explicitly be blackholed.
		if !top.Default {
			if !types.IsTCPRouteType(top.RouteType) {
				// There are no match criteria on a TCPRoute, so never a need
				// to add a catch-all.
				appendDefaultRouteNode(top, types.NullRouteBackend)
			}
		}

		mc := &pbmesh.ComputedPortRoutes{
			UsingDefaultConfig: top.Default,
			Targets:            top.NewTargets,
		}
		parentRef := &pbmesh.ParentReference{
			Ref:  parentServiceRef,
			Port: port,
		}

		switch {
		case resource.EqualType(top.RouteType, types.HTTPRouteType):
			mc.Config = &pbmesh.ComputedPortRoutes_Http{
				Http: &pbmesh.InterpretedHTTPRoute{
					ParentRef: parentRef,
					Rules:     top.HTTPRules,
				},
			}
			// TODO
		case resource.EqualType(top.RouteType, types.GRPCRouteType):
			mc.Config = &pbmesh.ComputedPortRoutes_Grpc{
				Grpc: &pbmesh.InterpretedGRPCRoute{
					ParentRef: parentRef,
					Rules:     top.GRPCRules,
				},
			}
			// TODO
		case resource.EqualType(top.RouteType, types.TCPRouteType):
			mc.Config = &pbmesh.ComputedPortRoutes_Tcp{
				Tcp: &pbmesh.InterpretedTCPRoute{
					ParentRef: parentRef,
					Rules:     top.TCPRules,
				},
			}
			// TODO
		default:
			panic("impossible")
		}

		meshConfig.PortedConfigs[port] = mc

		// TODO: prune dead targets from targets map

		for _, details := range mc.Targets {
			svcRef := details.BackendRef.Ref

			svc := related.GetService(svcRef)
			failoverPolicy := related.GetFailoverPolicyForService(svcRef)
			destConfig := related.GetDestinationPolicy(svcRef)

			if svc == nil {
				panic("impossible at this point; should already have been handled before getting here")
			}
			details.Service = svc.Data

			if failoverPolicy != nil {
				details.FailoverPolicy = catalog.SimplifyFailoverPolicy(
					svc.Data,
					failoverPolicy.Data,
				)
			}
			if destConfig != nil {
				details.DestinationPolicy = destConfig.Data
			}
		}

		// TODO: Create derived composite routing instruction.
		meshConfig.PortedConfigs[port] = mc
	}

	if !inMesh {
		meshConfig = nil
	}

	return &ComputedRoutesResult{
		ID:      computedRoutesID,
		OwnerID: parentServiceID,
		Data:    meshConfig,
	}
}

func compileHTTPRouteNode(
	parentServiceRef *pbresource.Reference,
	port string,
	res *pbresource.Resource,
	route *pbmesh.HTTPRoute,
	serviceGetter serviceGetter,
) *inputRouteNode {
	route = protoClone(route)
	node := newInputRouteNode(port)

	dec := &types.DecodedHTTPRoute{
		Resource: res,
		Data:     route,
	}

	node.RouteType = types.HTTPRouteType
	node.HTTP = dec
	node.HTTPRules = make([]*pbmesh.InterpretedHTTPRouteRule, 0, len(route.Rules))
	for _, rule := range route.Rules {
		irule := &pbmesh.InterpretedHTTPRouteRule{
			Matches:     protoSliceClone(rule.Matches),
			Filters:     protoSliceClone(rule.Filters),
			BackendRefs: make([]*pbmesh.InterpretedHTTPBackendRef, 0, len(rule.BackendRefs)), // TODO: populate
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

			if backendSvc == nil {
				backendTarget = types.NullRouteBackend
			} else {
				found := false
				inMesh := false
				for _, port := range backendSvc.Data.Ports {
					if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
						inMesh = true
						continue // skip
					}
					if port.TargetPort == backendRef.BackendRef.Port {
						found = true
					}
				}

				if inMesh && found {
					details := &pbmesh.BackendTargetDetails{
						BackendRef: backendRef.BackendRef,
						// TODO
					}
					backendTarget = node.AddTarget(backendRef.BackendRef, details)
				} else {
					backendTarget = types.NullRouteBackend
				}
			}
			ibr := &pbmesh.InterpretedHTTPBackendRef{
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
	parentServiceRef *pbresource.Reference,
	port string,
	res *pbresource.Resource,
	route *pbmesh.GRPCRoute,
	serviceGetter serviceGetter,
) *inputRouteNode {
	route = protoClone(route)

	node := newInputRouteNode(port)
	dec := &types.DecodedGRPCRoute{
		Resource: res,
		Data:     route,
	}

	node.RouteType = types.GRPCRouteType
	node.GRPC = dec
	node.GRPCRules = make([]*pbmesh.InterpretedGRPCRouteRule, 0, len(route.Rules))
	for _, rule := range route.Rules {
		irule := &pbmesh.InterpretedGRPCRouteRule{
			Matches:     protoSliceClone(rule.Matches),
			Filters:     protoSliceClone(rule.Filters),
			BackendRefs: make([]*pbmesh.InterpretedGRPCBackendRef, 0, len(rule.BackendRefs)), // TODO: populate
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
			if backendSvc == nil {
				backendTarget = types.NullRouteBackend
			} else {
				found := false
				inMesh := false
				for _, port := range backendSvc.Data.Ports {
					if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
						inMesh = true
						continue // skip
					}
					if port.TargetPort == backendRef.BackendRef.Port {
						found = true
					}
				}

				if inMesh && found {
					details := &pbmesh.BackendTargetDetails{
						BackendRef: backendRef.BackendRef,
						// TODO
					}
					backendTarget = node.AddTarget(backendRef.BackendRef, details)
				} else {
					backendTarget = types.NullRouteBackend
				}
			}

			ibr := &pbmesh.InterpretedGRPCBackendRef{
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
	parentServiceRef *pbresource.Reference,
	port string,
	res *pbresource.Resource,
	route *pbmesh.TCPRoute,
	serviceGetter serviceGetter,
) *inputRouteNode {
	route = protoClone(route)

	node := newInputRouteNode(port)
	dec := &types.DecodedTCPRoute{
		Resource: res,
		Data:     route,
	}

	node.RouteType = types.TCPRouteType
	node.TCP = dec
	node.TCPRules = make([]*pbmesh.InterpretedTCPRouteRule, 0, len(route.Rules))
	for _, rule := range route.Rules {
		irule := &pbmesh.InterpretedTCPRouteRule{
			BackendRefs: make([]*pbmesh.InterpretedTCPBackendRef, 0, len(rule.BackendRefs)), // TODO: populate
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
			if backendSvc == nil {
				backendTarget = types.NullRouteBackend
			} else {
				found := false
				inMesh := false
				for _, port := range backendSvc.Data.Ports {
					if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
						inMesh = true
						continue // skip
					}
					if port.TargetPort == backendRef.BackendRef.Port {
						found = true
					}
				}

				if inMesh && found {
					details := &pbmesh.BackendTargetDetails{
						BackendRef: backendRef.BackendRef,
						// TODO
					}
					backendTarget = node.AddTarget(backendRef.BackendRef, details)
				} else {
					backendTarget = types.NullRouteBackend
				}
			}

			ibr := &pbmesh.InterpretedTCPBackendRef{
				BackendTarget: backendTarget,
				Weight:        backendRef.Weight,
			}
			irule.BackendRefs = append(irule.BackendRefs, ibr)
		}

		node.TCPRules = append(node.TCPRules, irule)
	}

	return node
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
		// TODO
	})
	switch {
	case resource.EqualType(types.HTTPRouteType, typ):
		setupDefaultHTTPRouteNode(routeNode, defaultBackendTarget)
	case resource.EqualType(types.GRPCRouteType, typ):
		setupDefaultGRPCRouteNode(routeNode, defaultBackendTarget)
	case resource.EqualType(types.TCPRouteType, typ):
		fallthrough
	default:
		setupDefaultTCPRouteNode(routeNode, defaultBackendTarget)
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

func setupDefaultHTTPRouteNode(
	routeNode *inputRouteNode,
	defaultBackendTarget string,
) {
	routeNode.RouteType = types.HTTPRouteType
	appendDefaultHTTPRouteRule(routeNode, defaultBackendTarget)
}

func appendDefaultHTTPRouteRule(
	routeNode *inputRouteNode,
	backendTarget string,
) {
	routeNode.HTTPRules = append(routeNode.HTTPRules, &pbmesh.InterpretedHTTPRouteRule{
		Matches: defaultHTTPRouteMatches(),
		BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
			BackendTarget: backendTarget,
		}},
	})
}

func setupDefaultGRPCRouteNode(
	routeNode *inputRouteNode,
	defaultBackendTarget string,
) {
	routeNode.RouteType = types.GRPCRouteType
	appendDefaultGRPCRouteRule(routeNode, defaultBackendTarget)
}

func appendDefaultGRPCRouteRule(
	routeNode *inputRouteNode,
	backendTarget string,
) {
	routeNode.GRPCRules = append(routeNode.GRPCRules, &pbmesh.InterpretedGRPCRouteRule{
		Matches: defaultGRPCRouteMatches(),
		BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
			BackendTarget: backendTarget,
		}},
	})
}

func setupDefaultTCPRouteNode(
	routeNode *inputRouteNode,
	defaultBackendTarget string,
) {
	routeNode.RouteType = types.TCPRouteType
	appendDefaultTCPRouteRule(routeNode, defaultBackendTarget)
}

func appendDefaultTCPRouteRule(
	routeNode *inputRouteNode,
	backendTarget string,
) {
	routeNode.TCPRules = append(routeNode.TCPRules, &pbmesh.InterpretedTCPRouteRule{
		BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
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

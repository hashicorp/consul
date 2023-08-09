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
	logger hclog.Logger,
	related *loader.RelatedResources,
	pending PendingStatuses,
) []*ComputedRoutesResult {
	out := make([]*ComputedRoutesResult, 0, len(related.ComputedRoutesList))
	for _, computedRoutesID := range related.ComputedRoutesList {
		out = append(out, compile(ctx, logger, related, computedRoutesID, pending))
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
	logger hclog.Logger,
	related *loader.RelatedResources,
	computedRoutesID *pbresource.ID,
	pending PendingStatuses,
) *ComputedRoutesResult {
	logger = logger.With("mesh-config-id", resource.IDToString(computedRoutesID))

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
		port := ""
		found := false
		for _, ref := range xroute.GetParentRefs() {
			if resource.ReferenceOrIDMatch(ref.Ref, parentServiceRef) {
				found = true
				port = ref.Port
				break
			}
		}

		if !found {
			return // not relevant to this mesh config

		}

		var node *inputRouteNode
		switch route := xroute.(type) {
		case *pbmesh.HTTPRoute:
			node = compileHTTPRouteNode(port, res, route)
		case *pbmesh.GRPCRoute:
			node = compileGRPCRouteNode(port, res, route)
		case *pbmesh.TCPRoute:
			node = compileTCPRouteNode(port, res, route)
		default:
			return // unknown xroute type (impossible)
		}

		// Check if the user provided port is actually valid.
		if port != "" {
			if _, ok := allowedPortProtocols[port]; !ok {
				for _, details := range node.NewTargets {
					details.NullRouteTraffic = true
				}
			}
		}

		if node.ParentPort == "" {
			// Do a port explosion.
			for port := range allowedPortProtocols {
				nodeCopy := node.Clone()
				nodeCopy.ParentPort = port
				routeNodesByPort[nodeCopy.ParentPort] = append(routeNodesByPort[nodeCopy.ParentPort], nodeCopy)
			}
		} else {
			routeNodesByPort[node.ParentPort] = append(routeNodesByPort[node.ParentPort], node)
		}
	})

	// Fill in defaults where there was no xroute defined
	for port, protocol := range allowedPortProtocols {
		// always add the parent as a possible target due to catch-all
		defaultBackendRef := &pbmesh.BackendReference{
			Ref:  parentServiceRef,
			Port: port,
		}

		routeNode, ok := routeNodesByPort[port]
		if ok {
			// TODO: ignore this whole section?

			// TODO: figure out how to add the catchall route/match section like in v1.
			//
			// Just add it to one of them.
			defaultBackendTarget := routeNode[0].AddTarget(defaultBackendRef, &pbmesh.BackendTargetDetails{
				BackendRef: defaultBackendRef,
				// TODO
			})
			_ = defaultBackendTarget
		} else {
			var typ *pbresource.Type
			switch protocol {
			case pbcatalog.Protocol_PROTOCOL_HTTP2:
				fallthrough // to HTTP
			case pbcatalog.Protocol_PROTOCOL_HTTP:
				typ = types.HTTPRouteType
			case pbcatalog.Protocol_PROTOCOL_GRPC:
				typ = types.GRPCRouteType
			case pbcatalog.Protocol_PROTOCOL_TCP:
				fallthrough // to default
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

		// Inject catch-all rules to ensure stray requests will ultimately end
		// up routing to the parent ref.
		if !top.Default {
			defaultRouteNode := createDefaultRouteNode(parentServiceRef, port, top.RouteType)
			top.AppendRulesFrom(defaultRouteNode)
			top.AddTargetsFrom(defaultRouteNode)
		}

		mc := &pbmesh.ComputedPortRoutes{
			Targets: top.NewTargets,
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

		for _, details := range mc.Targets {
			svcRef := details.BackendRef.Ref

			svc := related.GetService(svcRef)
			failoverPolicy := related.GetFailoverPolicyForService(svcRef)
			destConfig := related.GetDestinationPolicy(svcRef)

			if svc == nil {
				details.NullRouteTraffic = true
			} else {
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
	port string,
	res *pbresource.Resource,
	route *pbmesh.HTTPRoute,
) *inputRouteNode {
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
			irule.Matches = []*pbmesh.HTTPRouteMatch{{
				Path: &pbmesh.HTTPPathMatch{
					Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
					Value: "/",
				},
			}}
		}
		for _, match := range irule.Matches {
			if match.Path == nil {
				// Path specifies a HTTP request path matcher. If this
				// field is not specified, a default prefix match on
				// the “/” path is provided.
				match.Path = &pbmesh.HTTPPathMatch{
					Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
					Value: "/",
				}
			}
		}

		for _, backendRef := range rule.BackendRefs {
			details := &pbmesh.BackendTargetDetails{
				BackendRef: backendRef.BackendRef,
				// TODO
			}
			key := node.AddTarget(backendRef.BackendRef, details)
			ibr := &pbmesh.InterpretedHTTPBackendRef{
				BackendTarget: key,
				Weight:        backendRef.Weight,
				Filters:       backendRef.Filters,
			}
			irule.BackendRefs = append(irule.BackendRefs, ibr)
		}

		node.HTTPRules = append(node.HTTPRules, irule)
	}

	// TODO: finish populating backendrefs using parentrefs if target unspecified
	return node
}

func compileGRPCRouteNode(
	port string,
	res *pbresource.Resource,
	route *pbmesh.GRPCRoute,
) *inputRouteNode {
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
			irule.Matches = []*pbmesh.GRPCRouteMatch{{}}
		}

		for _, backendRef := range rule.BackendRefs {
			details := &pbmesh.BackendTargetDetails{
				BackendRef: backendRef.BackendRef,
				// TODO
			}
			key := node.AddTarget(backendRef.BackendRef, details)
			ibr := &pbmesh.InterpretedGRPCBackendRef{
				BackendTarget: key,
				Weight:        backendRef.Weight,
				Filters:       backendRef.Filters,
			}
			irule.BackendRefs = append(irule.BackendRefs, ibr)
		}

		node.GRPCRules = append(node.GRPCRules, irule)
	}

	// TODO: finish populating backendrefs using parentrefs if target unspecified
	return node
}

func compileTCPRouteNode(
	port string,
	res *pbresource.Resource,
	route *pbmesh.TCPRoute,
) *inputRouteNode {
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
			details := &pbmesh.BackendTargetDetails{
				BackendRef: backendRef.BackendRef,
				// TODO
			}
			key := node.AddTarget(backendRef.BackendRef, details)
			ibr := &pbmesh.InterpretedTCPBackendRef{
				BackendTarget: key,
				Weight:        backendRef.Weight,
			}
			irule.BackendRefs = append(irule.BackendRefs, ibr)
		}

		node.TCPRules = append(node.TCPRules, irule)
	}

	// TODO: finish populating backendrefs using parentrefs if target unspecified
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

func setupDefaultHTTPRouteNode(
	routeNode *inputRouteNode,
	defaultBackendTarget string,
) {
	routeNode.RouteType = types.HTTPRouteType
	routeNode.HTTPRules = []*pbmesh.InterpretedHTTPRouteRule{{
		Matches: []*pbmesh.HTTPRouteMatch{{
			Path: &pbmesh.HTTPPathMatch{
				Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
				Value: "/",
			},
		}},
		BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
			BackendTarget: defaultBackendTarget,
		}},
	}}
}

func setupDefaultGRPCRouteNode(
	routeNode *inputRouteNode,
	defaultBackendTarget string,
) {
	routeNode.RouteType = types.GRPCRouteType
	routeNode.GRPCRules = []*pbmesh.InterpretedGRPCRouteRule{{
		Matches: []*pbmesh.GRPCRouteMatch{{}},
		BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
			BackendTarget: defaultBackendTarget,
		}},
	}}
}

func setupDefaultTCPRouteNode(
	routeNode *inputRouteNode,
	defaultBackendTarget string,
) {
	routeNode.RouteType = types.TCPRouteType
	routeNode.TCPRules = []*pbmesh.InterpretedTCPRouteRule{{
		BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
			BackendTarget: defaultBackendTarget,
		}},
	}}
}

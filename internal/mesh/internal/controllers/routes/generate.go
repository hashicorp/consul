// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/loader"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
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
		Type:    pbcatalog.ServiceType,
		Tenancy: computedRoutesID.Tenancy,
		Name:    computedRoutesID.Name,
	}

	parentServiceRef := resource.Reference(parentServiceID, "")

	// The bound reference collector is supposed to aggregate all
	// references to resources that influence the production of
	// a ComputedRoutes resource.
	//
	// We only add a reference to the collector if the following are ALL true:
	//
	// - We load the resource for some reason.
	// - The resource is found.
	// - We decided to use the information in that resource to produce
	//   ComputedRoutes.
	//
	// We currently add all of the data here, but only use the xRoute
	// references to enhance the DependencyMappers for this Controller. In the
	// future we could use the others, but for now they are harmless to include
	// in the produced resource and is beneficial from an audit/debugging
	// perspective to know all of the inputs that produced this output.
	boundRefCollector := NewBoundReferenceCollector()

	parentServiceDec := related.GetService(parentServiceID)
	if parentServiceDec == nil {
		return &ComputedRoutesResult{
			ID:   computedRoutesID,
			Data: nil, // returning nil signals a delete is requested
		}
	}
	parentServiceID = parentServiceDec.Resource.Id // get ULID out of it

	boundRefCollector.AddRefOrID(parentServiceRef)

	var (
		inMesh               = false
		parentMeshPort       string
		allowedPortProtocols = make(map[string]pbcatalog.Protocol)
	)
	for _, port := range parentServiceDec.Data.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			inMesh = true
			parentMeshPort = port.TargetPort
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
		boundRefCollector.AddRefOrID(res.Id)

		for _, port := range ports {
			if port == "" {
				panic("impossible to have an empty port here")
			}

			var node *inputRouteNode
			switch route := xroute.(type) {
			case *pbmesh.HTTPRoute:
				node = compileHTTPRouteNode(port, res, route, related, boundRefCollector)
			case *pbmesh.GRPCRoute:
				node = compileGRPCRouteNode(port, res, route, related, boundRefCollector)
			case *pbmesh.TCPRoute:
				node = compileTCPRouteNode(port, res, route, related, boundRefCollector)
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
				typ = pbmesh.HTTPRouteType
			case pbcatalog.Protocol_PROTOCOL_HTTP:
				typ = pbmesh.HTTPRouteType
			case pbcatalog.Protocol_PROTOCOL_GRPC:
				typ = pbmesh.GRPCRouteType
			case pbcatalog.Protocol_PROTOCOL_TCP:
				typ = pbmesh.TCPRouteType
			case pbcatalog.Protocol_PROTOCOL_MESH:
				fallthrough // to default
			case pbcatalog.Protocol_PROTOCOL_UNSPECIFIED:
				fallthrough // to default
			default:
				continue // not possible
			}

			routeNode := createDefaultRouteNode(parentServiceRef, parentMeshPort, port, typ)

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
		case resource.EqualType(top.RouteType, pbmesh.HTTPRouteType):
			if mc.Protocol == pbcatalog.Protocol_PROTOCOL_TCP {
				// The rest are HTTP-like
				mc.Protocol = pbcatalog.Protocol_PROTOCOL_HTTP
			}
			mc.Config = &pbmesh.ComputedPortRoutes_Http{
				Http: &pbmesh.ComputedHTTPRoute{
					Rules: top.HTTPRules,
				},
			}
		case resource.EqualType(top.RouteType, pbmesh.GRPCRouteType):
			mc.Protocol = pbcatalog.Protocol_PROTOCOL_GRPC
			mc.Config = &pbmesh.ComputedPortRoutes_Grpc{
				Grpc: &pbmesh.ComputedGRPCRoute{
					Rules: top.GRPCRules,
				},
			}
		case resource.EqualType(top.RouteType, pbmesh.TCPRouteType):
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

		// The first pass collects the failover policies and generates additional targets.
		for _, details := range mc.Targets {
			svcRef := details.BackendRef.Ref

			svc := related.GetService(svcRef)
			if svc == nil {
				panic("impossible at this point; should already have been handled before getting here")
			}
			boundRefCollector.AddRefOrID(svcRef)

			failoverPolicy := related.GetFailoverPolicyForService(svcRef)
			if failoverPolicy != nil {
				simpleFailoverPolicy := catalog.SimplifyFailoverPolicy(svc.Data, failoverPolicy.Data)
				portFailoverConfig, ok := simpleFailoverPolicy.PortConfigs[details.BackendRef.Port]
				if ok {
					boundRefCollector.AddRefOrID(failoverPolicy.Resource.Id)

					details.FailoverConfig = compileFailoverConfig(
						related,
						portFailoverConfig,
						mc.Targets,
						boundRefCollector,
					)
				}
			}
		}

		// Do a second pass to handle shared things after we've dumped the
		// failover legs into here.
		for _, details := range mc.Targets {
			svcRef := details.BackendRef.Ref
			destPolicy := related.GetDestinationPolicyForService(svcRef)
			if destPolicy != nil {
				portDestConfig, ok := destPolicy.Data.PortConfigs[details.BackendRef.Port]
				if ok {
					boundRefCollector.AddRefOrID(destPolicy.Resource.Id)
					details.DestinationConfig = portDestConfig
				}
			}
			details.DestinationConfig = fillInDefaultDestConfig(details.DestinationConfig)
		}

		// Pull target information up to the level of the rules.
		switch x := mc.Config.(type) {
		case *pbmesh.ComputedPortRoutes_Http:
			route := x.Http
			for _, rule := range route.Rules {
				// If there are multiple legs (split) then choose the first actually set value.
				var requestTimeoutFallback *durationpb.Duration
				for _, backendRef := range rule.BackendRefs {
					if backendRef.BackendTarget == types.NullRouteBackend {
						continue
					}
					details, ok := mc.Targets[backendRef.BackendTarget]
					if !ok {
						continue
					}
					if details.DestinationConfig.RequestTimeout != nil {
						requestTimeoutFallback = details.DestinationConfig.RequestTimeout
						break
					}
				}

				if requestTimeoutFallback == nil {
					continue // nothing to do
				}

				if rule.Timeouts == nil {
					rule.Timeouts = &pbmesh.HTTPRouteTimeouts{}
				}
				if rule.Timeouts.Request == nil {
					rule.Timeouts.Request = requestTimeoutFallback
				}
			}
		case *pbmesh.ComputedPortRoutes_Grpc:
			route := x.Grpc
			for _, rule := range route.Rules {
				// If there are multiple legs (split) then choose the first actually set value.
				var requestTimeoutFallback *durationpb.Duration
				for _, backendRef := range rule.BackendRefs {
					if backendRef.BackendTarget == types.NullRouteBackend {
						continue
					}
					details, ok := mc.Targets[backendRef.BackendTarget]
					if !ok {
						continue
					}
					if details.DestinationConfig.RequestTimeout != nil {
						requestTimeoutFallback = details.DestinationConfig.RequestTimeout
						break
					}
				}

				if requestTimeoutFallback == nil {
					continue // nothing to do
				}

				if rule.Timeouts == nil {
					rule.Timeouts = &pbmesh.HTTPRouteTimeouts{}
				}
				if rule.Timeouts.Request == nil {
					rule.Timeouts.Request = requestTimeoutFallback
				}
			}
		case *pbmesh.ComputedPortRoutes_Tcp:
		}

		computedRoutes.PortedConfigs[port] = mc
	}

	if len(computedRoutes.PortedConfigs) == 0 {
		// This service only exposes a "mesh" port, so it cannot be another service's upstream.
		return &ComputedRoutesResult{
			ID:   computedRoutesID,
			Data: nil, // returning nil signals a delete is requested
		}
	}

	computedRoutes.BoundReferences = boundRefCollector.List()

	return &ComputedRoutesResult{
		ID:      computedRoutesID,
		OwnerID: parentServiceID,
		Data:    computedRoutes,
	}
}

func compileFailoverConfig(
	related *loader.RelatedResources,
	failoverConfig *pbcatalog.FailoverConfig,
	targets map[string]*pbmesh.BackendTargetDetails,
	brc *BoundReferenceCollector,
) *pbmesh.ComputedFailoverConfig {
	if failoverConfig == nil {
		return nil
	}

	cfc := &pbmesh.ComputedFailoverConfig{
		Destinations:  make([]*pbmesh.ComputedFailoverDestination, 0, len(failoverConfig.Destinations)),
		Mode:          failoverConfig.Mode,
		Regions:       failoverConfig.Regions,
		SamenessGroup: failoverConfig.SamenessGroup,
	}

	for _, dest := range failoverConfig.Destinations {
		backendRef := &pbmesh.BackendReference{
			Ref:        dest.Ref,
			Port:       dest.Port,
			Datacenter: dest.Datacenter,
		}

		destSvcRef := dest.Ref

		svc := related.GetService(destSvcRef)
		if svc != nil {
			brc.AddRefOrID(svc.Resource.Id)
		}

		var backendTargetName string
		ok, meshPort := shouldRouteTrafficToBackend(svc, backendRef)
		if !ok {
			continue // skip this leg of failover for now
		}

		destTargetName := types.BackendRefToComputedRoutesTarget(backendRef)

		if _, exists := targets[destTargetName]; !exists {
			// Add to output as an indirect target.
			targets[destTargetName] = &pbmesh.BackendTargetDetails{
				Type:       pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_INDIRECT,
				BackendRef: backendRef,
				MeshPort:   meshPort,
			}
		}
		backendTargetName = destTargetName

		cfc.Destinations = append(cfc.Destinations, &pbmesh.ComputedFailoverDestination{
			BackendTarget: backendTargetName,
		})
	}
	return cfc
}

func fillInDefaultDestConfig(target *pbmesh.DestinationConfig) *pbmesh.DestinationConfig {
	base := defaultDestConfig()

	if target == nil {
		return proto.Clone(base).(*pbmesh.DestinationConfig)
	}

	out := proto.Clone(target).(*pbmesh.DestinationConfig)

	if out.ConnectTimeout == nil {
		out.ConnectTimeout = base.GetConnectTimeout()
	}

	return out
}

func defaultDestConfig() *pbmesh.DestinationConfig {
	return &pbmesh.DestinationConfig{
		ConnectTimeout: durationpb.New(5 * time.Second),
	}
}

func compileHTTPRouteNode(
	port string,
	res *pbresource.Resource,
	route *pbmesh.HTTPRoute,
	serviceGetter serviceGetter,
	brc *BoundReferenceCollector,
) *inputRouteNode {
	route = protoClone(route)
	node := newInputRouteNode(port)

	node.RouteType = pbmesh.HTTPRouteType
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
			if backendSvc != nil {
				brc.AddRefOrID(backendSvc.Resource.Id)
			}
			if ok, meshPort := shouldRouteTrafficToBackend(backendSvc, backendRef.BackendRef); ok {
				details := &pbmesh.BackendTargetDetails{
					Type:       pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
					BackendRef: backendRef.BackendRef,
					MeshPort:   meshPort,
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
	brc *BoundReferenceCollector,
) *inputRouteNode {
	route = protoClone(route)

	node := newInputRouteNode(port)

	node.RouteType = pbmesh.GRPCRouteType
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
			if backendSvc != nil {
				brc.AddRefOrID(backendSvc.Resource.Id)
			}
			if ok, meshPort := shouldRouteTrafficToBackend(backendSvc, backendRef.BackendRef); ok {
				details := &pbmesh.BackendTargetDetails{
					Type:       pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
					BackendRef: backendRef.BackendRef,
					MeshPort:   meshPort,
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
	brc *BoundReferenceCollector,
) *inputRouteNode {
	route = protoClone(route)

	node := newInputRouteNode(port)

	node.RouteType = pbmesh.TCPRouteType
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
			if backendSvc != nil {
				brc.AddRefOrID(backendSvc.Resource.Id)
			}
			if ok, meshPort := shouldRouteTrafficToBackend(backendSvc, backendRef.BackendRef); ok {
				details := &pbmesh.BackendTargetDetails{
					Type:       pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
					BackendRef: backendRef.BackendRef,
					MeshPort:   meshPort,
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

func shouldRouteTrafficToBackend(backendSvc *types.DecodedService, backendRef *pbmesh.BackendReference) (bool, string) {
	if backendSvc == nil {
		return false, ""
	}

	var (
		found    = false
		inMesh   = false
		meshPort string
	)
	for _, port := range backendSvc.Data.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			inMesh = true
			meshPort = port.TargetPort
			continue
		}
		if port.TargetPort == backendRef.Port {
			found = true
		}
	}

	return inMesh && found, meshPort
}

func createDefaultRouteNode(
	parentServiceRef *pbresource.Reference,
	parentMeshPort string,
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
		Type:       pbmesh.BackendTargetDetailsType_BACKEND_TARGET_DETAILS_TYPE_DIRECT,
		BackendRef: defaultBackendRef,
		MeshPort:   parentMeshPort,
	})
	switch {
	case resource.EqualType(pbmesh.HTTPRouteType, typ):
		routeNode.RouteType = pbmesh.HTTPRouteType
		appendDefaultHTTPRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(pbmesh.GRPCRouteType, typ):
		routeNode.RouteType = pbmesh.GRPCRouteType
		appendDefaultGRPCRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(pbmesh.TCPRouteType, typ):
		fallthrough
	default:
		routeNode.RouteType = pbmesh.TCPRouteType
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
	case resource.EqualType(pbmesh.HTTPRouteType, routeNode.RouteType):
		appendDefaultHTTPRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(pbmesh.GRPCRouteType, routeNode.RouteType):
		appendDefaultGRPCRouteRule(routeNode, defaultBackendTarget)
	case resource.EqualType(pbmesh.TCPRouteType, routeNode.RouteType):
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

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// TODO: move to a new package? compiler?

// inputRouteNode is a dressed up version of an XRoute meant as working state
// for one pass of the compilation procedure.
type inputRouteNode struct {
	ParentPort string // always set

	// only one of these can be set to non-empty
	HTTPRules []*pbmesh.InterpretedHTTPRouteRule
	GRPCRules []*pbmesh.InterpretedGRPCRouteRule
	TCPRules  []*pbmesh.InterpretedTCPRouteRule

	RouteType *pbresource.Type
	Default   bool

	NewTargets map[string]*pbmesh.BackendTargetDetails

	// These three are the originals. If something needs customization for
	// compilation a shadow field will exist on this enclosing object instead.
	//
	// only one of these can be set to non-nil
	HTTP *types.DecodedHTTPRoute
	GRPC *types.DecodedGRPCRoute
	TCP  *types.DecodedTCPRoute
}

func newInputRouteNode(port string) *inputRouteNode {
	return &inputRouteNode{
		ParentPort: port,
		NewTargets: make(map[string]*pbmesh.BackendTargetDetails),
	}
}

func (n *inputRouteNode) AddTarget(backendRef *pbmesh.BackendReference, data *pbmesh.BackendTargetDetails) string {
	n.Default = false
	key := types.BackendRefToString(backendRef)

	if _, ok := n.NewTargets[key]; !ok {
		n.NewTargets[key] = data
	}

	return key
}

func (n *inputRouteNode) AddTargetsFrom(next *inputRouteNode) {
	n.Default = false
	for key, details := range next.NewTargets {
		if _, ok := n.NewTargets[key]; !ok {
			n.NewTargets[key] = details // add if not already there
		}
	}
}

func (n *inputRouteNode) AppendRulesFrom(next *inputRouteNode) {
	n.Default = false
	switch {
	case resource.EqualType(n.RouteType, types.HTTPRouteType):
		n.HTTPRules = append(n.HTTPRules, next.HTTPRules...)
	case resource.EqualType(n.RouteType, types.GRPCRouteType):
		n.GRPCRules = append(n.GRPCRules, next.GRPCRules...)
	case resource.EqualType(n.RouteType, types.TCPRouteType):
		n.TCPRules = append(n.TCPRules, next.TCPRules...)
	default:
		panic("impossible")
	}
}

func (n *inputRouteNode) OriginalResource() *pbresource.Resource {
	switch {
	case n.HTTP != nil:
		return n.HTTP.GetResource()
	case n.GRPC != nil:
		return n.GRPC.GetResource()
	case n.TCP != nil:
		return n.TCP.GetResource()
	default:
		panic("impossible")
	}
}

func (n *inputRouteNode) Clone() *inputRouteNode {
	n2 := *n
	n2.HTTPRules = protoSliceClone(n.HTTPRules)
	n2.GRPCRules = protoSliceClone(n.GRPCRules)
	n2.TCPRules = protoSliceClone(n.TCPRules)

	n2.NewTargets = make(map[string]*pbmesh.BackendTargetDetails)
	for key, details := range n.NewTargets {
		n2.NewTargets[key] = proto.Clone(details).(*pbmesh.BackendTargetDetails)
	}

	// only shallow copy the protobuf stuff since we don't touch it
	return &n2
}

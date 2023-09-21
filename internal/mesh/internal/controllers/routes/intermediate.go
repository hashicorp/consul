// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// inputRouteNode is a dressed up version of an XRoute meant as working state
// for one pass of the compilation procedure.
type inputRouteNode struct {
	ParentPort string // always set

	RouteType *pbresource.Type
	Default   bool

	// only one of these can be set to non-empty
	HTTPRules []*pbmesh.ComputedHTTPRouteRule
	GRPCRules []*pbmesh.ComputedGRPCRouteRule
	TCPRules  []*pbmesh.ComputedTCPRouteRule

	NewTargets map[string]*pbmesh.BackendTargetDetails

	// This field is non-nil for nodes based on a single xRoute, and nil for
	// composite nodes or default nodes.
	OriginalResource *pbresource.Resource
}

func newInputRouteNode(port string) *inputRouteNode {
	return &inputRouteNode{
		ParentPort: port,
		NewTargets: make(map[string]*pbmesh.BackendTargetDetails),
	}
}

func (n *inputRouteNode) AddTarget(backendRef *pbmesh.BackendReference, data *pbmesh.BackendTargetDetails) string {
	n.Default = false
	key := types.BackendRefToComputedRoutesTarget(backendRef)

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
	case resource.EqualType(n.RouteType, pbmesh.HTTPRouteType):
		n.HTTPRules = append(n.HTTPRules, next.HTTPRules...)
	case resource.EqualType(n.RouteType, pbmesh.GRPCRouteType):
		n.GRPCRules = append(n.GRPCRules, next.GRPCRules...)
	case resource.EqualType(n.RouteType, pbmesh.TCPRouteType):
		n.TCPRules = append(n.TCPRules, next.TCPRules...)
	default:
		panic("impossible")
	}
}

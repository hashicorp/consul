package v2beta1

import "github.com/hashicorp/consul/proto-public/pbresource"

const (
	GroupName      = "mesh"
	VersionV2beta1 = "v2beta1"
	CurrentVersion = VersionV2beta1

	ComputedRoutesKind = "ComputedRoutes"
	GRPCRouteKind      = "GRPCRoute"
	HTTPRouteKind      = "HTTPRoute"
	TCPRouteKind       = "TCPRoute"

	DestinationPolicyKind      = "DestinationPolicy"
	ProxyConfigurationKind     = "ProxyConfiguration"
	ProxyStateTemplateKind     = "ProxyStateTemplate"
	UpstreamsKind              = "Upstreams"
	UpstreamsConfigurationKind = "UpstreamsConfiguration"
)

var (
	// ComputedRoutes
	ComputedRoutesV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         ComputedRoutesKind,
	}

	ComputedRoutesType = ComputedRoutesV2Beta1Type

	// DestinationPolicy
	DestinationPolicyV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         DestinationPolicyKind,
	}

	DestinationPolicyType = DestinationPolicyV2Beta1Type

	// GRPCRoute
	GRPCRouteV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         GRPCRouteKind,
	}

	GRPCRouteType = GRPCRouteV2Beta1Type

	// HTTPRoute
	HTTPRouteV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         HTTPRouteKind,
	}

	HTTPRouteType = HTTPRouteV2Beta1Type

	// ProxyConfiguration
	ProxyConfigurationV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         ProxyConfigurationKind,
	}

	ProxyConfigurationType = ProxyConfigurationV2Beta1Type

	// ProxyStateTemplate
	ProxyStateTemplateV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         ProxyStateTemplateKind,
	}

	ProxyStateTemplateType = ProxyStateTemplateV2Beta1Type

	// TCPRoute
	TCPRouteV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         TCPRouteKind,
	}

	TCPRouteType = TCPRouteV2Beta1Type

	// Upstreams
	UpstreamsV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         UpstreamsKind,
	}

	UpstreamsType = UpstreamsV2Beta1Type

	// UpstreamsConfiguration
	UpstreamsConfigurationV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         UpstreamsConfigurationKind,
	}

	UpstreamsConfigurationType = UpstreamsConfigurationV2Beta1Type
)

// Code generated by protoc-gen-resource-types. DO NOT EDIT.

package meshv2beta1

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	GroupName = "mesh"
	Version   = "v2beta1"

	ComputedExplicitDestinationsKind = "ComputedExplicitDestinations"
	ComputedProxyConfigurationKind   = "ComputedProxyConfiguration"
	ComputedRoutesKind               = "ComputedRoutes"
	DestinationPolicyKind            = "DestinationPolicy"
	DestinationsKind                 = "Destinations"
	DestinationsConfigurationKind    = "DestinationsConfiguration"
	GRPCRouteKind                    = "GRPCRoute"
	HTTPRouteKind                    = "HTTPRoute"
	ProxyConfigurationKind           = "ProxyConfiguration"
	ProxyStateTemplateKind           = "ProxyStateTemplate"
	TCPRouteKind                     = "TCPRoute"
)

var (
	ComputedExplicitDestinationsType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ComputedExplicitDestinationsKind,
	}

	ComputedProxyConfigurationType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ComputedProxyConfigurationKind,
	}

	ComputedRoutesType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ComputedRoutesKind,
	}

	DestinationPolicyType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         DestinationPolicyKind,
	}

	DestinationsType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         DestinationsKind,
	}

	DestinationsConfigurationType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         DestinationsConfigurationKind,
	}

	GRPCRouteType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         GRPCRouteKind,
	}

	HTTPRouteType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         HTTPRouteKind,
	}

	ProxyConfigurationType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ProxyConfigurationKind,
	}

	ProxyStateTemplateType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ProxyStateTemplateKind,
	}

	TCPRouteType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         TCPRouteKind,
	}
)

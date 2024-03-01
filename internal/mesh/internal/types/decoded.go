// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2"
)

type (
	DecodedHTTPRoute                  = resource.DecodedResource[*pbmesh.HTTPRoute]
	DecodedGRPCRoute                  = resource.DecodedResource[*pbmesh.GRPCRoute]
	DecodedTCPRoute                   = resource.DecodedResource[*pbmesh.TCPRoute]
	DecodedDestinationPolicy          = resource.DecodedResource[*pbmesh.DestinationPolicy]
	DecodedDestinationsConfiguration  = resource.DecodedResource[*pbmesh.DestinationsConfiguration]
	DecodedComputedRoutes             = resource.DecodedResource[*pbmesh.ComputedRoutes]
	DecodedComputedTrafficPermissions = resource.DecodedResource[*pbauth.ComputedTrafficPermissions]
	DecodedTrafficPermissions         = resource.DecodedResource[*pbauth.TrafficPermissions]
	DecodedComputedFailoverPolicy     = resource.DecodedResource[*pbcatalog.ComputedFailoverPolicy]
	DecodedFailoverPolicy             = resource.DecodedResource[*pbcatalog.FailoverPolicy]
	DecodedService                    = resource.DecodedResource[*pbcatalog.Service]
	DecodedServiceEndpoints           = resource.DecodedResource[*pbcatalog.ServiceEndpoints]
	DecodedWorkload                   = resource.DecodedResource[*pbcatalog.Workload]
	DecodedProxyConfiguration         = resource.DecodedResource[*pbmesh.ProxyConfiguration]
	DecodedComputedProxyConfiguration = resource.DecodedResource[*pbmesh.ComputedProxyConfiguration]
	DecodedDestinations               = resource.DecodedResource[*pbmesh.Destinations]
	DecodedComputedDestinations       = resource.DecodedResource[*pbmesh.ComputedExplicitDestinations]
	DecodedProxyStateTemplate         = resource.DecodedResource[*pbmesh.ProxyStateTemplate]
	DecodedMeshGateway                = resource.DecodedResource[*pbmesh.MeshGateway]
	DecodedComputedExportedServices   = resource.DecodedResource[*pbmulticluster.ComputedExportedServices]
	DecodedAPIGateway                 = resource.DecodedResource[*pbmesh.APIGateway]
)

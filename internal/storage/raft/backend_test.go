// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestIsRetiredType(t *testing.T) {
	var retired []*pbresource.Type
	{
		const (
			GroupName = "hcp"
			Version   = "v2"

			LinkKind           = "Link"
			TelemetryStateKind = "TelemetryState"
		)

		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         LinkKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         TelemetryStateKind,
		})
	}
	{
		const (
			GroupName = "tenancy"
			Version   = "v2beta1"

			NamespaceKind = "Namespace"
			PartitionKind = "Partition"
		)

		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         NamespaceKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         PartitionKind,
		})
	}
	{
		const (
			GroupName = "multicluster"
			Version   = "v2beta1"

			SamenessGroupKind = "SamenessGroup"
		)

		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         SamenessGroupKind,
		})
	}
	{
		const (
			GroupName = "mesh"
			Version   = "v2beta1"

			APIGatewayKind                   = "APIGateway"
			ComputedExplicitDestinationsKind = "ComputedExplicitDestinations"
			ComputedGatewayRoutesKind        = "ComputedGatewayRoutes"
			ComputedImplicitDestinationsKind = "ComputedImplicitDestinations"
			ComputedProxyConfigurationKind   = "ComputedProxyConfiguration"
			ComputedRoutesKind               = "ComputedRoutes"
			DestinationPolicyKind            = "DestinationPolicy"
			DestinationsKind                 = "Destinations"
			DestinationsConfigurationKind    = "DestinationsConfiguration"
			GRPCRouteKind                    = "GRPCRoute"
			HTTPRouteKind                    = "HTTPRoute"
			MeshConfigurationKind            = "MeshConfiguration"
			MeshGatewayKind                  = "MeshGateway"
			ProxyConfigurationKind           = "ProxyConfiguration"
			ProxyStateTemplateKind           = "ProxyStateTemplate"
			TCPRouteKind                     = "TCPRoute"
		)

		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         APIGatewayKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedExplicitDestinationsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedGatewayRoutesKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedImplicitDestinationsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedProxyConfigurationKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedRoutesKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         DestinationPolicyKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         DestinationsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         DestinationsConfigurationKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         GRPCRouteKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         HTTPRouteKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         MeshConfigurationKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         MeshGatewayKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ProxyConfigurationKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ProxyStateTemplateKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         TCPRouteKind,
		})
	}
	{
		const (
			GroupName = "auth"
			Version   = "v2beta1"

			ComputedTrafficPermissionsKind  = "ComputedTrafficPermissions"
			NamespaceTrafficPermissionsKind = "NamespaceTrafficPermissions"
			PartitionTrafficPermissionsKind = "PartitionTrafficPermissions"
			TrafficPermissionsKind          = "TrafficPermissions"
			WorkloadIdentityKind            = "WorkloadIdentity"
		)

		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedTrafficPermissionsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         NamespaceTrafficPermissionsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         PartitionTrafficPermissionsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         TrafficPermissionsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         WorkloadIdentityKind,
		})
	}
	{
		const (
			GroupName = "catalog"
			Version   = "v2beta1"

			ComputedFailoverPolicyKind = "ComputedFailoverPolicy"
			FailoverPolicyKind         = "FailoverPolicy"
			HealthChecksKind           = "HealthChecks"
			HealthStatusKind           = "HealthStatus"
			NodeKind                   = "Node"
			NodeHealthStatusKind       = "NodeHealthStatus"
			ServiceKind                = "Service"
			ServiceEndpointsKind       = "ServiceEndpoints"
			VirtualIPsKind             = "VirtualIPs"
			WorkloadKind               = "Workload"
		)

		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedFailoverPolicyKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         FailoverPolicyKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         HealthChecksKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         HealthStatusKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         NodeKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         NodeHealthStatusKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ServiceKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ServiceEndpointsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         VirtualIPsKind,
		})
		retired = append(retired, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         WorkloadKind,
		})
	}
	/*
	 */

	var retained []*pbresource.Type
	{
		const (
			GroupName = "demo"
			Version   = "v2"

			AlbumKind    = "Album"
			ArtistKind   = "Artist"
			FestivalKind = "Festival"
		)

		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         AlbumKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ArtistKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         FestivalKind,
		})
	}
	{
		const (
			GroupName = "demo"
			Version   = "v1"

			AlbumKind       = "Album"
			ArtistKind      = "Artist"
			ConceptKind     = "Concept"
			ExecutiveKind   = "Executive"
			RecordLabelKind = "RecordLabel"
		)

		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         AlbumKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ArtistKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ConceptKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ExecutiveKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         RecordLabelKind,
		})
	}
	{
		const (
			GroupName = "multicluster"
			Version   = "v2"

			ComputedExportedServicesKind  = "ComputedExportedServices"
			ExportedServicesKind          = "ExportedServices"
			NamespaceExportedServicesKind = "NamespaceExportedServices"
			PartitionExportedServicesKind = "PartitionExportedServices"
		)

		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ComputedExportedServicesKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         ExportedServicesKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         NamespaceExportedServicesKind,
		})
		retained = append(retained, &pbresource.Type{
			Group:        GroupName,
			GroupVersion: Version,
			Kind:         PartitionExportedServicesKind,
		})
	}

	for _, typ := range retired {
		t.Run("gone - "+resource.ToGVK(typ), func(t *testing.T) {
			require.True(t, isRetiredType(typ))
		})
	}
	for _, typ := range retained {
		t.Run("allowed - "+resource.ToGVK(typ), func(t *testing.T) {
			require.False(t, isRetiredType(typ))
		})
	}
}

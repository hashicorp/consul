// Code generated by protoc-gen-resource-types. DO NOT EDIT.

package authv1alpha1

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	GroupName = "auth"
	Version   = "v1alpha1"

	ComputedTrafficPermissionsKind  = "ComputedTrafficPermissions"
	NamespaceTrafficPermissionsKind = "NamespaceTrafficPermissions"
	PartitionTrafficPermissionsKind = "PartitionTrafficPermissions"
	TrafficPermissionsKind          = "TrafficPermissions"
	WorkloadIdentityKind            = "WorkloadIdentity"
)

var (
	ComputedTrafficPermissionsType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ComputedTrafficPermissionsKind,
	}

	NamespaceTrafficPermissionsType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         NamespaceTrafficPermissionsKind,
	}

	PartitionTrafficPermissionsType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         PartitionTrafficPermissionsKind,
	}

	TrafficPermissionsType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         TrafficPermissionsKind,
	}

	WorkloadIdentityType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         WorkloadIdentityKind,
	}
)

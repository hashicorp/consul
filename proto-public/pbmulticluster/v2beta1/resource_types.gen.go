// Code generated by protoc-gen-resource-types. DO NOT EDIT.

package multiclusterv2beta1

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	GroupName = "multicluster"
	Version   = "v2beta1"

	ComputedExportedServicesKind  = "ComputedExportedServices"
	ExportedServicesKind          = "ExportedServices"
	NamespaceExportedServicesKind = "NamespaceExportedServices"
	PartitionExportedServicesKind = "PartitionExportedServices"
)

var (
	ComputedExportedServicesType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ComputedExportedServicesKind,
	}

	ExportedServicesType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         ExportedServicesKind,
	}

	NamespaceExportedServicesType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         NamespaceExportedServicesKind,
	}

	PartitionExportedServicesType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         PartitionExportedServicesKind,
	}
)

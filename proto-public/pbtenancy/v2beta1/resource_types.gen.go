// Code generated by protoc-gen-resource-types. DO NOT EDIT.

package tenancyv2beta1

import (
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	GroupName = "tenancy"
	Version   = "v2beta1"

	NamespaceKind = "Namespace"
	PartitionKind = "Partition"
)

var (
	NamespaceType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         NamespaceKind,
	}

	PartitionType = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: Version,
		Kind:         PartitionKind,
	}
)

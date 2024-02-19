// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

type NodeID struct {
	Name      string `json:",omitempty"`
	Partition string `json:",omitempty"`
}

func NewNodeID(name, partition string) NodeID {
	id := NodeID{
		Name:      name,
		Partition: partition,
	}
	id.Normalize()
	return id
}

func (id *NodeID) Normalize() {
	id.Partition = PartitionOrDefault(id.Partition)
}

func (id NodeID) String() string {
	return fmt.Sprintf("%s/%s", id.Partition, id.Name)
}

func (id NodeID) ACLString() string {
	return fmt.Sprintf("%s--%s", id.Partition, id.Name)
}

func (id NodeID) TFString() string {
	return id.ACLString()
}

type ID struct {
	Name      string `json:",omitempty"`
	Namespace string `json:",omitempty"`
	Partition string `json:",omitempty"`
}

func NewID(name, namespace, partition string) ID {
	id := ID{
		Name:      name,
		Namespace: namespace,
		Partition: partition,
	}
	id.Normalize()
	return id
}

func (id ID) Less(other ID) bool {
	if id.Partition != other.Partition {
		return id.Partition < other.Partition
	}
	if id.Namespace != other.Namespace {
		return id.Namespace < other.Namespace
	}
	return id.Name < other.Name
}

func (id *ID) Normalize() {
	id.Namespace = NamespaceOrDefault(id.Namespace)
	id.Partition = PartitionOrDefault(id.Partition)
}

func (id ID) String() string {
	return fmt.Sprintf("%s/%s/%s", id.Partition, id.Namespace, id.Name)
}

func (id ID) ACLString() string {
	return fmt.Sprintf("%s--%s--%s", id.Partition, id.Namespace, id.Name)
}

func (id ID) TFString() string {
	return id.ACLString()
}

func (id ID) PartitionOrDefault() string {
	return PartitionOrDefault(id.Partition)
}

func (id ID) NamespaceOrDefault() string {
	return NamespaceOrDefault(id.Namespace)
}

func (id ID) QueryOptions() *api.QueryOptions {
	return &api.QueryOptions{
		Partition: DefaultToEmpty(id.Partition),
		Namespace: DefaultToEmpty(id.Namespace),
	}
}

func PartitionOrDefault(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func NamespaceOrDefault(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func DefaultToEmpty(name string) string {
	if name == "default" {
		return ""
	}
	return name
}

// PartitionQueryOptions returns an *api.QueryOptions with the given partition
// field set only if the partition is non-default. This helps when writing
// tests for joint use in CE and ENT.
func PartitionQueryOptions(partition string) *api.QueryOptions {
	return &api.QueryOptions{
		Partition: DefaultToEmpty(partition),
	}
}

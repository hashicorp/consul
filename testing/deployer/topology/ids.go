// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

type NodeServiceID struct {
	Node      string
	Service   string `json:",omitempty"`
	Namespace string `json:",omitempty"`
	Partition string `json:",omitempty"`
}

func NewNodeServiceID(node, service, namespace, partition string) NodeServiceID {
	id := NodeServiceID{
		Node:      node,
		Service:   service,
		Namespace: namespace,
		Partition: partition,
	}
	id.Normalize()
	return id
}

func (id NodeServiceID) NodeID() NodeID {
	return NewNodeID(id.Node, id.Partition)
}

func (id NodeServiceID) ServiceID() ServiceID {
	return NewServiceID(id.Service, id.Namespace, id.Partition)
}

func (id *NodeServiceID) Normalize() {
	id.Namespace = NamespaceOrDefault(id.Namespace)
	id.Partition = PartitionOrDefault(id.Partition)
}

func (id NodeServiceID) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", id.Partition, id.Node, id.Namespace, id.Service)
}

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

type ServiceID struct {
	Name      string `json:",omitempty"`
	Namespace string `json:",omitempty"`
	Partition string `json:",omitempty"`
}

func NewServiceID(name, namespace, partition string) ServiceID {
	id := ServiceID{
		Name:      name,
		Namespace: namespace,
		Partition: partition,
	}
	id.Normalize()
	return id
}

func (id ServiceID) Less(other ServiceID) bool {
	if id.Partition != other.Partition {
		return id.Partition < other.Partition
	}
	if id.Namespace != other.Namespace {
		return id.Namespace < other.Namespace
	}
	return id.Name < other.Name
}

func (id *ServiceID) Normalize() {
	id.Namespace = NamespaceOrDefault(id.Namespace)
	id.Partition = PartitionOrDefault(id.Partition)
}

func (id ServiceID) String() string {
	return fmt.Sprintf("%s/%s/%s", id.Partition, id.Namespace, id.Name)
}

func (id ServiceID) ACLString() string {
	return fmt.Sprintf("%s--%s--%s", id.Partition, id.Namespace, id.Name)
}
func (id ServiceID) TFString() string {
	return id.ACLString()
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
// tests for joint use in OSS and ENT.
func PartitionQueryOptions(partition string) *api.QueryOptions {
	return &api.QueryOptions{
		Partition: DefaultToEmpty(partition),
	}
}

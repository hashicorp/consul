// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbcatalog/v1alpha1/workload.proto

package catalogv1alpha1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Workload is the representation of a unit of addressable work. This could
// represent a process on a VM, a Kubernetes pod or something else entirely.
type Workload struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// addresses has a list of all workload addresses. This should include
	// LAN and WAN addresses as well as any addresses a proxy would need
	// to bind to (if different from the default address).
	Addresses []*WorkloadAddress `protobuf:"bytes,1,rep,name=addresses,proto3" json:"addresses,omitempty"`
	// ports is a map from port name to workload port’s number and protocol.
	Ports map[string]*WorkloadPort `protobuf:"bytes,2,rep,name=ports,proto3" json:"ports,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// node_name is the name of the node this workload belongs to.
	NodeName string `protobuf:"bytes,3,opt,name=node_name,json=nodeName,proto3" json:"node_name,omitempty"`
	// identity is the name of the workload identity this workload is associated with.
	Identity string `protobuf:"bytes,4,opt,name=identity,proto3" json:"identity,omitempty"`
	// Locality specifies workload locality.
	Locality *Locality `protobuf:"bytes,5,opt,name=locality,proto3" json:"locality,omitempty"`
	// deprecated: tags correspond to service tags that you can add to a service for DNS resolution.
	//
	// Deprecated: Marked as deprecated in pbcatalog/v1alpha1/workload.proto.
	Tags []string `protobuf:"bytes,6,rep,name=tags,proto3" json:"tags,omitempty"`
	// deprecated: enable_tag_override indicates whether agents should be overriding tags during anti-entropy syncs.
	//
	// Deprecated: Marked as deprecated in pbcatalog/v1alpha1/workload.proto.
	EnableTagOverride bool `protobuf:"varint,7,opt,name=enable_tag_override,json=enableTagOverride,proto3" json:"enable_tag_override,omitempty"`
	// deprecated: connect_native indicates whether this workload is connect native which will allow it to be
	// part of MeshEndpoints without having the corresponding Proxy resource.
	//
	// Deprecated: Marked as deprecated in pbcatalog/v1alpha1/workload.proto.
	ConnectNative bool `protobuf:"varint,8,opt,name=connect_native,json=connectNative,proto3" json:"connect_native,omitempty"`
}

func (x *Workload) Reset() {
	*x = Workload{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Workload) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Workload) ProtoMessage() {}

func (x *Workload) ProtoReflect() protoreflect.Message {
	mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Workload.ProtoReflect.Descriptor instead.
func (*Workload) Descriptor() ([]byte, []int) {
	return file_pbcatalog_v1alpha1_workload_proto_rawDescGZIP(), []int{0}
}

func (x *Workload) GetAddresses() []*WorkloadAddress {
	if x != nil {
		return x.Addresses
	}
	return nil
}

func (x *Workload) GetPorts() map[string]*WorkloadPort {
	if x != nil {
		return x.Ports
	}
	return nil
}

func (x *Workload) GetNodeName() string {
	if x != nil {
		return x.NodeName
	}
	return ""
}

func (x *Workload) GetIdentity() string {
	if x != nil {
		return x.Identity
	}
	return ""
}

func (x *Workload) GetLocality() *Locality {
	if x != nil {
		return x.Locality
	}
	return nil
}

// Deprecated: Marked as deprecated in pbcatalog/v1alpha1/workload.proto.
func (x *Workload) GetTags() []string {
	if x != nil {
		return x.Tags
	}
	return nil
}

// Deprecated: Marked as deprecated in pbcatalog/v1alpha1/workload.proto.
func (x *Workload) GetEnableTagOverride() bool {
	if x != nil {
		return x.EnableTagOverride
	}
	return false
}

// Deprecated: Marked as deprecated in pbcatalog/v1alpha1/workload.proto.
func (x *Workload) GetConnectNative() bool {
	if x != nil {
		return x.ConnectNative
	}
	return false
}

type WorkloadAddress struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// host can be an IP, DNS name or a unix socket.
	// If it's a unix socket, only one port can be provided.
	Host string `protobuf:"bytes,1,opt,name=host,proto3" json:"host,omitempty"`
	// ports is a list of names of ports that this host binds to.
	// If no ports are provided, we will assume all ports from the ports map.
	Ports []string `protobuf:"bytes,2,rep,name=ports,proto3" json:"ports,omitempty"`
	// external indicates whether this address should be used for external communication
	// (aka a WAN address).
	External bool `protobuf:"varint,3,opt,name=external,proto3" json:"external,omitempty"`
}

func (x *WorkloadAddress) Reset() {
	*x = WorkloadAddress{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WorkloadAddress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WorkloadAddress) ProtoMessage() {}

func (x *WorkloadAddress) ProtoReflect() protoreflect.Message {
	mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WorkloadAddress.ProtoReflect.Descriptor instead.
func (*WorkloadAddress) Descriptor() ([]byte, []int) {
	return file_pbcatalog_v1alpha1_workload_proto_rawDescGZIP(), []int{1}
}

func (x *WorkloadAddress) GetHost() string {
	if x != nil {
		return x.Host
	}
	return ""
}

func (x *WorkloadAddress) GetPorts() []string {
	if x != nil {
		return x.Ports
	}
	return nil
}

func (x *WorkloadAddress) GetExternal() bool {
	if x != nil {
		return x.External
	}
	return false
}

type WorkloadPort struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Port     uint32   `protobuf:"varint,1,opt,name=port,proto3" json:"port,omitempty"`
	Protocol Protocol `protobuf:"varint,2,opt,name=protocol,proto3,enum=hashicorp.consul.catalog.v1alpha1.Protocol" json:"protocol,omitempty"`
}

func (x *WorkloadPort) Reset() {
	*x = WorkloadPort{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WorkloadPort) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WorkloadPort) ProtoMessage() {}

func (x *WorkloadPort) ProtoReflect() protoreflect.Message {
	mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WorkloadPort.ProtoReflect.Descriptor instead.
func (*WorkloadPort) Descriptor() ([]byte, []int) {
	return file_pbcatalog_v1alpha1_workload_proto_rawDescGZIP(), []int{2}
}

func (x *WorkloadPort) GetPort() uint32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *WorkloadPort) GetProtocol() Protocol {
	if x != nil {
		return x.Protocol
	}
	return Protocol_PROTOCOL_TCP
}

type Locality struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Region is region the zone belongs to.
	Region string `protobuf:"bytes,1,opt,name=region,proto3" json:"region,omitempty"`
	// Zone is the zone the entity is running in.
	Zone string `protobuf:"bytes,2,opt,name=zone,proto3" json:"zone,omitempty"`
}

func (x *Locality) Reset() {
	*x = Locality{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Locality) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Locality) ProtoMessage() {}

func (x *Locality) ProtoReflect() protoreflect.Message {
	mi := &file_pbcatalog_v1alpha1_workload_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Locality.ProtoReflect.Descriptor instead.
func (*Locality) Descriptor() ([]byte, []int) {
	return file_pbcatalog_v1alpha1_workload_proto_rawDescGZIP(), []int{3}
}

func (x *Locality) GetRegion() string {
	if x != nil {
		return x.Region
	}
	return ""
}

func (x *Locality) GetZone() string {
	if x != nil {
		return x.Zone
	}
	return ""
}

var File_pbcatalog_v1alpha1_workload_proto protoreflect.FileDescriptor

var file_pbcatalog_v1alpha1_workload_proto_rawDesc = []byte{
	0x0a, 0x21, 0x70, 0x62, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2f, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x2f, 0x77, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x21, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x1a, 0x21, 0x70, 0x62, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f,
	0x67, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x63, 0x6f, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x8e, 0x04, 0x0a, 0x08, 0x57, 0x6f,
	0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x12, 0x50, 0x0a, 0x09, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x32, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x63, 0x61, 0x74,
	0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x57, 0x6f,
	0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x09, 0x61,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x65, 0x73, 0x12, 0x4c, 0x0a, 0x05, 0x70, 0x6f, 0x72, 0x74,
	0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x36, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c,
	0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x57, 0x6f, 0x72, 0x6b,
	0x6c, 0x6f, 0x61, 0x64, 0x2e, 0x50, 0x6f, 0x72, 0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52,
	0x05, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x12, 0x1b, 0x0a, 0x09, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6e, 0x6f, 0x64, 0x65, 0x4e,
	0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x12,
	0x47, 0x0a, 0x08, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x69, 0x74, 0x79, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x2b, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x4c, 0x6f, 0x63, 0x61, 0x6c, 0x69, 0x74, 0x79, 0x52, 0x08,
	0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x69, 0x74, 0x79, 0x12, 0x16, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73,
	0x18, 0x06, 0x20, 0x03, 0x28, 0x09, 0x42, 0x02, 0x18, 0x01, 0x52, 0x04, 0x74, 0x61, 0x67, 0x73,
	0x12, 0x32, 0x0a, 0x13, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x74, 0x61, 0x67, 0x5f, 0x6f,
	0x76, 0x65, 0x72, 0x72, 0x69, 0x64, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x08, 0x42, 0x02, 0x18,
	0x01, 0x52, 0x11, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x54, 0x61, 0x67, 0x4f, 0x76, 0x65, 0x72,
	0x72, 0x69, 0x64, 0x65, 0x12, 0x29, 0x0a, 0x0e, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x5f,
	0x6e, 0x61, 0x74, 0x69, 0x76, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x08, 0x42, 0x02, 0x18, 0x01,
	0x52, 0x0d, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x4e, 0x61, 0x74, 0x69, 0x76, 0x65, 0x1a,
	0x69, 0x0a, 0x0a, 0x50, 0x6f, 0x72, 0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a,
	0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12,
	0x45, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2f,
	0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0x31, 0x2e, 0x57, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x50, 0x6f, 0x72, 0x74, 0x52,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x57, 0x0a, 0x0f, 0x57, 0x6f,
	0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x12, 0x0a,
	0x04, 0x68, 0x6f, 0x73, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x6f, 0x73,
	0x74, 0x12, 0x14, 0x0a, 0x05, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x05, 0x70, 0x6f, 0x72, 0x74, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x65, 0x78, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x65, 0x78, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x22, 0x6b, 0x0a, 0x0c, 0x57, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x50,
	0x6f, 0x72, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0d, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x47, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x63, 0x6f, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2b, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x63, 0x61, 0x74,
	0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x50, 0x72,
	0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c,
	0x22, 0x36, 0x0a, 0x08, 0x4c, 0x6f, 0x63, 0x61, 0x6c, 0x69, 0x74, 0x79, 0x12, 0x16, 0x0a, 0x06,
	0x72, 0x65, 0x67, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72, 0x65,
	0x67, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x7a, 0x6f, 0x6e, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x7a, 0x6f, 0x6e, 0x65, 0x42, 0xaa, 0x02, 0x0a, 0x25, 0x63, 0x6f, 0x6d,
	0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0x31, 0x42, 0x0d, 0x57, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x50, 0x01, 0x5a, 0x4b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62,
	0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x3b, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0xa2, 0x02, 0x03, 0x48, 0x43, 0x43, 0xaa, 0x02, 0x21, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x43, 0x61, 0x74, 0x61, 0x6c, 0x6f,
	0x67, 0x2e, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xca, 0x02, 0x21, 0x48, 0x61, 0x73,
	0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x43, 0x61,
	0x74, 0x61, 0x6c, 0x6f, 0x67, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xe2, 0x02,
	0x2d, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x5c, 0x43, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02,
	0x24, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x3a, 0x3a, 0x43, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbcatalog_v1alpha1_workload_proto_rawDescOnce sync.Once
	file_pbcatalog_v1alpha1_workload_proto_rawDescData = file_pbcatalog_v1alpha1_workload_proto_rawDesc
)

func file_pbcatalog_v1alpha1_workload_proto_rawDescGZIP() []byte {
	file_pbcatalog_v1alpha1_workload_proto_rawDescOnce.Do(func() {
		file_pbcatalog_v1alpha1_workload_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbcatalog_v1alpha1_workload_proto_rawDescData)
	})
	return file_pbcatalog_v1alpha1_workload_proto_rawDescData
}

var file_pbcatalog_v1alpha1_workload_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_pbcatalog_v1alpha1_workload_proto_goTypes = []interface{}{
	(*Workload)(nil),        // 0: hashicorp.consul.catalog.v1alpha1.Workload
	(*WorkloadAddress)(nil), // 1: hashicorp.consul.catalog.v1alpha1.WorkloadAddress
	(*WorkloadPort)(nil),    // 2: hashicorp.consul.catalog.v1alpha1.WorkloadPort
	(*Locality)(nil),        // 3: hashicorp.consul.catalog.v1alpha1.Locality
	nil,                     // 4: hashicorp.consul.catalog.v1alpha1.Workload.PortsEntry
	(Protocol)(0),           // 5: hashicorp.consul.catalog.v1alpha1.Protocol
}
var file_pbcatalog_v1alpha1_workload_proto_depIdxs = []int32{
	1, // 0: hashicorp.consul.catalog.v1alpha1.Workload.addresses:type_name -> hashicorp.consul.catalog.v1alpha1.WorkloadAddress
	4, // 1: hashicorp.consul.catalog.v1alpha1.Workload.ports:type_name -> hashicorp.consul.catalog.v1alpha1.Workload.PortsEntry
	3, // 2: hashicorp.consul.catalog.v1alpha1.Workload.locality:type_name -> hashicorp.consul.catalog.v1alpha1.Locality
	5, // 3: hashicorp.consul.catalog.v1alpha1.WorkloadPort.protocol:type_name -> hashicorp.consul.catalog.v1alpha1.Protocol
	2, // 4: hashicorp.consul.catalog.v1alpha1.Workload.PortsEntry.value:type_name -> hashicorp.consul.catalog.v1alpha1.WorkloadPort
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_pbcatalog_v1alpha1_workload_proto_init() }
func file_pbcatalog_v1alpha1_workload_proto_init() {
	if File_pbcatalog_v1alpha1_workload_proto != nil {
		return
	}
	file_pbcatalog_v1alpha1_protocol_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_pbcatalog_v1alpha1_workload_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Workload); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pbcatalog_v1alpha1_workload_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WorkloadAddress); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pbcatalog_v1alpha1_workload_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WorkloadPort); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pbcatalog_v1alpha1_workload_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Locality); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbcatalog_v1alpha1_workload_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbcatalog_v1alpha1_workload_proto_goTypes,
		DependencyIndexes: file_pbcatalog_v1alpha1_workload_proto_depIdxs,
		MessageInfos:      file_pbcatalog_v1alpha1_workload_proto_msgTypes,
	}.Build()
	File_pbcatalog_v1alpha1_workload_proto = out.File
	file_pbcatalog_v1alpha1_workload_proto_rawDesc = nil
	file_pbcatalog_v1alpha1_workload_proto_goTypes = nil
	file_pbcatalog_v1alpha1_workload_proto_depIdxs = nil
}

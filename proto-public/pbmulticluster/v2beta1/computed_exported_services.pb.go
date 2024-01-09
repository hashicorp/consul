// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: pbmulticluster/v2beta1/computed_exported_services.proto

package multiclusterv2beta1

import (
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
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

type ComputedExportedServices struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Services []*ComputedExportedService `protobuf:"bytes,1,rep,name=services,proto3" json:"services,omitempty"`
}

func (x *ComputedExportedServices) Reset() {
	*x = ComputedExportedServices{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ComputedExportedServices) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ComputedExportedServices) ProtoMessage() {}

func (x *ComputedExportedServices) ProtoReflect() protoreflect.Message {
	mi := &file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ComputedExportedServices.ProtoReflect.Descriptor instead.
func (*ComputedExportedServices) Descriptor() ([]byte, []int) {
	return file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescGZIP(), []int{0}
}

func (x *ComputedExportedServices) GetServices() []*ComputedExportedService {
	if x != nil {
		return x.Services
	}
	return nil
}

type ComputedExportedService struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TargetRef *pbresource.Reference              `protobuf:"bytes,1,opt,name=target_ref,json=targetRef,proto3" json:"target_ref,omitempty"`
	Consumers []*ComputedExportedServiceConsumer `protobuf:"bytes,2,rep,name=consumers,proto3" json:"consumers,omitempty"`
}

func (x *ComputedExportedService) Reset() {
	*x = ComputedExportedService{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ComputedExportedService) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ComputedExportedService) ProtoMessage() {}

func (x *ComputedExportedService) ProtoReflect() protoreflect.Message {
	mi := &file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ComputedExportedService.ProtoReflect.Descriptor instead.
func (*ComputedExportedService) Descriptor() ([]byte, []int) {
	return file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescGZIP(), []int{1}
}

func (x *ComputedExportedService) GetTargetRef() *pbresource.Reference {
	if x != nil {
		return x.TargetRef
	}
	return nil
}

func (x *ComputedExportedService) GetConsumers() []*ComputedExportedServiceConsumer {
	if x != nil {
		return x.Consumers
	}
	return nil
}

type ComputedExportedServiceConsumer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// no sameness group
	//
	// Types that are assignable to Tenancy:
	//
	//	*ComputedExportedServiceConsumer_Peer
	//	*ComputedExportedServiceConsumer_Partition
	Tenancy isComputedExportedServiceConsumer_Tenancy `protobuf_oneof:"tenancy"`
}

func (x *ComputedExportedServiceConsumer) Reset() {
	*x = ComputedExportedServiceConsumer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ComputedExportedServiceConsumer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ComputedExportedServiceConsumer) ProtoMessage() {}

func (x *ComputedExportedServiceConsumer) ProtoReflect() protoreflect.Message {
	mi := &file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ComputedExportedServiceConsumer.ProtoReflect.Descriptor instead.
func (*ComputedExportedServiceConsumer) Descriptor() ([]byte, []int) {
	return file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescGZIP(), []int{2}
}

func (m *ComputedExportedServiceConsumer) GetTenancy() isComputedExportedServiceConsumer_Tenancy {
	if m != nil {
		return m.Tenancy
	}
	return nil
}

func (x *ComputedExportedServiceConsumer) GetPeer() string {
	if x, ok := x.GetTenancy().(*ComputedExportedServiceConsumer_Peer); ok {
		return x.Peer
	}
	return ""
}

func (x *ComputedExportedServiceConsumer) GetPartition() string {
	if x, ok := x.GetTenancy().(*ComputedExportedServiceConsumer_Partition); ok {
		return x.Partition
	}
	return ""
}

type isComputedExportedServiceConsumer_Tenancy interface {
	isComputedExportedServiceConsumer_Tenancy()
}

type ComputedExportedServiceConsumer_Peer struct {
	Peer string `protobuf:"bytes,3,opt,name=peer,proto3,oneof"`
}

type ComputedExportedServiceConsumer_Partition struct {
	Partition string `protobuf:"bytes,4,opt,name=partition,proto3,oneof"`
}

func (*ComputedExportedServiceConsumer_Peer) isComputedExportedServiceConsumer_Tenancy() {}

func (*ComputedExportedServiceConsumer_Partition) isComputedExportedServiceConsumer_Tenancy() {}

var File_pbmulticluster_v2beta1_computed_exported_services_proto protoreflect.FileDescriptor

var file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDesc = []byte{
	0x0a, 0x37, 0x70, 0x62, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65,
	0x64, 0x5f, 0x65, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x25, 0x68, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x75, 0x6c, 0x74,
	0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x1a, 0x1c, 0x70, 0x62, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x61, 0x6e, 0x6e,
	0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x19,
	0x70, 0x62, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x7e, 0x0a, 0x18, 0x43, 0x6f, 0x6d,
	0x70, 0x75, 0x74, 0x65, 0x64, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x53, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x73, 0x12, 0x5a, 0x0a, 0x08, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x3e, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x75, 0x6c, 0x74, 0x69,
	0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e,
	0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x64, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x52, 0x08, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x73, 0x3a, 0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x02, 0x22, 0xc4, 0x01, 0x0a, 0x17, 0x43, 0x6f,
	0x6d, 0x70, 0x75, 0x74, 0x65, 0x64, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x53, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x43, 0x0a, 0x0a, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74, 0x5f,
	0x72, 0x65, 0x66, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x72, 0x65, 0x73,
	0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x52,
	0x09, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74, 0x52, 0x65, 0x66, 0x12, 0x64, 0x0a, 0x09, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x46, 0x2e,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x76, 0x32,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x64, 0x45, 0x78,
	0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f, 0x6e,
	0x73, 0x75, 0x6d, 0x65, 0x72, 0x52, 0x09, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6d, 0x65, 0x72, 0x73,
	0x22, 0x62, 0x0a, 0x1f, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x64, 0x45, 0x78, 0x70, 0x6f,
	0x72, 0x74, 0x65, 0x64, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f, 0x6e, 0x73, 0x75,
	0x6d, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x04, 0x70, 0x65, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x48, 0x00, 0x52, 0x04, 0x70, 0x65, 0x65, 0x72, 0x12, 0x1e, 0x0a, 0x09, 0x70, 0x61, 0x72,
	0x74, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x09,
	0x70, 0x61, 0x72, 0x74, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x09, 0x0a, 0x07, 0x74, 0x65, 0x6e,
	0x61, 0x6e, 0x63, 0x79, 0x42, 0xd6, 0x02, 0x0a, 0x29, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73,
	0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x75,
	0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x42, 0x1d, 0x43, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x64, 0x45, 0x78, 0x70, 0x6f,
	0x72, 0x74, 0x65, 0x64, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x50, 0x01, 0x5a, 0x53, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62,
	0x6d, 0x75, 0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2f, 0x76, 0x32, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x3b, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xa2, 0x02, 0x03, 0x48, 0x43, 0x4d, 0xaa, 0x02,
	0x25, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x4d, 0x75, 0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x56,
	0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xca, 0x02, 0x25, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x75, 0x6c, 0x74, 0x69, 0x63,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x5c, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xe2, 0x02,
	0x31, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x5c, 0x4d, 0x75, 0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x5c, 0x56,
	0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61,
	0x74, 0x61, 0xea, 0x02, 0x28, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a,
	0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x75, 0x6c, 0x74, 0x69, 0x63, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x3a, 0x3a, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescOnce sync.Once
	file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescData = file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDesc
)

func file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescGZIP() []byte {
	file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescOnce.Do(func() {
		file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescData)
	})
	return file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDescData
}

var file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_pbmulticluster_v2beta1_computed_exported_services_proto_goTypes = []interface{}{
	(*ComputedExportedServices)(nil),        // 0: hashicorp.consul.multicluster.v2beta1.ComputedExportedServices
	(*ComputedExportedService)(nil),         // 1: hashicorp.consul.multicluster.v2beta1.ComputedExportedService
	(*ComputedExportedServiceConsumer)(nil), // 2: hashicorp.consul.multicluster.v2beta1.ComputedExportedServiceConsumer
	(*pbresource.Reference)(nil),            // 3: hashicorp.consul.resource.Reference
}
var file_pbmulticluster_v2beta1_computed_exported_services_proto_depIdxs = []int32{
	1, // 0: hashicorp.consul.multicluster.v2beta1.ComputedExportedServices.services:type_name -> hashicorp.consul.multicluster.v2beta1.ComputedExportedService
	3, // 1: hashicorp.consul.multicluster.v2beta1.ComputedExportedService.target_ref:type_name -> hashicorp.consul.resource.Reference
	2, // 2: hashicorp.consul.multicluster.v2beta1.ComputedExportedService.consumers:type_name -> hashicorp.consul.multicluster.v2beta1.ComputedExportedServiceConsumer
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_pbmulticluster_v2beta1_computed_exported_services_proto_init() }
func file_pbmulticluster_v2beta1_computed_exported_services_proto_init() {
	if File_pbmulticluster_v2beta1_computed_exported_services_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ComputedExportedServices); i {
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
		file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ComputedExportedService); i {
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
		file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ComputedExportedServiceConsumer); i {
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
	file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes[2].OneofWrappers = []interface{}{
		(*ComputedExportedServiceConsumer_Peer)(nil),
		(*ComputedExportedServiceConsumer_Partition)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmulticluster_v2beta1_computed_exported_services_proto_goTypes,
		DependencyIndexes: file_pbmulticluster_v2beta1_computed_exported_services_proto_depIdxs,
		MessageInfos:      file_pbmulticluster_v2beta1_computed_exported_services_proto_msgTypes,
	}.Build()
	File_pbmulticluster_v2beta1_computed_exported_services_proto = out.File
	file_pbmulticluster_v2beta1_computed_exported_services_proto_rawDesc = nil
	file_pbmulticluster_v2beta1_computed_exported_services_proto_goTypes = nil
	file_pbmulticluster_v2beta1_computed_exported_services_proto_depIdxs = nil
}

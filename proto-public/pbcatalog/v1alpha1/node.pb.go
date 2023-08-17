// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbcatalog/v1alpha1/node.proto

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

type Node struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Addresses []*NodeAddress `protobuf:"bytes,1,rep,name=addresses,proto3" json:"addresses,omitempty"`
}

func (x *Node) Reset() {
	*x = Node{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbcatalog_v1alpha1_node_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Node) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Node) ProtoMessage() {}

func (x *Node) ProtoReflect() protoreflect.Message {
	mi := &file_pbcatalog_v1alpha1_node_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Node.ProtoReflect.Descriptor instead.
func (*Node) Descriptor() ([]byte, []int) {
	return file_pbcatalog_v1alpha1_node_proto_rawDescGZIP(), []int{0}
}

func (x *Node) GetAddresses() []*NodeAddress {
	if x != nil {
		return x.Addresses
	}
	return nil
}

type NodeAddress struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// host can be an IP or DNS name.Í
	Host string `protobuf:"bytes,1,opt,name=host,proto3" json:"host,omitempty"`
	// external indicates whether this address should be used for external communication
	// (aka a WAN address).
	External bool `protobuf:"varint,3,opt,name=external,proto3" json:"external,omitempty"`
}

func (x *NodeAddress) Reset() {
	*x = NodeAddress{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbcatalog_v1alpha1_node_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeAddress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeAddress) ProtoMessage() {}

func (x *NodeAddress) ProtoReflect() protoreflect.Message {
	mi := &file_pbcatalog_v1alpha1_node_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeAddress.ProtoReflect.Descriptor instead.
func (*NodeAddress) Descriptor() ([]byte, []int) {
	return file_pbcatalog_v1alpha1_node_proto_rawDescGZIP(), []int{1}
}

func (x *NodeAddress) GetHost() string {
	if x != nil {
		return x.Host
	}
	return ""
}

func (x *NodeAddress) GetExternal() bool {
	if x != nil {
		return x.External
	}
	return false
}

var File_pbcatalog_v1alpha1_node_proto protoreflect.FileDescriptor

var file_pbcatalog_v1alpha1_node_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x70, 0x62, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2f, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x21, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0x31, 0x22, 0x54, 0x0a, 0x04, 0x4e, 0x6f, 0x64, 0x65, 0x12, 0x4c, 0x0a, 0x09, 0x61, 0x64,
	0x64, 0x72, 0x65, 0x73, 0x73, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x52, 0x09, 0x61,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x65, 0x73, 0x22, 0x3d, 0x0a, 0x0b, 0x4e, 0x6f, 0x64, 0x65,
	0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x68, 0x6f, 0x73, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x6f, 0x73, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x65,
	0x78, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x65,
	0x78, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x42, 0xa6, 0x02, 0x0a, 0x25, 0x63, 0x6f, 0x6d, 0x2e,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x42, 0x09, 0x4e, 0x6f, 0x64, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x4b,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62, 0x63, 0x61, 0x74, 0x61, 0x6c,
	0x6f, 0x67, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x3b, 0x63, 0x61, 0x74, 0x61,
	0x6c, 0x6f, 0x67, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xa2, 0x02, 0x03, 0x48, 0x43,
	0x43, 0xaa, 0x02, 0x21, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x43, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x56, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0xca, 0x02, 0x21, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x43, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67,
	0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xe2, 0x02, 0x2d, 0x48, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x43, 0x61, 0x74,
	0x61, 0x6c, 0x6f, 0x67, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x5c, 0x47, 0x50,
	0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x24, 0x48, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x43,
	0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbcatalog_v1alpha1_node_proto_rawDescOnce sync.Once
	file_pbcatalog_v1alpha1_node_proto_rawDescData = file_pbcatalog_v1alpha1_node_proto_rawDesc
)

func file_pbcatalog_v1alpha1_node_proto_rawDescGZIP() []byte {
	file_pbcatalog_v1alpha1_node_proto_rawDescOnce.Do(func() {
		file_pbcatalog_v1alpha1_node_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbcatalog_v1alpha1_node_proto_rawDescData)
	})
	return file_pbcatalog_v1alpha1_node_proto_rawDescData
}

var file_pbcatalog_v1alpha1_node_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_pbcatalog_v1alpha1_node_proto_goTypes = []interface{}{
	(*Node)(nil),        // 0: hashicorp.consul.catalog.v1alpha1.Node
	(*NodeAddress)(nil), // 1: hashicorp.consul.catalog.v1alpha1.NodeAddress
}
var file_pbcatalog_v1alpha1_node_proto_depIdxs = []int32{
	1, // 0: hashicorp.consul.catalog.v1alpha1.Node.addresses:type_name -> hashicorp.consul.catalog.v1alpha1.NodeAddress
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_pbcatalog_v1alpha1_node_proto_init() }
func file_pbcatalog_v1alpha1_node_proto_init() {
	if File_pbcatalog_v1alpha1_node_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbcatalog_v1alpha1_node_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Node); i {
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
		file_pbcatalog_v1alpha1_node_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeAddress); i {
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
			RawDescriptor: file_pbcatalog_v1alpha1_node_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbcatalog_v1alpha1_node_proto_goTypes,
		DependencyIndexes: file_pbcatalog_v1alpha1_node_proto_depIdxs,
		MessageInfos:      file_pbcatalog_v1alpha1_node_proto_msgTypes,
	}.Build()
	File_pbcatalog_v1alpha1_node_proto = out.File
	file_pbcatalog_v1alpha1_node_proto_rawDesc = nil
	file_pbcatalog_v1alpha1_node_proto_goTypes = nil
	file_pbcatalog_v1alpha1_node_proto_depIdxs = nil
}

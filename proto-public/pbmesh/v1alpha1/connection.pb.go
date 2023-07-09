// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v1alpha1/connection.proto

package meshv1alpha1

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

type BalanceConnections int32

const (
	// buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
	BalanceConnections_BALANCE_CONNECTIONS_DEFAULT BalanceConnections = 0
	BalanceConnections_BALANCE_CONNECTIONS_EXACT   BalanceConnections = 1
)

// Enum value maps for BalanceConnections.
var (
	BalanceConnections_name = map[int32]string{
		0: "BALANCE_CONNECTIONS_DEFAULT",
		1: "BALANCE_CONNECTIONS_EXACT",
	}
	BalanceConnections_value = map[string]int32{
		"BALANCE_CONNECTIONS_DEFAULT": 0,
		"BALANCE_CONNECTIONS_EXACT":   1,
	}
)

func (x BalanceConnections) Enum() *BalanceConnections {
	p := new(BalanceConnections)
	*p = x
	return p
}

func (x BalanceConnections) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (BalanceConnections) Descriptor() protoreflect.EnumDescriptor {
	return file_pbmesh_v1alpha1_connection_proto_enumTypes[0].Descriptor()
}

func (BalanceConnections) Type() protoreflect.EnumType {
	return &file_pbmesh_v1alpha1_connection_proto_enumTypes[0]
}

func (x BalanceConnections) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use BalanceConnections.Descriptor instead.
func (BalanceConnections) EnumDescriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_connection_proto_rawDescGZIP(), []int{0}
}

type ConnectionConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ConnectTimeoutMs uint64 `protobuf:"varint,2,opt,name=connect_timeout_ms,json=connectTimeoutMs,proto3" json:"connect_timeout_ms,omitempty"`
	RequestTimeoutMs uint64 `protobuf:"varint,3,opt,name=request_timeout_ms,json=requestTimeoutMs,proto3" json:"request_timeout_ms,omitempty"`
}

func (x *ConnectionConfig) Reset() {
	*x = ConnectionConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_connection_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ConnectionConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConnectionConfig) ProtoMessage() {}

func (x *ConnectionConfig) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_connection_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConnectionConfig.ProtoReflect.Descriptor instead.
func (*ConnectionConfig) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_connection_proto_rawDescGZIP(), []int{0}
}

func (x *ConnectionConfig) GetConnectTimeoutMs() uint64 {
	if x != nil {
		return x.ConnectTimeoutMs
	}
	return 0
}

func (x *ConnectionConfig) GetRequestTimeoutMs() uint64 {
	if x != nil {
		return x.RequestTimeoutMs
	}
	return 0
}

type InboundConnectionsConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MaxInboundConnections     uint64             `protobuf:"varint,12,opt,name=max_inbound_connections,json=maxInboundConnections,proto3" json:"max_inbound_connections,omitempty"`
	BalanceInboundConnections BalanceConnections `protobuf:"varint,13,opt,name=balance_inbound_connections,json=balanceInboundConnections,proto3,enum=hashicorp.consul.mesh.v1alpha1.BalanceConnections" json:"balance_inbound_connections,omitempty"`
}

func (x *InboundConnectionsConfig) Reset() {
	*x = InboundConnectionsConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_connection_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InboundConnectionsConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InboundConnectionsConfig) ProtoMessage() {}

func (x *InboundConnectionsConfig) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_connection_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InboundConnectionsConfig.ProtoReflect.Descriptor instead.
func (*InboundConnectionsConfig) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_connection_proto_rawDescGZIP(), []int{1}
}

func (x *InboundConnectionsConfig) GetMaxInboundConnections() uint64 {
	if x != nil {
		return x.MaxInboundConnections
	}
	return 0
}

func (x *InboundConnectionsConfig) GetBalanceInboundConnections() BalanceConnections {
	if x != nil {
		return x.BalanceInboundConnections
	}
	return BalanceConnections_BALANCE_CONNECTIONS_DEFAULT
}

var File_pbmesh_v1alpha1_connection_proto protoreflect.FileDescriptor

var file_pbmesh_v1alpha1_connection_proto_rawDesc = []byte{
	0x0a, 0x20, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x2f, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x1e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0x31, 0x22, 0x6e, 0x0a, 0x10, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x2c, 0x0a, 0x12, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63,
	0x74, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x5f, 0x6d, 0x73, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x10, 0x63, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x6f,
	0x75, 0x74, 0x4d, 0x73, 0x12, 0x2c, 0x0a, 0x12, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f,
	0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x5f, 0x6d, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x10, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74,
	0x4d, 0x73, 0x22, 0xc6, 0x01, 0x0a, 0x18, 0x49, 0x6e, 0x62, 0x6f, 0x75, 0x6e, 0x64, 0x43, 0x6f,
	0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12,
	0x36, 0x0a, 0x17, 0x6d, 0x61, 0x78, 0x5f, 0x69, 0x6e, 0x62, 0x6f, 0x75, 0x6e, 0x64, 0x5f, 0x63,
	0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x15, 0x6d, 0x61, 0x78, 0x49, 0x6e, 0x62, 0x6f, 0x75, 0x6e, 0x64, 0x43, 0x6f, 0x6e, 0x6e,
	0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x72, 0x0a, 0x1b, 0x62, 0x61, 0x6c, 0x61, 0x6e,
	0x63, 0x65, 0x5f, 0x69, 0x6e, 0x62, 0x6f, 0x75, 0x6e, 0x64, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x65,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x32, 0x2e, 0x68,
	0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e,
	0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x42, 0x61,
	0x6c, 0x61, 0x6e, 0x63, 0x65, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x52, 0x19, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x49, 0x6e, 0x62, 0x6f, 0x75, 0x6e, 0x64,
	0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2a, 0x54, 0x0a, 0x12, 0x42,
	0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x12, 0x1f, 0x0a, 0x1b, 0x42, 0x41, 0x4c, 0x41, 0x4e, 0x43, 0x45, 0x5f, 0x43, 0x4f, 0x4e,
	0x4e, 0x45, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x53, 0x5f, 0x44, 0x45, 0x46, 0x41, 0x55, 0x4c, 0x54,
	0x10, 0x00, 0x12, 0x1d, 0x0a, 0x19, 0x42, 0x41, 0x4c, 0x41, 0x4e, 0x43, 0x45, 0x5f, 0x43, 0x4f,
	0x4e, 0x4e, 0x45, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x53, 0x5f, 0x45, 0x58, 0x41, 0x43, 0x54, 0x10,
	0x01, 0x42, 0x97, 0x02, 0x0a, 0x22, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x42, 0x0f, 0x43, 0x6f, 0x6e, 0x6e, 0x65, 0x63,
	0x74, 0x69, 0x6f, 0x6e, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x45, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70,
	0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x3b, 0x6d, 0x65, 0x73, 0x68, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68,
	0x61, 0x31, 0xa2, 0x02, 0x03, 0x48, 0x43, 0x4d, 0xaa, 0x02, 0x1e, 0x48, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73, 0x68,
	0x2e, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xca, 0x02, 0x1e, 0x48, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73,
	0x68, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xe2, 0x02, 0x2a, 0x48, 0x61, 0x73,
	0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65,
	0x73, 0x68, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d,
	0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x21, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x65, 0x73,
	0x68, 0x3a, 0x3a, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v1alpha1_connection_proto_rawDescOnce sync.Once
	file_pbmesh_v1alpha1_connection_proto_rawDescData = file_pbmesh_v1alpha1_connection_proto_rawDesc
)

func file_pbmesh_v1alpha1_connection_proto_rawDescGZIP() []byte {
	file_pbmesh_v1alpha1_connection_proto_rawDescOnce.Do(func() {
		file_pbmesh_v1alpha1_connection_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v1alpha1_connection_proto_rawDescData)
	})
	return file_pbmesh_v1alpha1_connection_proto_rawDescData
}

var file_pbmesh_v1alpha1_connection_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pbmesh_v1alpha1_connection_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_pbmesh_v1alpha1_connection_proto_goTypes = []interface{}{
	(BalanceConnections)(0),          // 0: hashicorp.consul.mesh.v1alpha1.BalanceConnections
	(*ConnectionConfig)(nil),         // 1: hashicorp.consul.mesh.v1alpha1.ConnectionConfig
	(*InboundConnectionsConfig)(nil), // 2: hashicorp.consul.mesh.v1alpha1.InboundConnectionsConfig
}
var file_pbmesh_v1alpha1_connection_proto_depIdxs = []int32{
	0, // 0: hashicorp.consul.mesh.v1alpha1.InboundConnectionsConfig.balance_inbound_connections:type_name -> hashicorp.consul.mesh.v1alpha1.BalanceConnections
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_pbmesh_v1alpha1_connection_proto_init() }
func file_pbmesh_v1alpha1_connection_proto_init() {
	if File_pbmesh_v1alpha1_connection_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v1alpha1_connection_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ConnectionConfig); i {
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
		file_pbmesh_v1alpha1_connection_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InboundConnectionsConfig); i {
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
			RawDescriptor: file_pbmesh_v1alpha1_connection_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v1alpha1_connection_proto_goTypes,
		DependencyIndexes: file_pbmesh_v1alpha1_connection_proto_depIdxs,
		EnumInfos:         file_pbmesh_v1alpha1_connection_proto_enumTypes,
		MessageInfos:      file_pbmesh_v1alpha1_connection_proto_msgTypes,
	}.Build()
	File_pbmesh_v1alpha1_connection_proto = out.File
	file_pbmesh_v1alpha1_connection_proto_rawDesc = nil
	file_pbmesh_v1alpha1_connection_proto_goTypes = nil
	file_pbmesh_v1alpha1_connection_proto_depIdxs = nil
}

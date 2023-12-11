// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: pbmesh/v2beta1/pbproxystate/escape_hatches.proto

package pbproxystate

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

type EscapeHatches struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// listener_tracing_json contains user provided tracing configuration.
	ListenerTracingJson string `protobuf:"bytes,1,opt,name=listener_tracing_json,json=listenerTracingJson,proto3" json:"listener_tracing_json,omitempty"`
}

func (x *EscapeHatches) Reset() {
	*x = EscapeHatches{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EscapeHatches) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EscapeHatches) ProtoMessage() {}

func (x *EscapeHatches) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EscapeHatches.ProtoReflect.Descriptor instead.
func (*EscapeHatches) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescGZIP(), []int{0}
}

func (x *EscapeHatches) GetListenerTracingJson() string {
	if x != nil {
		return x.ListenerTracingJson
	}
	return ""
}

var File_pbmesh_v2beta1_pbproxystate_escape_hatches_proto protoreflect.FileDescriptor

var file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDesc = []byte{
	0x0a, 0x30, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2f, 0x65, 0x73,
	0x63, 0x61, 0x70, 0x65, 0x5f, 0x68, 0x61, 0x74, 0x63, 0x68, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x2a, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0x43,
	0x0a, 0x0d, 0x45, 0x73, 0x63, 0x61, 0x70, 0x65, 0x48, 0x61, 0x74, 0x63, 0x68, 0x65, 0x73, 0x12,
	0x32, 0x0a, 0x15, 0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x5f, 0x74, 0x72, 0x61, 0x63,
	0x69, 0x6e, 0x67, 0x5f, 0x6a, 0x73, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x13,
	0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x54, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67, 0x4a,
	0x73, 0x6f, 0x6e, 0x42, 0xd8, 0x02, 0x0a, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73,
	0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78,
	0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x42, 0x12, 0x45, 0x73, 0x63, 0x61, 0x70, 0x65, 0x48, 0x61,
	0x74, 0x63, 0x68, 0x65, 0x73, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x44, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d,
	0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61,
	0x74, 0x65, 0xa2, 0x02, 0x05, 0x48, 0x43, 0x4d, 0x56, 0x50, 0xaa, 0x02, 0x2a, 0x48, 0x61, 0x73,
	0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65,
	0x73, 0x68, 0x2e, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x50, 0x62, 0x70, 0x72, 0x6f,
	0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0xca, 0x02, 0x2a, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c,
	0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73,
	0x74, 0x61, 0x74, 0x65, 0xe2, 0x02, 0x36, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x32, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74,
	0x65, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x2e,
	0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x3a, 0x3a, 0x4d, 0x65, 0x73, 0x68, 0x3a, 0x3a, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x3a, 0x3a, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescOnce sync.Once
	file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescData = file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDesc
)

func file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescGZIP() []byte {
	file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescOnce.Do(func() {
		file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescData)
	})
	return file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDescData
}

var file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_goTypes = []interface{}{
	(*EscapeHatches)(nil), // 0: hashicorp.consul.mesh.v2beta1.pbproxystate.EscapeHatches
}
var file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_init() }
func file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_init() {
	if File_pbmesh_v2beta1_pbproxystate_escape_hatches_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EscapeHatches); i {
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
			RawDescriptor: file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_goTypes,
		DependencyIndexes: file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_depIdxs,
		MessageInfos:      file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_msgTypes,
	}.Build()
	File_pbmesh_v2beta1_pbproxystate_escape_hatches_proto = out.File
	file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_rawDesc = nil
	file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_goTypes = nil
	file_pbmesh_v2beta1_pbproxystate_escape_hatches_proto_depIdxs = nil
}

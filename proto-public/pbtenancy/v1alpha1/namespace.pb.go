// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: pbtenancy/v1alpha1/namespace.proto

package tenancyv1alpha1

import (
	_ "github.com/hashicorp/consul/proto-public/pbresource"
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

// The name of the Namespace is in the outer Resource.ID.Name.
// It must be unique within a partition and must be a
// DNS hostname. There are also other reserved names that may not be used.
type Namespace struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Description is where the user puts any information they want
	// about the namespace. It is not used internally.
	Description string `protobuf:"bytes,1,opt,name=description,proto3" json:"description,omitempty"`
}

func (x *Namespace) Reset() {
	*x = Namespace{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbtenancy_v1alpha1_namespace_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Namespace) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Namespace) ProtoMessage() {}

func (x *Namespace) ProtoReflect() protoreflect.Message {
	mi := &file_pbtenancy_v1alpha1_namespace_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Namespace.ProtoReflect.Descriptor instead.
func (*Namespace) Descriptor() ([]byte, []int) {
	return file_pbtenancy_v1alpha1_namespace_proto_rawDescGZIP(), []int{0}
}

func (x *Namespace) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

var File_pbtenancy_v1alpha1_namespace_proto protoreflect.FileDescriptor

var file_pbtenancy_v1alpha1_namespace_proto_rawDesc = []byte{
	0x0a, 0x22, 0x70, 0x62, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x2f, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x2f, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x21, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e,
	0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x2e, 0x76,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x1a, 0x1c, 0x70, 0x62, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x35, 0x0a, 0x09, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61,
	0x63, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x3a, 0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x02, 0x42, 0xab, 0x02, 0x0a,
	0x25, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x2e, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x42, 0x0e, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x4b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c,
	0x69, 0x63, 0x2f, 0x70, 0x62, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x2f, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x3b, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0xa2, 0x02, 0x03, 0x48, 0x43, 0x54, 0xaa, 0x02, 0x21, 0x48, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x54,
	0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x2e, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xca,
	0x02, 0x21, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x5c, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0xe2, 0x02, 0x2d, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c,
	0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79, 0x5c, 0x56,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0xea, 0x02, 0x24, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a,
	0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x63, 0x79,
	0x3a, 0x3a, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_pbtenancy_v1alpha1_namespace_proto_rawDescOnce sync.Once
	file_pbtenancy_v1alpha1_namespace_proto_rawDescData = file_pbtenancy_v1alpha1_namespace_proto_rawDesc
)

func file_pbtenancy_v1alpha1_namespace_proto_rawDescGZIP() []byte {
	file_pbtenancy_v1alpha1_namespace_proto_rawDescOnce.Do(func() {
		file_pbtenancy_v1alpha1_namespace_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbtenancy_v1alpha1_namespace_proto_rawDescData)
	})
	return file_pbtenancy_v1alpha1_namespace_proto_rawDescData
}

var file_pbtenancy_v1alpha1_namespace_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pbtenancy_v1alpha1_namespace_proto_goTypes = []interface{}{
	(*Namespace)(nil), // 0: hashicorp.consul.tenancy.v1alpha1.Namespace
}
var file_pbtenancy_v1alpha1_namespace_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pbtenancy_v1alpha1_namespace_proto_init() }
func file_pbtenancy_v1alpha1_namespace_proto_init() {
	if File_pbtenancy_v1alpha1_namespace_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbtenancy_v1alpha1_namespace_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Namespace); i {
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
			RawDescriptor: file_pbtenancy_v1alpha1_namespace_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbtenancy_v1alpha1_namespace_proto_goTypes,
		DependencyIndexes: file_pbtenancy_v1alpha1_namespace_proto_depIdxs,
		MessageInfos:      file_pbtenancy_v1alpha1_namespace_proto_msgTypes,
	}.Build()
	File_pbtenancy_v1alpha1_namespace_proto = out.File
	file_pbtenancy_v1alpha1_namespace_proto_rawDesc = nil
	file_pbtenancy_v1alpha1_namespace_proto_goTypes = nil
	file_pbtenancy_v1alpha1_namespace_proto_depIdxs = nil
}

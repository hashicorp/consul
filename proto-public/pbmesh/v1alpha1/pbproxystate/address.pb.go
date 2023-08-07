// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v1alpha1/pbproxystate/address.proto

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

type HostPortAddress struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Host string `protobuf:"bytes,1,opt,name=host,proto3" json:"host,omitempty"`
	Port uint32 `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
}

func (x *HostPortAddress) Reset() {
	*x = HostPortAddress{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HostPortAddress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HostPortAddress) ProtoMessage() {}

func (x *HostPortAddress) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HostPortAddress.ProtoReflect.Descriptor instead.
func (*HostPortAddress) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescGZIP(), []int{0}
}

func (x *HostPortAddress) GetHost() string {
	if x != nil {
		return x.Host
	}
	return ""
}

func (x *HostPortAddress) GetPort() uint32 {
	if x != nil {
		return x.Port
	}
	return 0
}

type UnixSocketAddress struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// path is the file system path at which to bind a Unix domain socket listener.
	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	// mode is the Unix file mode for the socket file. It should be provided
	// in the numeric notation, for example, "0600".
	Mode string `protobuf:"bytes,2,opt,name=mode,proto3" json:"mode,omitempty"`
}

func (x *UnixSocketAddress) Reset() {
	*x = UnixSocketAddress{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnixSocketAddress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnixSocketAddress) ProtoMessage() {}

func (x *UnixSocketAddress) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UnixSocketAddress.ProtoReflect.Descriptor instead.
func (*UnixSocketAddress) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescGZIP(), []int{1}
}

func (x *UnixSocketAddress) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *UnixSocketAddress) GetMode() string {
	if x != nil {
		return x.Mode
	}
	return ""
}

var File_pbmesh_v1alpha1_pbproxystate_address_proto protoreflect.FileDescriptor

var file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDesc = []byte{
	0x0a, 0x2a, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x2f, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2f, 0x61,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x2b, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70,
	0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0x39, 0x0a, 0x0f, 0x48, 0x6f, 0x73,
	0x74, 0x50, 0x6f, 0x72, 0x74, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x12, 0x0a, 0x04,
	0x68, 0x6f, 0x73, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x6f, 0x73, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04,
	0x70, 0x6f, 0x72, 0x74, 0x22, 0x3b, 0x0a, 0x11, 0x55, 0x6e, 0x69, 0x78, 0x53, 0x6f, 0x63, 0x6b,
	0x65, 0x74, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74,
	0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x12, 0x0a,
	0x04, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6d, 0x6f, 0x64,
	0x65, 0x42, 0xd8, 0x02, 0x0a, 0x2f, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x42, 0x0c, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x50, 0x72,
	0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x45, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f,
	0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f,
	0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0xa2, 0x02, 0x05, 0x48,
	0x43, 0x4d, 0x56, 0x50, 0xaa, 0x02, 0x2b, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73, 0x68, 0x2e, 0x56, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61,
	0x74, 0x65, 0xca, 0x02, 0x2b, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65,
	0xe2, 0x02, 0x37, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x5c, 0x47,
	0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x2f, 0x48, 0x61, 0x73,
	0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a,
	0x4d, 0x65, 0x73, 0x68, 0x3a, 0x3a, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x3a, 0x3a,
	0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescOnce sync.Once
	file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescData = file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDesc
)

func file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescGZIP() []byte {
	file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescOnce.Do(func() {
		file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescData)
	})
	return file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDescData
}

var file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_pbmesh_v1alpha1_pbproxystate_address_proto_goTypes = []interface{}{
	(*HostPortAddress)(nil),   // 0: hashicorp.consul.mesh.v1alpha1.pbproxystate.HostPortAddress
	(*UnixSocketAddress)(nil), // 1: hashicorp.consul.mesh.v1alpha1.pbproxystate.UnixSocketAddress
}
var file_pbmesh_v1alpha1_pbproxystate_address_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pbmesh_v1alpha1_pbproxystate_address_proto_init() }
func file_pbmesh_v1alpha1_pbproxystate_address_proto_init() {
	if File_pbmesh_v1alpha1_pbproxystate_address_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HostPortAddress); i {
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
		file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UnixSocketAddress); i {
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
			RawDescriptor: file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v1alpha1_pbproxystate_address_proto_goTypes,
		DependencyIndexes: file_pbmesh_v1alpha1_pbproxystate_address_proto_depIdxs,
		MessageInfos:      file_pbmesh_v1alpha1_pbproxystate_address_proto_msgTypes,
	}.Build()
	File_pbmesh_v1alpha1_pbproxystate_address_proto = out.File
	file_pbmesh_v1alpha1_pbproxystate_address_proto_rawDesc = nil
	file_pbmesh_v1alpha1_pbproxystate_address_proto_goTypes = nil
	file_pbmesh_v1alpha1_pbproxystate_address_proto_depIdxs = nil
}

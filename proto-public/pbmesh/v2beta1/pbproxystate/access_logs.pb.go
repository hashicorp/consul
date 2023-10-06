// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v2beta1/pbproxystate/access_logs.proto

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

type LogSinkType int32

const (
	// buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
	LogSinkType_LOG_SINK_TYPE_DEFAULT LogSinkType = 0
	LogSinkType_LOG_SINK_TYPE_FILE    LogSinkType = 1
	LogSinkType_LOG_SINK_TYPE_STDERR  LogSinkType = 2
	LogSinkType_LOG_SINK_TYPE_STDOUT  LogSinkType = 3
)

// Enum value maps for LogSinkType.
var (
	LogSinkType_name = map[int32]string{
		0: "LOG_SINK_TYPE_DEFAULT",
		1: "LOG_SINK_TYPE_FILE",
		2: "LOG_SINK_TYPE_STDERR",
		3: "LOG_SINK_TYPE_STDOUT",
	}
	LogSinkType_value = map[string]int32{
		"LOG_SINK_TYPE_DEFAULT": 0,
		"LOG_SINK_TYPE_FILE":    1,
		"LOG_SINK_TYPE_STDERR":  2,
		"LOG_SINK_TYPE_STDOUT":  3,
	}
)

func (x LogSinkType) Enum() *LogSinkType {
	p := new(LogSinkType)
	*p = x
	return p
}

func (x LogSinkType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogSinkType) Descriptor() protoreflect.EnumDescriptor {
	return file_pbmesh_v2beta1_pbproxystate_access_logs_proto_enumTypes[0].Descriptor()
}

func (LogSinkType) Type() protoreflect.EnumType {
	return &file_pbmesh_v2beta1_pbproxystate_access_logs_proto_enumTypes[0]
}

func (x LogSinkType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogSinkType.Descriptor instead.
func (LogSinkType) EnumDescriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescGZIP(), []int{0}
}

type AccessLogs struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// enabled enables access logging.
	Enabled bool `protobuf:"varint,1,opt,name=enabled,proto3" json:"enabled,omitempty"`
	// disable_listener_logs turns off just listener logs for connections rejected by Envoy because they don't
	// have a matching listener filter.
	DisableListenerLogs bool `protobuf:"varint,2,opt,name=disable_listener_logs,json=disableListenerLogs,proto3" json:"disable_listener_logs,omitempty"`
	// type selects the output for logs: "file", "stderr". "stdout"
	Type LogSinkType `protobuf:"varint,3,opt,name=type,proto3,enum=hashicorp.consul.mesh.v2beta1.pbproxystate.LogSinkType" json:"type,omitempty"`
	// path is the output file to write logs
	Path string `protobuf:"bytes,4,opt,name=path,proto3" json:"path,omitempty"`
	// The presence of one format string or the other implies the access log string encoding.
	// Defining both is invalid.
	//
	// Types that are assignable to Format:
	//
	//	*AccessLogs_Json
	//	*AccessLogs_Text
	Format isAccessLogs_Format `protobuf_oneof:"format"`
}

func (x *AccessLogs) Reset() {
	*x = AccessLogs{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_access_logs_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AccessLogs) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AccessLogs) ProtoMessage() {}

func (x *AccessLogs) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_access_logs_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AccessLogs.ProtoReflect.Descriptor instead.
func (*AccessLogs) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescGZIP(), []int{0}
}

func (x *AccessLogs) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

func (x *AccessLogs) GetDisableListenerLogs() bool {
	if x != nil {
		return x.DisableListenerLogs
	}
	return false
}

func (x *AccessLogs) GetType() LogSinkType {
	if x != nil {
		return x.Type
	}
	return LogSinkType_LOG_SINK_TYPE_DEFAULT
}

func (x *AccessLogs) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (m *AccessLogs) GetFormat() isAccessLogs_Format {
	if m != nil {
		return m.Format
	}
	return nil
}

func (x *AccessLogs) GetJson() string {
	if x, ok := x.GetFormat().(*AccessLogs_Json); ok {
		return x.Json
	}
	return ""
}

func (x *AccessLogs) GetText() string {
	if x, ok := x.GetFormat().(*AccessLogs_Text); ok {
		return x.Text
	}
	return ""
}

type isAccessLogs_Format interface {
	isAccessLogs_Format()
}

type AccessLogs_Json struct {
	Json string `protobuf:"bytes,5,opt,name=json,proto3,oneof"`
}

type AccessLogs_Text struct {
	Text string `protobuf:"bytes,6,opt,name=text,proto3,oneof"`
}

func (*AccessLogs_Json) isAccessLogs_Format() {}

func (*AccessLogs_Text) isAccessLogs_Format() {}

var File_pbmesh_v2beta1_pbproxystate_access_logs_proto protoreflect.FileDescriptor

var file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDesc = []byte{
	0x0a, 0x2d, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2f, 0x61, 0x63,
	0x63, 0x65, 0x73, 0x73, 0x5f, 0x6c, 0x6f, 0x67, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x2a, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70,
	0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0xf1, 0x01, 0x0a, 0x0a,
	0x41, 0x63, 0x63, 0x65, 0x73, 0x73, 0x4c, 0x6f, 0x67, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x65, 0x6e,
	0x61, 0x62, 0x6c, 0x65, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x65, 0x6e, 0x61,
	0x62, 0x6c, 0x65, 0x64, 0x12, 0x32, 0x0a, 0x15, 0x64, 0x69, 0x73, 0x61, 0x62, 0x6c, 0x65, 0x5f,
	0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x65, 0x72, 0x5f, 0x6c, 0x6f, 0x67, 0x73, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x13, 0x64, 0x69, 0x73, 0x61, 0x62, 0x6c, 0x65, 0x4c, 0x69, 0x73, 0x74,
	0x65, 0x6e, 0x65, 0x72, 0x4c, 0x6f, 0x67, 0x73, 0x12, 0x4b, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x37, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76,
	0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74,
	0x61, 0x74, 0x65, 0x2e, 0x4c, 0x6f, 0x67, 0x53, 0x69, 0x6e, 0x6b, 0x54, 0x79, 0x70, 0x65, 0x52,
	0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x14, 0x0a, 0x04, 0x6a, 0x73, 0x6f,
	0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x04, 0x6a, 0x73, 0x6f, 0x6e, 0x12,
	0x14, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52,
	0x04, 0x74, 0x65, 0x78, 0x74, 0x42, 0x08, 0x0a, 0x06, 0x66, 0x6f, 0x72, 0x6d, 0x61, 0x74, 0x2a,
	0x74, 0x0a, 0x0b, 0x4c, 0x6f, 0x67, 0x53, 0x69, 0x6e, 0x6b, 0x54, 0x79, 0x70, 0x65, 0x12, 0x19,
	0x0a, 0x15, 0x4c, 0x4f, 0x47, 0x5f, 0x53, 0x49, 0x4e, 0x4b, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f,
	0x44, 0x45, 0x46, 0x41, 0x55, 0x4c, 0x54, 0x10, 0x00, 0x12, 0x16, 0x0a, 0x12, 0x4c, 0x4f, 0x47,
	0x5f, 0x53, 0x49, 0x4e, 0x4b, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x46, 0x49, 0x4c, 0x45, 0x10,
	0x01, 0x12, 0x18, 0x0a, 0x14, 0x4c, 0x4f, 0x47, 0x5f, 0x53, 0x49, 0x4e, 0x4b, 0x5f, 0x54, 0x59,
	0x50, 0x45, 0x5f, 0x53, 0x54, 0x44, 0x45, 0x52, 0x52, 0x10, 0x02, 0x12, 0x18, 0x0a, 0x14, 0x4c,
	0x4f, 0x47, 0x5f, 0x53, 0x49, 0x4e, 0x4b, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x53, 0x54, 0x44,
	0x4f, 0x55, 0x54, 0x10, 0x03, 0x42, 0xd5, 0x02, 0x0a, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72,
	0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x42, 0x0f, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73,
	0x4c, 0x6f, 0x67, 0x73, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x44, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70,
	0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x2f, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74,
	0x65, 0xa2, 0x02, 0x05, 0x48, 0x43, 0x4d, 0x56, 0x50, 0xaa, 0x02, 0x2a, 0x48, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73,
	0x68, 0x2e, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78,
	0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0xca, 0x02, 0x2a, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56,
	0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74,
	0x61, 0x74, 0x65, 0xe2, 0x02, 0x36, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c,
	0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x32, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65,
	0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x2e, 0x48,
	0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x3a, 0x3a, 0x4d, 0x65, 0x73, 0x68, 0x3a, 0x3a, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x3a,
	0x3a, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescOnce sync.Once
	file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescData = file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDesc
)

func file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescGZIP() []byte {
	file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescOnce.Do(func() {
		file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescData)
	})
	return file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDescData
}

var file_pbmesh_v2beta1_pbproxystate_access_logs_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pbmesh_v2beta1_pbproxystate_access_logs_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pbmesh_v2beta1_pbproxystate_access_logs_proto_goTypes = []interface{}{
	(LogSinkType)(0),   // 0: hashicorp.consul.mesh.v2beta1.pbproxystate.LogSinkType
	(*AccessLogs)(nil), // 1: hashicorp.consul.mesh.v2beta1.pbproxystate.AccessLogs
}
var file_pbmesh_v2beta1_pbproxystate_access_logs_proto_depIdxs = []int32{
	0, // 0: hashicorp.consul.mesh.v2beta1.pbproxystate.AccessLogs.type:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.LogSinkType
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_pbmesh_v2beta1_pbproxystate_access_logs_proto_init() }
func file_pbmesh_v2beta1_pbproxystate_access_logs_proto_init() {
	if File_pbmesh_v2beta1_pbproxystate_access_logs_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v2beta1_pbproxystate_access_logs_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AccessLogs); i {
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
	file_pbmesh_v2beta1_pbproxystate_access_logs_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*AccessLogs_Json)(nil),
		(*AccessLogs_Text)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v2beta1_pbproxystate_access_logs_proto_goTypes,
		DependencyIndexes: file_pbmesh_v2beta1_pbproxystate_access_logs_proto_depIdxs,
		EnumInfos:         file_pbmesh_v2beta1_pbproxystate_access_logs_proto_enumTypes,
		MessageInfos:      file_pbmesh_v2beta1_pbproxystate_access_logs_proto_msgTypes,
	}.Build()
	File_pbmesh_v2beta1_pbproxystate_access_logs_proto = out.File
	file_pbmesh_v2beta1_pbproxystate_access_logs_proto_rawDesc = nil
	file_pbmesh_v2beta1_pbproxystate_access_logs_proto_goTypes = nil
	file_pbmesh_v2beta1_pbproxystate_access_logs_proto_depIdxs = nil
}

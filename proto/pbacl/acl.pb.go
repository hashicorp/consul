// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        (unknown)
// source: proto/pbacl/acl.proto

package pbacl

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

type ACLLink struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID string `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	// @gotags: hash:ignore-"
	Name string `protobuf:"bytes,2,opt,name=Name,proto3" json:"Name,omitempty"`
}

func (x *ACLLink) Reset() {
	*x = ACLLink{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_pbacl_acl_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ACLLink) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ACLLink) ProtoMessage() {}

func (x *ACLLink) ProtoReflect() protoreflect.Message {
	mi := &file_proto_pbacl_acl_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ACLLink.ProtoReflect.Descriptor instead.
func (*ACLLink) Descriptor() ([]byte, []int) {
	return file_proto_pbacl_acl_proto_rawDescGZIP(), []int{0}
}

func (x *ACLLink) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *ACLLink) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

var File_proto_pbacl_acl_proto protoreflect.FileDescriptor

var file_proto_pbacl_acl_proto_rawDesc = []byte{
	0x0a, 0x15, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x62, 0x61, 0x63, 0x6c, 0x2f, 0x61, 0x63,
	0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1d, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e,
	0x61, 0x6c, 0x2e, 0x61, 0x63, 0x6c, 0x22, 0x2d, 0x0a, 0x07, 0x41, 0x43, 0x4c, 0x4c, 0x69, 0x6e,
	0x6b, 0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x49,
	0x44, 0x12, 0x12, 0x0a, 0x04, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0xee, 0x01, 0x0a, 0x21, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x61, 0x63, 0x6c, 0x42, 0x08, 0x41, 0x63, 0x6c,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x27, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x62, 0x61, 0x63, 0x6c,
	0xa2, 0x02, 0x04, 0x48, 0x43, 0x49, 0x41, 0xaa, 0x02, 0x1d, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x49, 0x6e, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x2e, 0x41, 0x63, 0x6c, 0xca, 0x02, 0x1d, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x49, 0x6e, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x5c, 0x41, 0x63, 0x6c, 0xe2, 0x02, 0x29, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x49, 0x6e, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x5c, 0x41, 0x63, 0x6c, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0xea, 0x02, 0x20, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a,
	0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61,
	0x6c, 0x3a, 0x3a, 0x41, 0x63, 0x6c, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_pbacl_acl_proto_rawDescOnce sync.Once
	file_proto_pbacl_acl_proto_rawDescData = file_proto_pbacl_acl_proto_rawDesc
)

func file_proto_pbacl_acl_proto_rawDescGZIP() []byte {
	file_proto_pbacl_acl_proto_rawDescOnce.Do(func() {
		file_proto_pbacl_acl_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_pbacl_acl_proto_rawDescData)
	})
	return file_proto_pbacl_acl_proto_rawDescData
}

var file_proto_pbacl_acl_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_proto_pbacl_acl_proto_goTypes = []interface{}{
	(*ACLLink)(nil), // 0: hashicorp.consul.internal.acl.ACLLink
}
var file_proto_pbacl_acl_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_proto_pbacl_acl_proto_init() }
func file_proto_pbacl_acl_proto_init() {
	if File_proto_pbacl_acl_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_pbacl_acl_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ACLLink); i {
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
			RawDescriptor: file_proto_pbacl_acl_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_pbacl_acl_proto_goTypes,
		DependencyIndexes: file_proto_pbacl_acl_proto_depIdxs,
		MessageInfos:      file_proto_pbacl_acl_proto_msgTypes,
	}.Build()
	File_proto_pbacl_acl_proto = out.File
	file_proto_pbacl_acl_proto_rawDesc = nil
	file_proto_pbacl_acl_proto_goTypes = nil
	file_proto_pbacl_acl_proto_depIdxs = nil
}

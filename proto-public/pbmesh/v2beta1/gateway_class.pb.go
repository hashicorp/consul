// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v2beta1/gateway_class.proto

package meshv2beta1

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

// NOTE: this should align to the GAMMA/gateway-api version, or at least be
// easily translatable.
//
// https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1alpha2.GatewayClass
//
// This is a Resource type.
type GatewayClass struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ControllerName is the name of the controller that manages Gateways of this class
	ControllerName string `protobuf:"bytes,1,opt,name=controllerName,proto3" json:"controllerName,omitempty"`
	// ParametersRef is a reference to a resource that contains the configuration
	// parameters corresponding to the GatewayClass.
	ParametersRef *ParametersReference `protobuf:"bytes,2,opt,name=parametersRef,proto3" json:"parametersRef,omitempty"`
	// Description of GatewayClass
	Description string `protobuf:"bytes,3,opt,name=description,proto3" json:"description,omitempty"`
}

func (x *GatewayClass) Reset() {
	*x = GatewayClass{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_gateway_class_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GatewayClass) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GatewayClass) ProtoMessage() {}

func (x *GatewayClass) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_gateway_class_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GatewayClass.ProtoReflect.Descriptor instead.
func (*GatewayClass) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_gateway_class_proto_rawDescGZIP(), []int{0}
}

func (x *GatewayClass) GetControllerName() string {
	if x != nil {
		return x.ControllerName
	}
	return ""
}

func (x *GatewayClass) GetParametersRef() *ParametersReference {
	if x != nil {
		return x.ParametersRef
	}
	return nil
}

func (x *GatewayClass) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

// NOTE: this should align to the GAMMA/gateway-api version, or at least be
// easily translatable.
//
// ParametersReference specifies a resource that contains controller-specific configuration
// for a resource
// https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.ParametersReference
type ParametersReference struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The Kubernetes Group that the referred object belongs to
	Group string `protobuf:"bytes,1,opt,name=group,proto3" json:"group,omitempty"`
	// The Kubernetes Kind that the referred object is
	Kind string `protobuf:"bytes,2,opt,name=kind,proto3" json:"kind,omitempty"`
	// The name of the referred object
	Name string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	// The kubernetes namespace that the referred object is in
	Namespace string `protobuf:"bytes,4,opt,name=namespace,proto3" json:"namespace,omitempty"`
}

func (x *ParametersReference) Reset() {
	*x = ParametersReference{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_gateway_class_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ParametersReference) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ParametersReference) ProtoMessage() {}

func (x *ParametersReference) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_gateway_class_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ParametersReference.ProtoReflect.Descriptor instead.
func (*ParametersReference) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_gateway_class_proto_rawDescGZIP(), []int{1}
}

func (x *ParametersReference) GetGroup() string {
	if x != nil {
		return x.Group
	}
	return ""
}

func (x *ParametersReference) GetKind() string {
	if x != nil {
		return x.Kind
	}
	return ""
}

func (x *ParametersReference) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ParametersReference) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

var File_pbmesh_v2beta1_gateway_class_proto protoreflect.FileDescriptor

var file_pbmesh_v2beta1_gateway_class_proto_rawDesc = []byte{
	0x0a, 0x22, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x67, 0x61, 0x74, 0x65, 0x77, 0x61, 0x79, 0x5f, 0x63, 0x6c, 0x61, 0x73, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1d, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e,
	0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x1a, 0x1c, 0x70, 0x62, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f,
	0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0xba, 0x01, 0x0a, 0x0c, 0x47, 0x61, 0x74, 0x65, 0x77, 0x61, 0x79, 0x43, 0x6c, 0x61,
	0x73, 0x73, 0x12, 0x26, 0x0a, 0x0e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72,
	0x4e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0e, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x58, 0x0a, 0x0d, 0x70, 0x61,
	0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x52, 0x65, 0x66, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x32, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x52, 0x65, 0x66, 0x65,
	0x72, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x0d, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72,
	0x73, 0x52, 0x65, 0x66, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72,
	0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x3a, 0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x01, 0x22, 0x71,
	0x0a, 0x13, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x73, 0x52, 0x65, 0x66, 0x65,
	0x72, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x6b,
	0x69, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6b, 0x69, 0x6e, 0x64, 0x12,
	0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x42, 0x92, 0x02, 0x0a, 0x21, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e,
	0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x42, 0x11, 0x47, 0x61, 0x74, 0x65, 0x77, 0x61, 0x79,
	0x43, 0x6c, 0x61, 0x73, 0x73, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x43, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d,
	0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x3b, 0x6d, 0x65, 0x73, 0x68, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61,
	0x31, 0xa2, 0x02, 0x03, 0x48, 0x43, 0x4d, 0xaa, 0x02, 0x1d, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73, 0x68, 0x2e,
	0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xca, 0x02, 0x1d, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c,
	0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xe2, 0x02, 0x29, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c,
	0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0xea, 0x02, 0x20, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a,
	0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x65, 0x73, 0x68, 0x3a, 0x3a, 0x56,
	0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v2beta1_gateway_class_proto_rawDescOnce sync.Once
	file_pbmesh_v2beta1_gateway_class_proto_rawDescData = file_pbmesh_v2beta1_gateway_class_proto_rawDesc
)

func file_pbmesh_v2beta1_gateway_class_proto_rawDescGZIP() []byte {
	file_pbmesh_v2beta1_gateway_class_proto_rawDescOnce.Do(func() {
		file_pbmesh_v2beta1_gateway_class_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v2beta1_gateway_class_proto_rawDescData)
	})
	return file_pbmesh_v2beta1_gateway_class_proto_rawDescData
}

var file_pbmesh_v2beta1_gateway_class_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_pbmesh_v2beta1_gateway_class_proto_goTypes = []interface{}{
	(*GatewayClass)(nil),        // 0: hashicorp.consul.mesh.v2beta1.GatewayClass
	(*ParametersReference)(nil), // 1: hashicorp.consul.mesh.v2beta1.ParametersReference
}
var file_pbmesh_v2beta1_gateway_class_proto_depIdxs = []int32{
	1, // 0: hashicorp.consul.mesh.v2beta1.GatewayClass.parametersRef:type_name -> hashicorp.consul.mesh.v2beta1.ParametersReference
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_pbmesh_v2beta1_gateway_class_proto_init() }
func file_pbmesh_v2beta1_gateway_class_proto_init() {
	if File_pbmesh_v2beta1_gateway_class_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v2beta1_gateway_class_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GatewayClass); i {
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
		file_pbmesh_v2beta1_gateway_class_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ParametersReference); i {
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
			RawDescriptor: file_pbmesh_v2beta1_gateway_class_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v2beta1_gateway_class_proto_goTypes,
		DependencyIndexes: file_pbmesh_v2beta1_gateway_class_proto_depIdxs,
		MessageInfos:      file_pbmesh_v2beta1_gateway_class_proto_msgTypes,
	}.Build()
	File_pbmesh_v2beta1_gateway_class_proto = out.File
	file_pbmesh_v2beta1_gateway_class_proto_rawDesc = nil
	file_pbmesh_v2beta1_gateway_class_proto_goTypes = nil
	file_pbmesh_v2beta1_gateway_class_proto_depIdxs = nil
}

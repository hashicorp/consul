// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v2beta1/routing.proto

package meshv2beta1

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

type MeshGatewayMode int32

const (
	// MESH_GATEWAY_MODE_UNSPECIFIED represents no specific mode and should be
	// used to indicate that a the decision on the mode will be made by other
	// configuration or default settings.
	MeshGatewayMode_MESH_GATEWAY_MODE_UNSPECIFIED MeshGatewayMode = 0
	// MESH_GATEWAY_MODE_NONE is the mode to use when traffic should not be
	// routed through any gateway but instead be routed directly to the
	// destination.
	MeshGatewayMode_MESH_GATEWAY_MODE_NONE MeshGatewayMode = 1
	// MESH_GATEWAY_MODE_LOCAL is the mode to use when traffic should be routed
	// to the local gateway. The local gateway will then ensure that the
	// connection is proxied correctly to its final destination. This mode will
	// most often be needed for workloads that are prevented from making outbound
	// requests outside of their local network/environment. In this case a
	// gateway will sit at the edge of sit at the edge of the network and will
	// proxy outbound connections potentially to other gateways in remote
	// environments.
	MeshGatewayMode_MESH_GATEWAY_MODE_LOCAL MeshGatewayMode = 2
	// MESH_GATEWAY_MODE_REMOTE is the mode to use when traffic should be routed
	// to a remote mesh gateway. This mode will most often be used when workloads
	// can make outbound requests destined for a remote network/environment but
	// where the remote network/environment will not allow direct addressing. The
	// mesh gateway in the remote environment will sit at the edge and proxy
	// requests into that environment.
	MeshGatewayMode_MESH_GATEWAY_MODE_REMOTE MeshGatewayMode = 3
)

// Enum value maps for MeshGatewayMode.
var (
	MeshGatewayMode_name = map[int32]string{
		0: "MESH_GATEWAY_MODE_UNSPECIFIED",
		1: "MESH_GATEWAY_MODE_NONE",
		2: "MESH_GATEWAY_MODE_LOCAL",
		3: "MESH_GATEWAY_MODE_REMOTE",
	}
	MeshGatewayMode_value = map[string]int32{
		"MESH_GATEWAY_MODE_UNSPECIFIED": 0,
		"MESH_GATEWAY_MODE_NONE":        1,
		"MESH_GATEWAY_MODE_LOCAL":       2,
		"MESH_GATEWAY_MODE_REMOTE":      3,
	}
)

func (x MeshGatewayMode) Enum() *MeshGatewayMode {
	p := new(MeshGatewayMode)
	*p = x
	return p
}

func (x MeshGatewayMode) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (MeshGatewayMode) Descriptor() protoreflect.EnumDescriptor {
	return file_pbmesh_v2beta1_routing_proto_enumTypes[0].Descriptor()
}

func (MeshGatewayMode) Type() protoreflect.EnumType {
	return &file_pbmesh_v2beta1_routing_proto_enumTypes[0]
}

func (x MeshGatewayMode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use MeshGatewayMode.Descriptor instead.
func (MeshGatewayMode) EnumDescriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_routing_proto_rawDescGZIP(), []int{0}
}

var File_pbmesh_v2beta1_routing_proto protoreflect.FileDescriptor

var file_pbmesh_v2beta1_routing_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1d,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2a, 0x8b, 0x01,
	0x0a, 0x0f, 0x4d, 0x65, 0x73, 0x68, 0x47, 0x61, 0x74, 0x65, 0x77, 0x61, 0x79, 0x4d, 0x6f, 0x64,
	0x65, 0x12, 0x21, 0x0a, 0x1d, 0x4d, 0x45, 0x53, 0x48, 0x5f, 0x47, 0x41, 0x54, 0x45, 0x57, 0x41,
	0x59, 0x5f, 0x4d, 0x4f, 0x44, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49,
	0x45, 0x44, 0x10, 0x00, 0x12, 0x1a, 0x0a, 0x16, 0x4d, 0x45, 0x53, 0x48, 0x5f, 0x47, 0x41, 0x54,
	0x45, 0x57, 0x41, 0x59, 0x5f, 0x4d, 0x4f, 0x44, 0x45, 0x5f, 0x4e, 0x4f, 0x4e, 0x45, 0x10, 0x01,
	0x12, 0x1b, 0x0a, 0x17, 0x4d, 0x45, 0x53, 0x48, 0x5f, 0x47, 0x41, 0x54, 0x45, 0x57, 0x41, 0x59,
	0x5f, 0x4d, 0x4f, 0x44, 0x45, 0x5f, 0x4c, 0x4f, 0x43, 0x41, 0x4c, 0x10, 0x02, 0x12, 0x1c, 0x0a,
	0x18, 0x4d, 0x45, 0x53, 0x48, 0x5f, 0x47, 0x41, 0x54, 0x45, 0x57, 0x41, 0x59, 0x5f, 0x4d, 0x4f,
	0x44, 0x45, 0x5f, 0x52, 0x45, 0x4d, 0x4f, 0x54, 0x45, 0x10, 0x03, 0x42, 0x8d, 0x02, 0x0a, 0x21,
	0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x42, 0x0c, 0x52, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50,
	0x01, 0x5a, 0x43, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62, 0x6d, 0x65,
	0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x3b, 0x6d, 0x65, 0x73, 0x68, 0x76,
	0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xa2, 0x02, 0x03, 0x48, 0x43, 0x4d, 0xaa, 0x02, 0x1d, 0x48,
	0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e,
	0x4d, 0x65, 0x73, 0x68, 0x2e, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xca, 0x02, 0x1d, 0x48,
	0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c,
	0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xe2, 0x02, 0x29, 0x48,
	0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c,
	0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x20, 0x48, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x65,
	0x73, 0x68, 0x3a, 0x3a, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v2beta1_routing_proto_rawDescOnce sync.Once
	file_pbmesh_v2beta1_routing_proto_rawDescData = file_pbmesh_v2beta1_routing_proto_rawDesc
)

func file_pbmesh_v2beta1_routing_proto_rawDescGZIP() []byte {
	file_pbmesh_v2beta1_routing_proto_rawDescOnce.Do(func() {
		file_pbmesh_v2beta1_routing_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v2beta1_routing_proto_rawDescData)
	})
	return file_pbmesh_v2beta1_routing_proto_rawDescData
}

var file_pbmesh_v2beta1_routing_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pbmesh_v2beta1_routing_proto_goTypes = []interface{}{
	(MeshGatewayMode)(0), // 0: hashicorp.consul.mesh.v2beta1.MeshGatewayMode
}
var file_pbmesh_v2beta1_routing_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pbmesh_v2beta1_routing_proto_init() }
func file_pbmesh_v2beta1_routing_proto_init() {
	if File_pbmesh_v2beta1_routing_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbmesh_v2beta1_routing_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v2beta1_routing_proto_goTypes,
		DependencyIndexes: file_pbmesh_v2beta1_routing_proto_depIdxs,
		EnumInfos:         file_pbmesh_v2beta1_routing_proto_enumTypes,
	}.Build()
	File_pbmesh_v2beta1_routing_proto = out.File
	file_pbmesh_v2beta1_routing_proto_rawDesc = nil
	file_pbmesh_v2beta1_routing_proto_goTypes = nil
	file_pbmesh_v2beta1_routing_proto_depIdxs = nil
}

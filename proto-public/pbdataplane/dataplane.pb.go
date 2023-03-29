// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package dataplane provides a service on Consul servers for the Consul Dataplane

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        (unknown)
// source: pbdataplane/dataplane.proto

package pbdataplane

import (
	_ "github.com/hashicorp/consul/proto-public/annotations/ratelimit"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type DataplaneFeatures int32

const (
	DataplaneFeatures_DATAPLANE_FEATURES_UNSPECIFIED                   DataplaneFeatures = 0
	DataplaneFeatures_DATAPLANE_FEATURES_WATCH_SERVERS                 DataplaneFeatures = 1
	DataplaneFeatures_DATAPLANE_FEATURES_EDGE_CERTIFICATE_MANAGEMENT   DataplaneFeatures = 2
	DataplaneFeatures_DATAPLANE_FEATURES_ENVOY_BOOTSTRAP_CONFIGURATION DataplaneFeatures = 3
)

// Enum value maps for DataplaneFeatures.
var (
	DataplaneFeatures_name = map[int32]string{
		0: "DATAPLANE_FEATURES_UNSPECIFIED",
		1: "DATAPLANE_FEATURES_WATCH_SERVERS",
		2: "DATAPLANE_FEATURES_EDGE_CERTIFICATE_MANAGEMENT",
		3: "DATAPLANE_FEATURES_ENVOY_BOOTSTRAP_CONFIGURATION",
	}
	DataplaneFeatures_value = map[string]int32{
		"DATAPLANE_FEATURES_UNSPECIFIED":                   0,
		"DATAPLANE_FEATURES_WATCH_SERVERS":                 1,
		"DATAPLANE_FEATURES_EDGE_CERTIFICATE_MANAGEMENT":   2,
		"DATAPLANE_FEATURES_ENVOY_BOOTSTRAP_CONFIGURATION": 3,
	}
)

func (x DataplaneFeatures) Enum() *DataplaneFeatures {
	p := new(DataplaneFeatures)
	*p = x
	return p
}

func (x DataplaneFeatures) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (DataplaneFeatures) Descriptor() protoreflect.EnumDescriptor {
	return file_pbdataplane_dataplane_proto_enumTypes[0].Descriptor()
}

func (DataplaneFeatures) Type() protoreflect.EnumType {
	return &file_pbdataplane_dataplane_proto_enumTypes[0]
}

func (x DataplaneFeatures) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use DataplaneFeatures.Descriptor instead.
func (DataplaneFeatures) EnumDescriptor() ([]byte, []int) {
	return file_pbdataplane_dataplane_proto_rawDescGZIP(), []int{0}
}

type ServiceKind int32

const (
	// ServiceKind UNSPECIFIED is a sentinel value for when a request
	// did not specify a service kind. This will be treated the same
	// as if TYPICAL was explicitly used.
	ServiceKind_SERVICE_KIND_UNSPECIFIED ServiceKind = 0
	// ServiceKind Typical is a typical, classic Consul service. This is
	// represented by the absence of a value. This was chosen for ease of
	// backwards compatibility: existing services in the catalog would
	// default to the typical service.
	ServiceKind_SERVICE_KIND_TYPICAL ServiceKind = 1
	// ServiceKind Connect Proxy is a proxy for the Connect feature. This
	// service proxies another service within Consul and speaks the connect
	// protocol.
	ServiceKind_SERVICE_KIND_CONNECT_PROXY ServiceKind = 2
	// ServiceKind Mesh Gateway is a Mesh Gateway for the Connect feature. This
	// service will proxy connections based off the SNI header set by other
	// connect proxies.
	ServiceKind_SERVICE_KIND_MESH_GATEWAY ServiceKind = 3
	// ServiceKind Terminating Gateway is a Terminating Gateway for the Connect
	// feature. This service will proxy connections to services outside the mesh.
	ServiceKind_SERVICE_KIND_TERMINATING_GATEWAY ServiceKind = 4
	// ServiceKind Ingress Gateway is an Ingress Gateway for the Connect feature.
	// This service will ingress connections into the service mesh.
	ServiceKind_SERVICE_KIND_INGRESS_GATEWAY ServiceKind = 5
	// ServiceKind API Gateway is an API Gateway for the Connect feature.
	// This service will ingress connections in to the service mesh.
	ServiceKind_SERVICE_KIND_API_GATEWAY ServiceKind = 6
)

// Enum value maps for ServiceKind.
var (
	ServiceKind_name = map[int32]string{
		0: "SERVICE_KIND_UNSPECIFIED",
		1: "SERVICE_KIND_TYPICAL",
		2: "SERVICE_KIND_CONNECT_PROXY",
		3: "SERVICE_KIND_MESH_GATEWAY",
		4: "SERVICE_KIND_TERMINATING_GATEWAY",
		5: "SERVICE_KIND_INGRESS_GATEWAY",
		6: "SERVICE_KIND_API_GATEWAY",
	}
	ServiceKind_value = map[string]int32{
		"SERVICE_KIND_UNSPECIFIED":         0,
		"SERVICE_KIND_TYPICAL":             1,
		"SERVICE_KIND_CONNECT_PROXY":       2,
		"SERVICE_KIND_MESH_GATEWAY":        3,
		"SERVICE_KIND_TERMINATING_GATEWAY": 4,
		"SERVICE_KIND_INGRESS_GATEWAY":     5,
		"SERVICE_KIND_API_GATEWAY":         6,
	}
)

func (x ServiceKind) Enum() *ServiceKind {
	p := new(ServiceKind)
	*p = x
	return p
}

func (x ServiceKind) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ServiceKind) Descriptor() protoreflect.EnumDescriptor {
	return file_pbdataplane_dataplane_proto_enumTypes[1].Descriptor()
}

func (ServiceKind) Type() protoreflect.EnumType {
	return &file_pbdataplane_dataplane_proto_enumTypes[1]
}

func (x ServiceKind) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ServiceKind.Descriptor instead.
func (ServiceKind) EnumDescriptor() ([]byte, []int) {
	return file_pbdataplane_dataplane_proto_rawDescGZIP(), []int{1}
}

type GetSupportedDataplaneFeaturesRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetSupportedDataplaneFeaturesRequest) Reset() {
	*x = GetSupportedDataplaneFeaturesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbdataplane_dataplane_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetSupportedDataplaneFeaturesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSupportedDataplaneFeaturesRequest) ProtoMessage() {}

func (x *GetSupportedDataplaneFeaturesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pbdataplane_dataplane_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetSupportedDataplaneFeaturesRequest.ProtoReflect.Descriptor instead.
func (*GetSupportedDataplaneFeaturesRequest) Descriptor() ([]byte, []int) {
	return file_pbdataplane_dataplane_proto_rawDescGZIP(), []int{0}
}

type DataplaneFeatureSupport struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FeatureName DataplaneFeatures `protobuf:"varint,1,opt,name=feature_name,json=featureName,proto3,enum=hashicorp.consul.dataplane.DataplaneFeatures" json:"feature_name,omitempty"`
	Supported   bool              `protobuf:"varint,2,opt,name=supported,proto3" json:"supported,omitempty"`
}

func (x *DataplaneFeatureSupport) Reset() {
	*x = DataplaneFeatureSupport{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbdataplane_dataplane_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataplaneFeatureSupport) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataplaneFeatureSupport) ProtoMessage() {}

func (x *DataplaneFeatureSupport) ProtoReflect() protoreflect.Message {
	mi := &file_pbdataplane_dataplane_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataplaneFeatureSupport.ProtoReflect.Descriptor instead.
func (*DataplaneFeatureSupport) Descriptor() ([]byte, []int) {
	return file_pbdataplane_dataplane_proto_rawDescGZIP(), []int{1}
}

func (x *DataplaneFeatureSupport) GetFeatureName() DataplaneFeatures {
	if x != nil {
		return x.FeatureName
	}
	return DataplaneFeatures_DATAPLANE_FEATURES_UNSPECIFIED
}

func (x *DataplaneFeatureSupport) GetSupported() bool {
	if x != nil {
		return x.Supported
	}
	return false
}

type GetSupportedDataplaneFeaturesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SupportedDataplaneFeatures []*DataplaneFeatureSupport `protobuf:"bytes,1,rep,name=supported_dataplane_features,json=supportedDataplaneFeatures,proto3" json:"supported_dataplane_features,omitempty"`
}

func (x *GetSupportedDataplaneFeaturesResponse) Reset() {
	*x = GetSupportedDataplaneFeaturesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbdataplane_dataplane_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetSupportedDataplaneFeaturesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSupportedDataplaneFeaturesResponse) ProtoMessage() {}

func (x *GetSupportedDataplaneFeaturesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pbdataplane_dataplane_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetSupportedDataplaneFeaturesResponse.ProtoReflect.Descriptor instead.
func (*GetSupportedDataplaneFeaturesResponse) Descriptor() ([]byte, []int) {
	return file_pbdataplane_dataplane_proto_rawDescGZIP(), []int{2}
}

func (x *GetSupportedDataplaneFeaturesResponse) GetSupportedDataplaneFeatures() []*DataplaneFeatureSupport {
	if x != nil {
		return x.SupportedDataplaneFeatures
	}
	return nil
}

type GetEnvoyBootstrapParamsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to NodeSpec:
	//
	//	*GetEnvoyBootstrapParamsRequest_NodeId
	//	*GetEnvoyBootstrapParamsRequest_NodeName
	NodeSpec isGetEnvoyBootstrapParamsRequest_NodeSpec `protobuf_oneof:"node_spec"`
	// The proxy service ID
	ServiceId string `protobuf:"bytes,3,opt,name=service_id,json=serviceId,proto3" json:"service_id,omitempty"`
	Partition string `protobuf:"bytes,4,opt,name=partition,proto3" json:"partition,omitempty"`
	Namespace string `protobuf:"bytes,5,opt,name=namespace,proto3" json:"namespace,omitempty"`
}

func (x *GetEnvoyBootstrapParamsRequest) Reset() {
	*x = GetEnvoyBootstrapParamsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbdataplane_dataplane_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetEnvoyBootstrapParamsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetEnvoyBootstrapParamsRequest) ProtoMessage() {}

func (x *GetEnvoyBootstrapParamsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pbdataplane_dataplane_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetEnvoyBootstrapParamsRequest.ProtoReflect.Descriptor instead.
func (*GetEnvoyBootstrapParamsRequest) Descriptor() ([]byte, []int) {
	return file_pbdataplane_dataplane_proto_rawDescGZIP(), []int{3}
}

func (m *GetEnvoyBootstrapParamsRequest) GetNodeSpec() isGetEnvoyBootstrapParamsRequest_NodeSpec {
	if m != nil {
		return m.NodeSpec
	}
	return nil
}

func (x *GetEnvoyBootstrapParamsRequest) GetNodeId() string {
	if x, ok := x.GetNodeSpec().(*GetEnvoyBootstrapParamsRequest_NodeId); ok {
		return x.NodeId
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsRequest) GetNodeName() string {
	if x, ok := x.GetNodeSpec().(*GetEnvoyBootstrapParamsRequest_NodeName); ok {
		return x.NodeName
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsRequest) GetServiceId() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsRequest) GetPartition() string {
	if x != nil {
		return x.Partition
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

type isGetEnvoyBootstrapParamsRequest_NodeSpec interface {
	isGetEnvoyBootstrapParamsRequest_NodeSpec()
}

type GetEnvoyBootstrapParamsRequest_NodeId struct {
	NodeId string `protobuf:"bytes,1,opt,name=node_id,json=nodeId,proto3,oneof"`
}

type GetEnvoyBootstrapParamsRequest_NodeName struct {
	NodeName string `protobuf:"bytes,2,opt,name=node_name,json=nodeName,proto3,oneof"`
}

func (*GetEnvoyBootstrapParamsRequest_NodeId) isGetEnvoyBootstrapParamsRequest_NodeSpec() {}

func (*GetEnvoyBootstrapParamsRequest_NodeName) isGetEnvoyBootstrapParamsRequest_NodeSpec() {}

type GetEnvoyBootstrapParamsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ServiceKind ServiceKind `protobuf:"varint,1,opt,name=service_kind,json=serviceKind,proto3,enum=hashicorp.consul.dataplane.ServiceKind" json:"service_kind,omitempty"`
	// service is be used to identify the service (as the local cluster name and
	// in metric tags). If the service is a connect proxy it will be the name of
	// the proxy's destination service, for gateways it will be the gateway
	// service's name.
	Service    string           `protobuf:"bytes,2,opt,name=service,proto3" json:"service,omitempty"`
	Namespace  string           `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Partition  string           `protobuf:"bytes,4,opt,name=partition,proto3" json:"partition,omitempty"`
	Datacenter string           `protobuf:"bytes,5,opt,name=datacenter,proto3" json:"datacenter,omitempty"`
	Config     *structpb.Struct `protobuf:"bytes,6,opt,name=config,proto3" json:"config,omitempty"`
	NodeId     string           `protobuf:"bytes,7,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	NodeName   string           `protobuf:"bytes,8,opt,name=node_name,json=nodeName,proto3" json:"node_name,omitempty"`
	AccessLogs []string         `protobuf:"bytes,9,rep,name=access_logs,json=accessLogs,proto3" json:"access_logs,omitempty"`
}

func (x *GetEnvoyBootstrapParamsResponse) Reset() {
	*x = GetEnvoyBootstrapParamsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbdataplane_dataplane_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetEnvoyBootstrapParamsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetEnvoyBootstrapParamsResponse) ProtoMessage() {}

func (x *GetEnvoyBootstrapParamsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pbdataplane_dataplane_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetEnvoyBootstrapParamsResponse.ProtoReflect.Descriptor instead.
func (*GetEnvoyBootstrapParamsResponse) Descriptor() ([]byte, []int) {
	return file_pbdataplane_dataplane_proto_rawDescGZIP(), []int{4}
}

func (x *GetEnvoyBootstrapParamsResponse) GetServiceKind() ServiceKind {
	if x != nil {
		return x.ServiceKind
	}
	return ServiceKind_SERVICE_KIND_UNSPECIFIED
}

func (x *GetEnvoyBootstrapParamsResponse) GetService() string {
	if x != nil {
		return x.Service
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsResponse) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsResponse) GetPartition() string {
	if x != nil {
		return x.Partition
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsResponse) GetDatacenter() string {
	if x != nil {
		return x.Datacenter
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsResponse) GetConfig() *structpb.Struct {
	if x != nil {
		return x.Config
	}
	return nil
}

func (x *GetEnvoyBootstrapParamsResponse) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsResponse) GetNodeName() string {
	if x != nil {
		return x.NodeName
	}
	return ""
}

func (x *GetEnvoyBootstrapParamsResponse) GetAccessLogs() []string {
	if x != nil {
		return x.AccessLogs
	}
	return nil
}

var File_pbdataplane_dataplane_proto protoreflect.FileDescriptor

var file_pbdataplane_dataplane_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x70, 0x62, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2f, 0x64, 0x61,
	0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1a, 0x68,
	0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e,
	0x64, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x1a, 0x25, 0x61, 0x6e, 0x6e, 0x6f, 0x74,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x72, 0x61, 0x74, 0x65, 0x6c, 0x69, 0x6d, 0x69, 0x74,
	0x2f, 0x72, 0x61, 0x74, 0x65, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x26,
	0x0a, 0x24, 0x47, 0x65, 0x74, 0x53, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x44, 0x61,
	0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x89, 0x01, 0x0a, 0x17, 0x44, 0x61, 0x74, 0x61, 0x70,
	0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x53, 0x75, 0x70, 0x70, 0x6f,
	0x72, 0x74, 0x12, 0x50, 0x0a, 0x0c, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x64, 0x61, 0x74, 0x61,
	0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46,
	0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x52, 0x0b, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65,
	0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x65,
	0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x73, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74,
	0x65, 0x64, 0x22, 0x9e, 0x01, 0x0a, 0x25, 0x47, 0x65, 0x74, 0x53, 0x75, 0x70, 0x70, 0x6f, 0x72,
	0x74, 0x65, 0x64, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74,
	0x75, 0x72, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x75, 0x0a, 0x1c,
	0x73, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c,
	0x61, 0x6e, 0x65, 0x5f, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x33, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e,
	0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65,
	0x53, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x1a, 0x73, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74,
	0x65, 0x64, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x73, 0x22, 0xc2, 0x01, 0x0a, 0x1e, 0x47, 0x65, 0x74, 0x45, 0x6e, 0x76, 0x6f, 0x79,
	0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x19, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49,
	0x64, 0x12, 0x1d, 0x0a, 0x09, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x08, 0x6e, 0x6f, 0x64, 0x65, 0x4e, 0x61, 0x6d, 0x65,
	0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12,
	0x1c, 0x0a, 0x09, 0x70, 0x61, 0x72, 0x74, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x70, 0x61, 0x72, 0x74, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1c, 0x0a,
	0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x42, 0x0b, 0x0a, 0x09, 0x6e,
	0x6f, 0x64, 0x65, 0x5f, 0x73, 0x70, 0x65, 0x63, 0x22, 0xeb, 0x02, 0x0a, 0x1f, 0x47, 0x65, 0x74,
	0x45, 0x6e, 0x76, 0x6f, 0x79, 0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x50, 0x61,
	0x72, 0x61, 0x6d, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4a, 0x0a, 0x0c,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x6b, 0x69, 0x6e, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x27, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x4b, 0x69, 0x6e, 0x64, 0x52, 0x0b, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x4b, 0x69, 0x6e, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65,
	0x12, 0x1c, 0x0a, 0x09, 0x70, 0x61, 0x72, 0x74, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x61, 0x72, 0x74, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1e,
	0x0a, 0x0a, 0x64, 0x61, 0x74, 0x61, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0a, 0x64, 0x61, 0x74, 0x61, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x12, 0x2f,
	0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12,
	0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x1b, 0x0a, 0x09, 0x6e, 0x6f, 0x64, 0x65,
	0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6e, 0x6f, 0x64,
	0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x5f,
	0x6c, 0x6f, 0x67, 0x73, 0x18, 0x09, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0a, 0x61, 0x63, 0x63, 0x65,
	0x73, 0x73, 0x4c, 0x6f, 0x67, 0x73, 0x2a, 0xc7, 0x01, 0x0a, 0x11, 0x44, 0x61, 0x74, 0x61, 0x70,
	0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x12, 0x22, 0x0a, 0x1e,
	0x44, 0x41, 0x54, 0x41, 0x50, 0x4c, 0x41, 0x4e, 0x45, 0x5f, 0x46, 0x45, 0x41, 0x54, 0x55, 0x52,
	0x45, 0x53, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00,
	0x12, 0x24, 0x0a, 0x20, 0x44, 0x41, 0x54, 0x41, 0x50, 0x4c, 0x41, 0x4e, 0x45, 0x5f, 0x46, 0x45,
	0x41, 0x54, 0x55, 0x52, 0x45, 0x53, 0x5f, 0x57, 0x41, 0x54, 0x43, 0x48, 0x5f, 0x53, 0x45, 0x52,
	0x56, 0x45, 0x52, 0x53, 0x10, 0x01, 0x12, 0x32, 0x0a, 0x2e, 0x44, 0x41, 0x54, 0x41, 0x50, 0x4c,
	0x41, 0x4e, 0x45, 0x5f, 0x46, 0x45, 0x41, 0x54, 0x55, 0x52, 0x45, 0x53, 0x5f, 0x45, 0x44, 0x47,
	0x45, 0x5f, 0x43, 0x45, 0x52, 0x54, 0x49, 0x46, 0x49, 0x43, 0x41, 0x54, 0x45, 0x5f, 0x4d, 0x41,
	0x4e, 0x41, 0x47, 0x45, 0x4d, 0x45, 0x4e, 0x54, 0x10, 0x02, 0x12, 0x34, 0x0a, 0x30, 0x44, 0x41,
	0x54, 0x41, 0x50, 0x4c, 0x41, 0x4e, 0x45, 0x5f, 0x46, 0x45, 0x41, 0x54, 0x55, 0x52, 0x45, 0x53,
	0x5f, 0x45, 0x4e, 0x56, 0x4f, 0x59, 0x5f, 0x42, 0x4f, 0x4f, 0x54, 0x53, 0x54, 0x52, 0x41, 0x50,
	0x5f, 0x43, 0x4f, 0x4e, 0x46, 0x49, 0x47, 0x55, 0x52, 0x41, 0x54, 0x49, 0x4f, 0x4e, 0x10, 0x03,
	0x2a, 0xea, 0x01, 0x0a, 0x0b, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x4b, 0x69, 0x6e, 0x64,
	0x12, 0x1c, 0x0a, 0x18, 0x53, 0x45, 0x52, 0x56, 0x49, 0x43, 0x45, 0x5f, 0x4b, 0x49, 0x4e, 0x44,
	0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x18,
	0x0a, 0x14, 0x53, 0x45, 0x52, 0x56, 0x49, 0x43, 0x45, 0x5f, 0x4b, 0x49, 0x4e, 0x44, 0x5f, 0x54,
	0x59, 0x50, 0x49, 0x43, 0x41, 0x4c, 0x10, 0x01, 0x12, 0x1e, 0x0a, 0x1a, 0x53, 0x45, 0x52, 0x56,
	0x49, 0x43, 0x45, 0x5f, 0x4b, 0x49, 0x4e, 0x44, 0x5f, 0x43, 0x4f, 0x4e, 0x4e, 0x45, 0x43, 0x54,
	0x5f, 0x50, 0x52, 0x4f, 0x58, 0x59, 0x10, 0x02, 0x12, 0x1d, 0x0a, 0x19, 0x53, 0x45, 0x52, 0x56,
	0x49, 0x43, 0x45, 0x5f, 0x4b, 0x49, 0x4e, 0x44, 0x5f, 0x4d, 0x45, 0x53, 0x48, 0x5f, 0x47, 0x41,
	0x54, 0x45, 0x57, 0x41, 0x59, 0x10, 0x03, 0x12, 0x24, 0x0a, 0x20, 0x53, 0x45, 0x52, 0x56, 0x49,
	0x43, 0x45, 0x5f, 0x4b, 0x49, 0x4e, 0x44, 0x5f, 0x54, 0x45, 0x52, 0x4d, 0x49, 0x4e, 0x41, 0x54,
	0x49, 0x4e, 0x47, 0x5f, 0x47, 0x41, 0x54, 0x45, 0x57, 0x41, 0x59, 0x10, 0x04, 0x12, 0x20, 0x0a,
	0x1c, 0x53, 0x45, 0x52, 0x56, 0x49, 0x43, 0x45, 0x5f, 0x4b, 0x49, 0x4e, 0x44, 0x5f, 0x49, 0x4e,
	0x47, 0x52, 0x45, 0x53, 0x53, 0x5f, 0x47, 0x41, 0x54, 0x45, 0x57, 0x41, 0x59, 0x10, 0x05, 0x12,
	0x1c, 0x0a, 0x18, 0x53, 0x45, 0x52, 0x56, 0x49, 0x43, 0x45, 0x5f, 0x4b, 0x49, 0x4e, 0x44, 0x5f,
	0x41, 0x50, 0x49, 0x5f, 0x47, 0x41, 0x54, 0x45, 0x57, 0x41, 0x59, 0x10, 0x06, 0x32, 0xe2, 0x02,
	0x0a, 0x10, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0xae, 0x01, 0x0a, 0x1d, 0x47, 0x65, 0x74, 0x53, 0x75, 0x70, 0x70, 0x6f, 0x72,
	0x74, 0x65, 0x64, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74,
	0x75, 0x72, 0x65, 0x73, 0x12, 0x40, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e,
	0x65, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x44, 0x61,
	0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x41, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c,
	0x61, 0x6e, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x53, 0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64,
	0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65,
	0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x08, 0xe2, 0x86, 0x04, 0x04, 0x08,
	0x02, 0x10, 0x07, 0x12, 0x9c, 0x01, 0x0a, 0x17, 0x47, 0x65, 0x74, 0x45, 0x6e, 0x76, 0x6f, 0x79,
	0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x12,
	0x3a, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x2e, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x47, 0x65, 0x74,
	0x45, 0x6e, 0x76, 0x6f, 0x79, 0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x50, 0x61,
	0x72, 0x61, 0x6d, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x3b, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x64,
	0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x45, 0x6e, 0x76, 0x6f,
	0x79, 0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x08, 0xe2, 0x86, 0x04, 0x04, 0x08, 0x02,
	0x10, 0x07, 0x42, 0xf0, 0x01, 0x0a, 0x1e, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x64, 0x61, 0x74, 0x61,
	0x70, 0x6c, 0x61, 0x6e, 0x65, 0x42, 0x0e, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x34, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69,
	0x63, 0x2f, 0x70, 0x62, 0x64, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0xa2, 0x02, 0x03,
	0x48, 0x43, 0x44, 0xaa, 0x02, 0x1a, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e,
	0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65,
	0xca, 0x02, 0x1a, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x5c, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0xe2, 0x02, 0x26,
	0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x5c, 0x44, 0x61, 0x74, 0x61, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65,
	0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x1c, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x44, 0x61, 0x74, 0x61,
	0x70, 0x6c, 0x61, 0x6e, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbdataplane_dataplane_proto_rawDescOnce sync.Once
	file_pbdataplane_dataplane_proto_rawDescData = file_pbdataplane_dataplane_proto_rawDesc
)

func file_pbdataplane_dataplane_proto_rawDescGZIP() []byte {
	file_pbdataplane_dataplane_proto_rawDescOnce.Do(func() {
		file_pbdataplane_dataplane_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbdataplane_dataplane_proto_rawDescData)
	})
	return file_pbdataplane_dataplane_proto_rawDescData
}

var file_pbdataplane_dataplane_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_pbdataplane_dataplane_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_pbdataplane_dataplane_proto_goTypes = []interface{}{
	(DataplaneFeatures)(0),                        // 0: hashicorp.consul.dataplane.DataplaneFeatures
	(ServiceKind)(0),                              // 1: hashicorp.consul.dataplane.ServiceKind
	(*GetSupportedDataplaneFeaturesRequest)(nil),  // 2: hashicorp.consul.dataplane.GetSupportedDataplaneFeaturesRequest
	(*DataplaneFeatureSupport)(nil),               // 3: hashicorp.consul.dataplane.DataplaneFeatureSupport
	(*GetSupportedDataplaneFeaturesResponse)(nil), // 4: hashicorp.consul.dataplane.GetSupportedDataplaneFeaturesResponse
	(*GetEnvoyBootstrapParamsRequest)(nil),        // 5: hashicorp.consul.dataplane.GetEnvoyBootstrapParamsRequest
	(*GetEnvoyBootstrapParamsResponse)(nil),       // 6: hashicorp.consul.dataplane.GetEnvoyBootstrapParamsResponse
	(*structpb.Struct)(nil),                       // 7: google.protobuf.Struct
}
var file_pbdataplane_dataplane_proto_depIdxs = []int32{
	0, // 0: hashicorp.consul.dataplane.DataplaneFeatureSupport.feature_name:type_name -> hashicorp.consul.dataplane.DataplaneFeatures
	3, // 1: hashicorp.consul.dataplane.GetSupportedDataplaneFeaturesResponse.supported_dataplane_features:type_name -> hashicorp.consul.dataplane.DataplaneFeatureSupport
	1, // 2: hashicorp.consul.dataplane.GetEnvoyBootstrapParamsResponse.service_kind:type_name -> hashicorp.consul.dataplane.ServiceKind
	7, // 3: hashicorp.consul.dataplane.GetEnvoyBootstrapParamsResponse.config:type_name -> google.protobuf.Struct
	2, // 4: hashicorp.consul.dataplane.DataplaneService.GetSupportedDataplaneFeatures:input_type -> hashicorp.consul.dataplane.GetSupportedDataplaneFeaturesRequest
	5, // 5: hashicorp.consul.dataplane.DataplaneService.GetEnvoyBootstrapParams:input_type -> hashicorp.consul.dataplane.GetEnvoyBootstrapParamsRequest
	4, // 6: hashicorp.consul.dataplane.DataplaneService.GetSupportedDataplaneFeatures:output_type -> hashicorp.consul.dataplane.GetSupportedDataplaneFeaturesResponse
	6, // 7: hashicorp.consul.dataplane.DataplaneService.GetEnvoyBootstrapParams:output_type -> hashicorp.consul.dataplane.GetEnvoyBootstrapParamsResponse
	6, // [6:8] is the sub-list for method output_type
	4, // [4:6] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_pbdataplane_dataplane_proto_init() }
func file_pbdataplane_dataplane_proto_init() {
	if File_pbdataplane_dataplane_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbdataplane_dataplane_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetSupportedDataplaneFeaturesRequest); i {
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
		file_pbdataplane_dataplane_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataplaneFeatureSupport); i {
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
		file_pbdataplane_dataplane_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetSupportedDataplaneFeaturesResponse); i {
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
		file_pbdataplane_dataplane_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetEnvoyBootstrapParamsRequest); i {
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
		file_pbdataplane_dataplane_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetEnvoyBootstrapParamsResponse); i {
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
	file_pbdataplane_dataplane_proto_msgTypes[3].OneofWrappers = []interface{}{
		(*GetEnvoyBootstrapParamsRequest_NodeId)(nil),
		(*GetEnvoyBootstrapParamsRequest_NodeName)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbdataplane_dataplane_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pbdataplane_dataplane_proto_goTypes,
		DependencyIndexes: file_pbdataplane_dataplane_proto_depIdxs,
		EnumInfos:         file_pbdataplane_dataplane_proto_enumTypes,
		MessageInfos:      file_pbdataplane_dataplane_proto_msgTypes,
	}.Build()
	File_pbdataplane_dataplane_proto = out.File
	file_pbdataplane_dataplane_proto_rawDesc = nil
	file_pbdataplane_dataplane_proto_goTypes = nil
	file_pbdataplane_dataplane_proto_depIdxs = nil
}

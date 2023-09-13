// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v2beta1/grpc_route.proto

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

type GRPCMethodMatchType int32

const (
	GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_UNSPECIFIED GRPCMethodMatchType = 0
	GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT       GRPCMethodMatchType = 1
	GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_REGEX       GRPCMethodMatchType = 2
)

// Enum value maps for GRPCMethodMatchType.
var (
	GRPCMethodMatchType_name = map[int32]string{
		0: "GRPC_METHOD_MATCH_TYPE_UNSPECIFIED",
		1: "GRPC_METHOD_MATCH_TYPE_EXACT",
		2: "GRPC_METHOD_MATCH_TYPE_REGEX",
	}
	GRPCMethodMatchType_value = map[string]int32{
		"GRPC_METHOD_MATCH_TYPE_UNSPECIFIED": 0,
		"GRPC_METHOD_MATCH_TYPE_EXACT":       1,
		"GRPC_METHOD_MATCH_TYPE_REGEX":       2,
	}
)

func (x GRPCMethodMatchType) Enum() *GRPCMethodMatchType {
	p := new(GRPCMethodMatchType)
	*p = x
	return p
}

func (x GRPCMethodMatchType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (GRPCMethodMatchType) Descriptor() protoreflect.EnumDescriptor {
	return file_pbmesh_v2beta1_grpc_route_proto_enumTypes[0].Descriptor()
}

func (GRPCMethodMatchType) Type() protoreflect.EnumType {
	return &file_pbmesh_v2beta1_grpc_route_proto_enumTypes[0]
}

func (x GRPCMethodMatchType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use GRPCMethodMatchType.Descriptor instead.
func (GRPCMethodMatchType) EnumDescriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{0}
}

// NOTE: this should align to the GAMMA/gateway-api version, or at least be
// easily translatable.
//
// https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1alpha2.GRPCRoute
//
// This is a Resource type.
type GRPCRoute struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ParentRefs references the resources (usually Gateways) that a Route wants
	// to be attached to. Note that the referenced parent resource needs to allow
	// this for the attachment to be complete. For Gateways, that means the
	// Gateway needs to allow attachment from Routes of this kind and namespace.
	//
	// It is invalid to reference an identical parent more than once. It is valid
	// to reference multiple distinct sections within the same parent resource,
	// such as 2 Listeners within a Gateway.
	ParentRefs []*ParentReference `protobuf:"bytes,1,rep,name=parent_refs,json=parentRefs,proto3" json:"parent_refs,omitempty"`
	Hostnames  []string           `protobuf:"bytes,2,rep,name=hostnames,proto3" json:"hostnames,omitempty"`
	// Rules are a list of GRPC matchers, filters and actions.
	Rules []*GRPCRouteRule `protobuf:"bytes,3,rep,name=rules,proto3" json:"rules,omitempty"`
}

func (x *GRPCRoute) Reset() {
	*x = GRPCRoute{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GRPCRoute) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GRPCRoute) ProtoMessage() {}

func (x *GRPCRoute) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GRPCRoute.ProtoReflect.Descriptor instead.
func (*GRPCRoute) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{0}
}

func (x *GRPCRoute) GetParentRefs() []*ParentReference {
	if x != nil {
		return x.ParentRefs
	}
	return nil
}

func (x *GRPCRoute) GetHostnames() []string {
	if x != nil {
		return x.Hostnames
	}
	return nil
}

func (x *GRPCRoute) GetRules() []*GRPCRouteRule {
	if x != nil {
		return x.Rules
	}
	return nil
}

type GRPCRouteRule struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Matches     []*GRPCRouteMatch  `protobuf:"bytes,1,rep,name=matches,proto3" json:"matches,omitempty"`
	Filters     []*GRPCRouteFilter `protobuf:"bytes,2,rep,name=filters,proto3" json:"filters,omitempty"`
	BackendRefs []*GRPCBackendRef  `protobuf:"bytes,3,rep,name=backend_refs,json=backendRefs,proto3" json:"backend_refs,omitempty"`
	// ALTERNATIVE: Timeouts defines the timeouts that can be configured for an HTTP request.
	Timeouts *HTTPRouteTimeouts `protobuf:"bytes,4,opt,name=timeouts,proto3" json:"timeouts,omitempty"`
	// ALTERNATIVE:
	Retries *HTTPRouteRetries `protobuf:"bytes,5,opt,name=retries,proto3" json:"retries,omitempty"`
}

func (x *GRPCRouteRule) Reset() {
	*x = GRPCRouteRule{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GRPCRouteRule) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GRPCRouteRule) ProtoMessage() {}

func (x *GRPCRouteRule) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GRPCRouteRule.ProtoReflect.Descriptor instead.
func (*GRPCRouteRule) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{1}
}

func (x *GRPCRouteRule) GetMatches() []*GRPCRouteMatch {
	if x != nil {
		return x.Matches
	}
	return nil
}

func (x *GRPCRouteRule) GetFilters() []*GRPCRouteFilter {
	if x != nil {
		return x.Filters
	}
	return nil
}

func (x *GRPCRouteRule) GetBackendRefs() []*GRPCBackendRef {
	if x != nil {
		return x.BackendRefs
	}
	return nil
}

func (x *GRPCRouteRule) GetTimeouts() *HTTPRouteTimeouts {
	if x != nil {
		return x.Timeouts
	}
	return nil
}

func (x *GRPCRouteRule) GetRetries() *HTTPRouteRetries {
	if x != nil {
		return x.Retries
	}
	return nil
}

type GRPCRouteMatch struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Method specifies a gRPC request service/method matcher. If this field is
	// not specified, all services and methods will match.
	Method *GRPCMethodMatch `protobuf:"bytes,1,opt,name=method,proto3" json:"method,omitempty"`
	// Headers specifies gRPC request header matchers. Multiple match values are
	// ANDed together, meaning, a request MUST match all the specified headers to
	// select the route.
	Headers []*GRPCHeaderMatch `protobuf:"bytes,2,rep,name=headers,proto3" json:"headers,omitempty"`
}

func (x *GRPCRouteMatch) Reset() {
	*x = GRPCRouteMatch{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GRPCRouteMatch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GRPCRouteMatch) ProtoMessage() {}

func (x *GRPCRouteMatch) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GRPCRouteMatch.ProtoReflect.Descriptor instead.
func (*GRPCRouteMatch) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{2}
}

func (x *GRPCRouteMatch) GetMethod() *GRPCMethodMatch {
	if x != nil {
		return x.Method
	}
	return nil
}

func (x *GRPCRouteMatch) GetHeaders() []*GRPCHeaderMatch {
	if x != nil {
		return x.Headers
	}
	return nil
}

type GRPCMethodMatch struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Type specifies how to match against the service and/or method. Support:
	// Core (Exact with service and method specified)
	Type GRPCMethodMatchType `protobuf:"varint,1,opt,name=type,proto3,enum=hashicorp.consul.mesh.v2beta1.GRPCMethodMatchType" json:"type,omitempty"`
	// Value of the service to match against. If left empty or omitted, will
	// match any service.
	//
	// At least one of Service and Method MUST be a non-empty string.
	Service string `protobuf:"bytes,2,opt,name=service,proto3" json:"service,omitempty"`
	// Value of the method to match against. If left empty or omitted, will match
	// all services.
	//
	// At least one of Service and Method MUST be a non-empty string.}
	Method string `protobuf:"bytes,3,opt,name=method,proto3" json:"method,omitempty"`
}

func (x *GRPCMethodMatch) Reset() {
	*x = GRPCMethodMatch{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GRPCMethodMatch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GRPCMethodMatch) ProtoMessage() {}

func (x *GRPCMethodMatch) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GRPCMethodMatch.ProtoReflect.Descriptor instead.
func (*GRPCMethodMatch) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{3}
}

func (x *GRPCMethodMatch) GetType() GRPCMethodMatchType {
	if x != nil {
		return x.Type
	}
	return GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_UNSPECIFIED
}

func (x *GRPCMethodMatch) GetService() string {
	if x != nil {
		return x.Service
	}
	return ""
}

func (x *GRPCMethodMatch) GetMethod() string {
	if x != nil {
		return x.Method
	}
	return ""
}

type GRPCHeaderMatch struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type  HeaderMatchType `protobuf:"varint,1,opt,name=type,proto3,enum=hashicorp.consul.mesh.v2beta1.HeaderMatchType" json:"type,omitempty"`
	Name  string          `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Value string          `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *GRPCHeaderMatch) Reset() {
	*x = GRPCHeaderMatch{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GRPCHeaderMatch) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GRPCHeaderMatch) ProtoMessage() {}

func (x *GRPCHeaderMatch) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GRPCHeaderMatch.ProtoReflect.Descriptor instead.
func (*GRPCHeaderMatch) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{4}
}

func (x *GRPCHeaderMatch) GetType() HeaderMatchType {
	if x != nil {
		return x.Type
	}
	return HeaderMatchType_HEADER_MATCH_TYPE_UNSPECIFIED
}

func (x *GRPCHeaderMatch) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *GRPCHeaderMatch) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

type GRPCRouteFilter struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// RequestHeaderModifier defines a schema for a filter that modifies request
	// headers.
	RequestHeaderModifier *HTTPHeaderFilter `protobuf:"bytes,1,opt,name=request_header_modifier,json=requestHeaderModifier,proto3" json:"request_header_modifier,omitempty"`
	// ResponseHeaderModifier defines a schema for a filter that modifies
	// response headers.
	ResponseHeaderModifier *HTTPHeaderFilter `protobuf:"bytes,2,opt,name=response_header_modifier,json=responseHeaderModifier,proto3" json:"response_header_modifier,omitempty"`
	// URLRewrite defines a schema for a filter that modifies a request during
	// forwarding.
	UrlRewrite *HTTPURLRewriteFilter `protobuf:"bytes,5,opt,name=url_rewrite,json=urlRewrite,proto3" json:"url_rewrite,omitempty"`
}

func (x *GRPCRouteFilter) Reset() {
	*x = GRPCRouteFilter{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GRPCRouteFilter) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GRPCRouteFilter) ProtoMessage() {}

func (x *GRPCRouteFilter) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GRPCRouteFilter.ProtoReflect.Descriptor instead.
func (*GRPCRouteFilter) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{5}
}

func (x *GRPCRouteFilter) GetRequestHeaderModifier() *HTTPHeaderFilter {
	if x != nil {
		return x.RequestHeaderModifier
	}
	return nil
}

func (x *GRPCRouteFilter) GetResponseHeaderModifier() *HTTPHeaderFilter {
	if x != nil {
		return x.ResponseHeaderModifier
	}
	return nil
}

func (x *GRPCRouteFilter) GetUrlRewrite() *HTTPURLRewriteFilter {
	if x != nil {
		return x.UrlRewrite
	}
	return nil
}

type GRPCBackendRef struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BackendRef *BackendReference `protobuf:"bytes,1,opt,name=backend_ref,json=backendRef,proto3" json:"backend_ref,omitempty"`
	// Weight specifies the proportion of requests forwarded to the referenced
	// backend. This is computed as weight/(sum of all weights in this
	// BackendRefs list). For non-zero values, there may be some epsilon from the
	// exact proportion defined here depending on the precision an implementation
	// supports. Weight is not a percentage and the sum of weights does not need
	// to equal 100.
	//
	// If only one backend is specified and it has a weight greater than 0, 100%
	// of the traffic is forwarded to that backend. If weight is set to 0, no
	// traffic should be forwarded for this entry. If unspecified, weight defaults
	// to 1.
	Weight uint32 `protobuf:"varint,2,opt,name=weight,proto3" json:"weight,omitempty"`
	// Filters defined at this level should be executed if and only if the
	// request is being forwarded to the backend defined here.
	Filters []*GRPCRouteFilter `protobuf:"bytes,3,rep,name=filters,proto3" json:"filters,omitempty"`
}

func (x *GRPCBackendRef) Reset() {
	*x = GRPCBackendRef{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GRPCBackendRef) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GRPCBackendRef) ProtoMessage() {}

func (x *GRPCBackendRef) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_grpc_route_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GRPCBackendRef.ProtoReflect.Descriptor instead.
func (*GRPCBackendRef) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP(), []int{6}
}

func (x *GRPCBackendRef) GetBackendRef() *BackendReference {
	if x != nil {
		return x.BackendRef
	}
	return nil
}

func (x *GRPCBackendRef) GetWeight() uint32 {
	if x != nil {
		return x.Weight
	}
	return 0
}

func (x *GRPCBackendRef) GetFilters() []*GRPCRouteFilter {
	if x != nil {
		return x.Filters
	}
	return nil
}

var File_pbmesh_v2beta1_grpc_route_proto protoreflect.FileDescriptor

var file_pbmesh_v2beta1_grpc_route_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x67, 0x72, 0x70, 0x63, 0x5f, 0x72, 0x6f, 0x75, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x1d, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x1a, 0x1b, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x70,
	0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x68, 0x74,
	0x74, 0x70, 0x5f, 0x72, 0x6f, 0x75, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x27,
	0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x68,
	0x74, 0x74, 0x70, 0x5f, 0x72, 0x6f, 0x75, 0x74, 0x65, 0x5f, 0x72, 0x65, 0x74, 0x72, 0x69, 0x65,
	0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x28, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f,
	0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x68, 0x74, 0x74, 0x70, 0x5f, 0x72, 0x6f, 0x75,
	0x74, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x1c, 0x70, 0x62, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x61, 0x6e,
	0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0xc6, 0x01, 0x0a, 0x09, 0x47, 0x52, 0x50, 0x43, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x12, 0x4f, 0x0a,
	0x0b, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x72, 0x65, 0x66, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2e, 0x50, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e,
	0x63, 0x65, 0x52, 0x0a, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x66, 0x73, 0x12, 0x1c,
	0x0a, 0x09, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x09, 0x68, 0x6f, 0x73, 0x74, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x12, 0x42, 0x0a, 0x05,
	0x72, 0x75, 0x6c, 0x65, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x52, 0x50, 0x43,
	0x52, 0x6f, 0x75, 0x74, 0x65, 0x52, 0x75, 0x6c, 0x65, 0x52, 0x05, 0x72, 0x75, 0x6c, 0x65, 0x73,
	0x3a, 0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x03, 0x22, 0x8d, 0x03, 0x0a, 0x0d, 0x47, 0x52, 0x50,
	0x43, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x52, 0x75, 0x6c, 0x65, 0x12, 0x47, 0x0a, 0x07, 0x6d, 0x61,
	0x74, 0x63, 0x68, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x52, 0x50, 0x43,
	0x52, 0x6f, 0x75, 0x74, 0x65, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x52, 0x07, 0x6d, 0x61, 0x74, 0x63,
	0x68, 0x65, 0x73, 0x12, 0x48, 0x0a, 0x07, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x73, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x52, 0x50, 0x43, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x46, 0x69,
	0x6c, 0x74, 0x65, 0x72, 0x52, 0x07, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x73, 0x12, 0x50, 0x0a,
	0x0c, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x5f, 0x72, 0x65, 0x66, 0x73, 0x18, 0x03, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e,
	0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x2e, 0x47, 0x52, 0x50, 0x43, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52,
	0x65, 0x66, 0x52, 0x0b, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x66, 0x73, 0x12,
	0x4c, 0x0a, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x30, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x48, 0x54, 0x54, 0x50, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x6f,
	0x75, 0x74, 0x73, 0x52, 0x08, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x73, 0x12, 0x49, 0x0a,
	0x07, 0x72, 0x65, 0x74, 0x72, 0x69, 0x65, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2f,
	0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x48,
	0x54, 0x54, 0x50, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x52, 0x65, 0x74, 0x72, 0x69, 0x65, 0x73, 0x52,
	0x07, 0x72, 0x65, 0x74, 0x72, 0x69, 0x65, 0x73, 0x22, 0xa2, 0x01, 0x0a, 0x0e, 0x47, 0x52, 0x50,
	0x43, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x12, 0x46, 0x0a, 0x06, 0x6d,
	0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x52, 0x50, 0x43,
	0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x52, 0x06, 0x6d, 0x65, 0x74,
	0x68, 0x6f, 0x64, 0x12, 0x48, 0x0a, 0x07, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x52, 0x50, 0x43, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d,
	0x61, 0x74, 0x63, 0x68, 0x52, 0x07, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73, 0x22, 0x8b, 0x01,
	0x0a, 0x0f, 0x47, 0x52, 0x50, 0x43, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4d, 0x61, 0x74, 0x63,
	0x68, 0x12, 0x46, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x32, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e,
	0x47, 0x52, 0x50, 0x43, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x22, 0x7f, 0x0a, 0x0f, 0x47,
	0x52, 0x50, 0x43, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x12, 0x42,
	0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2e, 0x2e, 0x68,
	0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e,
	0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x48, 0x65, 0x61,
	0x64, 0x65, 0x72, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0xbb, 0x02, 0x0a,
	0x0f, 0x47, 0x52, 0x50, 0x43, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72,
	0x12, 0x67, 0x0a, 0x17, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x68, 0x65, 0x61, 0x64,
	0x65, 0x72, 0x5f, 0x6d, 0x6f, 0x64, 0x69, 0x66, 0x69, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x2f, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x48, 0x54, 0x54, 0x50, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x46, 0x69, 0x6c, 0x74,
	0x65, 0x72, 0x52, 0x15, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65,
	0x72, 0x4d, 0x6f, 0x64, 0x69, 0x66, 0x69, 0x65, 0x72, 0x12, 0x69, 0x0a, 0x18, 0x72, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x5f, 0x6d, 0x6f, 0x64,
	0x69, 0x66, 0x69, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2f, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x48, 0x54, 0x54, 0x50,
	0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x52, 0x16, 0x72, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x6f, 0x64, 0x69,
	0x66, 0x69, 0x65, 0x72, 0x12, 0x54, 0x0a, 0x0b, 0x75, 0x72, 0x6c, 0x5f, 0x72, 0x65, 0x77, 0x72,
	0x69, 0x74, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73,
	0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x48, 0x54, 0x54, 0x50, 0x55, 0x52,
	0x4c, 0x52, 0x65, 0x77, 0x72, 0x69, 0x74, 0x65, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x52, 0x0a,
	0x75, 0x72, 0x6c, 0x52, 0x65, 0x77, 0x72, 0x69, 0x74, 0x65, 0x22, 0xc4, 0x01, 0x0a, 0x0e, 0x47,
	0x52, 0x50, 0x43, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x66, 0x12, 0x50, 0x0a,
	0x0b, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x5f, 0x72, 0x65, 0x66, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x2f, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65,
	0x6e, 0x63, 0x65, 0x52, 0x0a, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x66, 0x12,
	0x16, 0x0a, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x48, 0x0a, 0x07, 0x66, 0x69, 0x6c, 0x74, 0x65,
	0x72, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68,
	0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x52, 0x50, 0x43, 0x52, 0x6f, 0x75,
	0x74, 0x65, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x52, 0x07, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72,
	0x73, 0x2a, 0x81, 0x01, 0x0a, 0x13, 0x47, 0x52, 0x50, 0x43, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64,
	0x4d, 0x61, 0x74, 0x63, 0x68, 0x54, 0x79, 0x70, 0x65, 0x12, 0x26, 0x0a, 0x22, 0x47, 0x52, 0x50,
	0x43, 0x5f, 0x4d, 0x45, 0x54, 0x48, 0x4f, 0x44, 0x5f, 0x4d, 0x41, 0x54, 0x43, 0x48, 0x5f, 0x54,
	0x59, 0x50, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10,
	0x00, 0x12, 0x20, 0x0a, 0x1c, 0x47, 0x52, 0x50, 0x43, 0x5f, 0x4d, 0x45, 0x54, 0x48, 0x4f, 0x44,
	0x5f, 0x4d, 0x41, 0x54, 0x43, 0x48, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x45, 0x58, 0x41, 0x43,
	0x54, 0x10, 0x01, 0x12, 0x20, 0x0a, 0x1c, 0x47, 0x52, 0x50, 0x43, 0x5f, 0x4d, 0x45, 0x54, 0x48,
	0x4f, 0x44, 0x5f, 0x4d, 0x41, 0x54, 0x43, 0x48, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x52, 0x45,
	0x47, 0x45, 0x58, 0x10, 0x02, 0x42, 0x8f, 0x02, 0x0a, 0x21, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x42, 0x0e, 0x47, 0x72, 0x70,
	0x63, 0x52, 0x6f, 0x75, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x43, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76,
	0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x3b, 0x6d, 0x65, 0x73, 0x68, 0x76, 0x32, 0x62, 0x65, 0x74,
	0x61, 0x31, 0xa2, 0x02, 0x03, 0x48, 0x43, 0x4d, 0xaa, 0x02, 0x1d, 0x48, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73, 0x68,
	0x2e, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xca, 0x02, 0x1d, 0x48, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68,
	0x5c, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0xe2, 0x02, 0x29, 0x48, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68,
	0x5c, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61,
	0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x20, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x65, 0x73, 0x68, 0x3a, 0x3a,
	0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v2beta1_grpc_route_proto_rawDescOnce sync.Once
	file_pbmesh_v2beta1_grpc_route_proto_rawDescData = file_pbmesh_v2beta1_grpc_route_proto_rawDesc
)

func file_pbmesh_v2beta1_grpc_route_proto_rawDescGZIP() []byte {
	file_pbmesh_v2beta1_grpc_route_proto_rawDescOnce.Do(func() {
		file_pbmesh_v2beta1_grpc_route_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v2beta1_grpc_route_proto_rawDescData)
	})
	return file_pbmesh_v2beta1_grpc_route_proto_rawDescData
}

var file_pbmesh_v2beta1_grpc_route_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pbmesh_v2beta1_grpc_route_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_pbmesh_v2beta1_grpc_route_proto_goTypes = []interface{}{
	(GRPCMethodMatchType)(0),     // 0: hashicorp.consul.mesh.v2beta1.GRPCMethodMatchType
	(*GRPCRoute)(nil),            // 1: hashicorp.consul.mesh.v2beta1.GRPCRoute
	(*GRPCRouteRule)(nil),        // 2: hashicorp.consul.mesh.v2beta1.GRPCRouteRule
	(*GRPCRouteMatch)(nil),       // 3: hashicorp.consul.mesh.v2beta1.GRPCRouteMatch
	(*GRPCMethodMatch)(nil),      // 4: hashicorp.consul.mesh.v2beta1.GRPCMethodMatch
	(*GRPCHeaderMatch)(nil),      // 5: hashicorp.consul.mesh.v2beta1.GRPCHeaderMatch
	(*GRPCRouteFilter)(nil),      // 6: hashicorp.consul.mesh.v2beta1.GRPCRouteFilter
	(*GRPCBackendRef)(nil),       // 7: hashicorp.consul.mesh.v2beta1.GRPCBackendRef
	(*ParentReference)(nil),      // 8: hashicorp.consul.mesh.v2beta1.ParentReference
	(*HTTPRouteTimeouts)(nil),    // 9: hashicorp.consul.mesh.v2beta1.HTTPRouteTimeouts
	(*HTTPRouteRetries)(nil),     // 10: hashicorp.consul.mesh.v2beta1.HTTPRouteRetries
	(HeaderMatchType)(0),         // 11: hashicorp.consul.mesh.v2beta1.HeaderMatchType
	(*HTTPHeaderFilter)(nil),     // 12: hashicorp.consul.mesh.v2beta1.HTTPHeaderFilter
	(*HTTPURLRewriteFilter)(nil), // 13: hashicorp.consul.mesh.v2beta1.HTTPURLRewriteFilter
	(*BackendReference)(nil),     // 14: hashicorp.consul.mesh.v2beta1.BackendReference
}
var file_pbmesh_v2beta1_grpc_route_proto_depIdxs = []int32{
	8,  // 0: hashicorp.consul.mesh.v2beta1.GRPCRoute.parent_refs:type_name -> hashicorp.consul.mesh.v2beta1.ParentReference
	2,  // 1: hashicorp.consul.mesh.v2beta1.GRPCRoute.rules:type_name -> hashicorp.consul.mesh.v2beta1.GRPCRouteRule
	3,  // 2: hashicorp.consul.mesh.v2beta1.GRPCRouteRule.matches:type_name -> hashicorp.consul.mesh.v2beta1.GRPCRouteMatch
	6,  // 3: hashicorp.consul.mesh.v2beta1.GRPCRouteRule.filters:type_name -> hashicorp.consul.mesh.v2beta1.GRPCRouteFilter
	7,  // 4: hashicorp.consul.mesh.v2beta1.GRPCRouteRule.backend_refs:type_name -> hashicorp.consul.mesh.v2beta1.GRPCBackendRef
	9,  // 5: hashicorp.consul.mesh.v2beta1.GRPCRouteRule.timeouts:type_name -> hashicorp.consul.mesh.v2beta1.HTTPRouteTimeouts
	10, // 6: hashicorp.consul.mesh.v2beta1.GRPCRouteRule.retries:type_name -> hashicorp.consul.mesh.v2beta1.HTTPRouteRetries
	4,  // 7: hashicorp.consul.mesh.v2beta1.GRPCRouteMatch.method:type_name -> hashicorp.consul.mesh.v2beta1.GRPCMethodMatch
	5,  // 8: hashicorp.consul.mesh.v2beta1.GRPCRouteMatch.headers:type_name -> hashicorp.consul.mesh.v2beta1.GRPCHeaderMatch
	0,  // 9: hashicorp.consul.mesh.v2beta1.GRPCMethodMatch.type:type_name -> hashicorp.consul.mesh.v2beta1.GRPCMethodMatchType
	11, // 10: hashicorp.consul.mesh.v2beta1.GRPCHeaderMatch.type:type_name -> hashicorp.consul.mesh.v2beta1.HeaderMatchType
	12, // 11: hashicorp.consul.mesh.v2beta1.GRPCRouteFilter.request_header_modifier:type_name -> hashicorp.consul.mesh.v2beta1.HTTPHeaderFilter
	12, // 12: hashicorp.consul.mesh.v2beta1.GRPCRouteFilter.response_header_modifier:type_name -> hashicorp.consul.mesh.v2beta1.HTTPHeaderFilter
	13, // 13: hashicorp.consul.mesh.v2beta1.GRPCRouteFilter.url_rewrite:type_name -> hashicorp.consul.mesh.v2beta1.HTTPURLRewriteFilter
	14, // 14: hashicorp.consul.mesh.v2beta1.GRPCBackendRef.backend_ref:type_name -> hashicorp.consul.mesh.v2beta1.BackendReference
	6,  // 15: hashicorp.consul.mesh.v2beta1.GRPCBackendRef.filters:type_name -> hashicorp.consul.mesh.v2beta1.GRPCRouteFilter
	16, // [16:16] is the sub-list for method output_type
	16, // [16:16] is the sub-list for method input_type
	16, // [16:16] is the sub-list for extension type_name
	16, // [16:16] is the sub-list for extension extendee
	0,  // [0:16] is the sub-list for field type_name
}

func init() { file_pbmesh_v2beta1_grpc_route_proto_init() }
func file_pbmesh_v2beta1_grpc_route_proto_init() {
	if File_pbmesh_v2beta1_grpc_route_proto != nil {
		return
	}
	file_pbmesh_v2beta1_common_proto_init()
	file_pbmesh_v2beta1_http_route_proto_init()
	file_pbmesh_v2beta1_http_route_retries_proto_init()
	file_pbmesh_v2beta1_http_route_timeouts_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v2beta1_grpc_route_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GRPCRoute); i {
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
		file_pbmesh_v2beta1_grpc_route_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GRPCRouteRule); i {
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
		file_pbmesh_v2beta1_grpc_route_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GRPCRouteMatch); i {
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
		file_pbmesh_v2beta1_grpc_route_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GRPCMethodMatch); i {
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
		file_pbmesh_v2beta1_grpc_route_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GRPCHeaderMatch); i {
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
		file_pbmesh_v2beta1_grpc_route_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GRPCRouteFilter); i {
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
		file_pbmesh_v2beta1_grpc_route_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GRPCBackendRef); i {
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
			RawDescriptor: file_pbmesh_v2beta1_grpc_route_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v2beta1_grpc_route_proto_goTypes,
		DependencyIndexes: file_pbmesh_v2beta1_grpc_route_proto_depIdxs,
		EnumInfos:         file_pbmesh_v2beta1_grpc_route_proto_enumTypes,
		MessageInfos:      file_pbmesh_v2beta1_grpc_route_proto_msgTypes,
	}.Build()
	File_pbmesh_v2beta1_grpc_route_proto = out.File
	file_pbmesh_v2beta1_grpc_route_proto_rawDesc = nil
	file_pbmesh_v2beta1_grpc_route_proto_goTypes = nil
	file_pbmesh_v2beta1_grpc_route_proto_depIdxs = nil
}

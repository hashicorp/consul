// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: pbmesh/v2beta1/pbproxystate/header_mutations.proto

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

// +kubebuilder:validation:Enum=APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD;APPEND_ACTION_OVERWRITE_IF_EXISTS_OR_ADD
// +kubebuilder:validation:Type=string
type AppendAction int32

const (
	// buf:lint:ignore ENUM_ZERO_VALUE_SUFFIX
	AppendAction_APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD    AppendAction = 0
	AppendAction_APPEND_ACTION_OVERWRITE_IF_EXISTS_OR_ADD AppendAction = 1
)

// Enum value maps for AppendAction.
var (
	AppendAction_name = map[int32]string{
		0: "APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD",
		1: "APPEND_ACTION_OVERWRITE_IF_EXISTS_OR_ADD",
	}
	AppendAction_value = map[string]int32{
		"APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD":    0,
		"APPEND_ACTION_OVERWRITE_IF_EXISTS_OR_ADD": 1,
	}
)

func (x AppendAction) Enum() *AppendAction {
	p := new(AppendAction)
	*p = x
	return p
}

func (x AppendAction) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (AppendAction) Descriptor() protoreflect.EnumDescriptor {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_enumTypes[0].Descriptor()
}

func (AppendAction) Type() protoreflect.EnumType {
	return &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_enumTypes[0]
}

func (x AppendAction) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use AppendAction.Descriptor instead.
func (AppendAction) EnumDescriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP(), []int{0}
}

// Note: it's nice to have this list of header mutations as opposed to configuration similar to Envoy because it
// translates more nicely from GAMMA HTTPRoute, and our existing service router config. Then xds code can handle turning
// it into envoy xds.
type HeaderMutation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Action:
	//
	//	*HeaderMutation_RequestHeaderAdd
	//	*HeaderMutation_RequestHeaderRemove
	//	*HeaderMutation_ResponseHeaderAdd
	//	*HeaderMutation_ResponseHeaderRemove
	Action isHeaderMutation_Action `protobuf_oneof:"action"`
}

func (x *HeaderMutation) Reset() {
	*x = HeaderMutation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HeaderMutation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HeaderMutation) ProtoMessage() {}

func (x *HeaderMutation) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HeaderMutation.ProtoReflect.Descriptor instead.
func (*HeaderMutation) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP(), []int{0}
}

func (m *HeaderMutation) GetAction() isHeaderMutation_Action {
	if m != nil {
		return m.Action
	}
	return nil
}

func (x *HeaderMutation) GetRequestHeaderAdd() *RequestHeaderAdd {
	if x, ok := x.GetAction().(*HeaderMutation_RequestHeaderAdd); ok {
		return x.RequestHeaderAdd
	}
	return nil
}

func (x *HeaderMutation) GetRequestHeaderRemove() *RequestHeaderRemove {
	if x, ok := x.GetAction().(*HeaderMutation_RequestHeaderRemove); ok {
		return x.RequestHeaderRemove
	}
	return nil
}

func (x *HeaderMutation) GetResponseHeaderAdd() *ResponseHeaderAdd {
	if x, ok := x.GetAction().(*HeaderMutation_ResponseHeaderAdd); ok {
		return x.ResponseHeaderAdd
	}
	return nil
}

func (x *HeaderMutation) GetResponseHeaderRemove() *ResponseHeaderRemove {
	if x, ok := x.GetAction().(*HeaderMutation_ResponseHeaderRemove); ok {
		return x.ResponseHeaderRemove
	}
	return nil
}

type isHeaderMutation_Action interface {
	isHeaderMutation_Action()
}

type HeaderMutation_RequestHeaderAdd struct {
	RequestHeaderAdd *RequestHeaderAdd `protobuf:"bytes,1,opt,name=request_header_add,json=requestHeaderAdd,proto3,oneof"`
}

type HeaderMutation_RequestHeaderRemove struct {
	RequestHeaderRemove *RequestHeaderRemove `protobuf:"bytes,2,opt,name=request_header_remove,json=requestHeaderRemove,proto3,oneof"`
}

type HeaderMutation_ResponseHeaderAdd struct {
	ResponseHeaderAdd *ResponseHeaderAdd `protobuf:"bytes,3,opt,name=response_header_add,json=responseHeaderAdd,proto3,oneof"`
}

type HeaderMutation_ResponseHeaderRemove struct {
	ResponseHeaderRemove *ResponseHeaderRemove `protobuf:"bytes,4,opt,name=response_header_remove,json=responseHeaderRemove,proto3,oneof"`
}

func (*HeaderMutation_RequestHeaderAdd) isHeaderMutation_Action() {}

func (*HeaderMutation_RequestHeaderRemove) isHeaderMutation_Action() {}

func (*HeaderMutation_ResponseHeaderAdd) isHeaderMutation_Action() {}

func (*HeaderMutation_ResponseHeaderRemove) isHeaderMutation_Action() {}

type RequestHeaderAdd struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Header       *Header      `protobuf:"bytes,1,opt,name=header,proto3" json:"header,omitempty"`
	AppendAction AppendAction `protobuf:"varint,2,opt,name=append_action,json=appendAction,proto3,enum=hashicorp.consul.mesh.v2beta1.pbproxystate.AppendAction" json:"append_action,omitempty"`
}

func (x *RequestHeaderAdd) Reset() {
	*x = RequestHeaderAdd{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RequestHeaderAdd) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RequestHeaderAdd) ProtoMessage() {}

func (x *RequestHeaderAdd) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RequestHeaderAdd.ProtoReflect.Descriptor instead.
func (*RequestHeaderAdd) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP(), []int{1}
}

func (x *RequestHeaderAdd) GetHeader() *Header {
	if x != nil {
		return x.Header
	}
	return nil
}

func (x *RequestHeaderAdd) GetAppendAction() AppendAction {
	if x != nil {
		return x.AppendAction
	}
	return AppendAction_APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD
}

type RequestHeaderRemove struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	HeaderKeys []string `protobuf:"bytes,1,rep,name=header_keys,json=headerKeys,proto3" json:"header_keys,omitempty"`
}

func (x *RequestHeaderRemove) Reset() {
	*x = RequestHeaderRemove{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RequestHeaderRemove) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RequestHeaderRemove) ProtoMessage() {}

func (x *RequestHeaderRemove) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RequestHeaderRemove.ProtoReflect.Descriptor instead.
func (*RequestHeaderRemove) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP(), []int{2}
}

func (x *RequestHeaderRemove) GetHeaderKeys() []string {
	if x != nil {
		return x.HeaderKeys
	}
	return nil
}

type ResponseHeaderAdd struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Header       *Header      `protobuf:"bytes,1,opt,name=header,proto3" json:"header,omitempty"`
	AppendAction AppendAction `protobuf:"varint,2,opt,name=append_action,json=appendAction,proto3,enum=hashicorp.consul.mesh.v2beta1.pbproxystate.AppendAction" json:"append_action,omitempty"`
}

func (x *ResponseHeaderAdd) Reset() {
	*x = ResponseHeaderAdd{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ResponseHeaderAdd) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseHeaderAdd) ProtoMessage() {}

func (x *ResponseHeaderAdd) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseHeaderAdd.ProtoReflect.Descriptor instead.
func (*ResponseHeaderAdd) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP(), []int{3}
}

func (x *ResponseHeaderAdd) GetHeader() *Header {
	if x != nil {
		return x.Header
	}
	return nil
}

func (x *ResponseHeaderAdd) GetAppendAction() AppendAction {
	if x != nil {
		return x.AppendAction
	}
	return AppendAction_APPEND_ACTION_APPEND_IF_EXISTS_OR_ADD
}

type ResponseHeaderRemove struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	HeaderKeys []string `protobuf:"bytes,1,rep,name=header_keys,json=headerKeys,proto3" json:"header_keys,omitempty"`
}

func (x *ResponseHeaderRemove) Reset() {
	*x = ResponseHeaderRemove{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ResponseHeaderRemove) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseHeaderRemove) ProtoMessage() {}

func (x *ResponseHeaderRemove) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseHeaderRemove.ProtoReflect.Descriptor instead.
func (*ResponseHeaderRemove) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP(), []int{4}
}

func (x *ResponseHeaderRemove) GetHeaderKeys() []string {
	if x != nil {
		return x.HeaderKeys
	}
	return nil
}

type Header struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key   string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Header) Reset() {
	*x = Header{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Header) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Header) ProtoMessage() {}

func (x *Header) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Header.ProtoReflect.Descriptor instead.
func (*Header) Descriptor() ([]byte, []int) {
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP(), []int{5}
}

func (x *Header) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *Header) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

var File_pbmesh_v2beta1_pbproxystate_header_mutations_proto protoreflect.FileDescriptor

var file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDesc = []byte{
	0x0a, 0x32, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2f, 0x68, 0x65,
	0x61, 0x64, 0x65, 0x72, 0x5f, 0x6d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x2a, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e,
	0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65,
	0x22, 0xea, 0x03, 0x0a, 0x0e, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x75, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x12, 0x6c, 0x0a, 0x12, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x68,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x5f, 0x61, 0x64, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x3c, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e,
	0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x41, 0x64, 0x64, 0x48, 0x00, 0x52,
	0x10, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x41, 0x64,
	0x64, 0x12, 0x75, 0x0a, 0x15, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x68, 0x65, 0x61,
	0x64, 0x65, 0x72, 0x5f, 0x72, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x3f, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x52, 0x65, 0x6d, 0x6f, 0x76,
	0x65, 0x48, 0x00, 0x52, 0x13, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64,
	0x65, 0x72, 0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x12, 0x6f, 0x0a, 0x13, 0x72, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x5f, 0x61, 0x64, 0x64, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x3d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65,
	0x72, 0x41, 0x64, 0x64, 0x48, 0x00, 0x52, 0x11, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x41, 0x64, 0x64, 0x12, 0x78, 0x0a, 0x16, 0x72, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x5f, 0x72, 0x65, 0x6d,
	0x6f, 0x76, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x40, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73,
	0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78,
	0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x48, 0x00, 0x52, 0x14, 0x72,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x52, 0x65, 0x6d,
	0x6f, 0x76, 0x65, 0x42, 0x08, 0x0a, 0x06, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0xbd, 0x01,
	0x0a, 0x10, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x41,
	0x64, 0x64, 0x12, 0x4a, 0x0a, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x32, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e,
	0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x52, 0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x5d,
	0x0a, 0x0d, 0x61, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x5f, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x38, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x2e, 0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x0c, 0x61, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x36, 0x0a,
	0x13, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x52, 0x65,
	0x6d, 0x6f, 0x76, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x5f, 0x6b,
	0x65, 0x79, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0a, 0x68, 0x65, 0x61, 0x64, 0x65,
	0x72, 0x4b, 0x65, 0x79, 0x73, 0x22, 0xbe, 0x01, 0x0a, 0x11, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x41, 0x64, 0x64, 0x12, 0x4a, 0x0a, 0x06, 0x68,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x32, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72,
	0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x52,
	0x06, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x5d, 0x0a, 0x0d, 0x61, 0x70, 0x70, 0x65, 0x6e,
	0x64, 0x5f, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x38,
	0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x70,
	0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x41, 0x70, 0x70, 0x65,
	0x6e, 0x64, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0c, 0x61, 0x70, 0x70, 0x65, 0x6e, 0x64,
	0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x37, 0x0a, 0x14, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x52, 0x65, 0x6d, 0x6f, 0x76, 0x65, 0x12, 0x1f,
	0x0a, 0x0b, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x5f, 0x6b, 0x65, 0x79, 0x73, 0x18, 0x01, 0x20,
	0x03, 0x28, 0x09, 0x52, 0x0a, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x4b, 0x65, 0x79, 0x73, 0x22,
	0x30, 0x0a, 0x06, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x2a, 0x67, 0x0a, 0x0c, 0x41, 0x70, 0x70, 0x65, 0x6e, 0x64, 0x41, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x29, 0x0a, 0x25, 0x41, 0x50, 0x50, 0x45, 0x4e, 0x44, 0x5f, 0x41, 0x43, 0x54, 0x49,
	0x4f, 0x4e, 0x5f, 0x41, 0x50, 0x50, 0x45, 0x4e, 0x44, 0x5f, 0x49, 0x46, 0x5f, 0x45, 0x58, 0x49,
	0x53, 0x54, 0x53, 0x5f, 0x4f, 0x52, 0x5f, 0x41, 0x44, 0x44, 0x10, 0x00, 0x12, 0x2c, 0x0a, 0x28,
	0x41, 0x50, 0x50, 0x45, 0x4e, 0x44, 0x5f, 0x41, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x4f, 0x56,
	0x45, 0x52, 0x57, 0x52, 0x49, 0x54, 0x45, 0x5f, 0x49, 0x46, 0x5f, 0x45, 0x58, 0x49, 0x53, 0x54,
	0x53, 0x5f, 0x4f, 0x52, 0x5f, 0x41, 0x44, 0x44, 0x10, 0x01, 0x42, 0xda, 0x02, 0x0a, 0x2e, 0x63,
	0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x42, 0x14, 0x48,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x4d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x50, 0x72,
	0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x44, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f,
	0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x70,
	0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0xa2, 0x02, 0x05, 0x48, 0x43,
	0x4d, 0x56, 0x50, 0xaa, 0x02, 0x2a, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e,
	0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73, 0x68, 0x2e, 0x56, 0x32, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x2e, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65,
	0xca, 0x02, 0x2a, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0xe2, 0x02, 0x36,
	0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x50, 0x62,
	0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65,
	0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x2e, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x65, 0x73, 0x68,
	0x3a, 0x3a, 0x56, 0x32, 0x62, 0x65, 0x74, 0x61, 0x31, 0x3a, 0x3a, 0x50, 0x62, 0x70, 0x72, 0x6f,
	0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescOnce sync.Once
	file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescData = file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDesc
)

func file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescGZIP() []byte {
	file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescOnce.Do(func() {
		file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescData)
	})
	return file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDescData
}

var file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_goTypes = []interface{}{
	(AppendAction)(0),            // 0: hashicorp.consul.mesh.v2beta1.pbproxystate.AppendAction
	(*HeaderMutation)(nil),       // 1: hashicorp.consul.mesh.v2beta1.pbproxystate.HeaderMutation
	(*RequestHeaderAdd)(nil),     // 2: hashicorp.consul.mesh.v2beta1.pbproxystate.RequestHeaderAdd
	(*RequestHeaderRemove)(nil),  // 3: hashicorp.consul.mesh.v2beta1.pbproxystate.RequestHeaderRemove
	(*ResponseHeaderAdd)(nil),    // 4: hashicorp.consul.mesh.v2beta1.pbproxystate.ResponseHeaderAdd
	(*ResponseHeaderRemove)(nil), // 5: hashicorp.consul.mesh.v2beta1.pbproxystate.ResponseHeaderRemove
	(*Header)(nil),               // 6: hashicorp.consul.mesh.v2beta1.pbproxystate.Header
}
var file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_depIdxs = []int32{
	2, // 0: hashicorp.consul.mesh.v2beta1.pbproxystate.HeaderMutation.request_header_add:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.RequestHeaderAdd
	3, // 1: hashicorp.consul.mesh.v2beta1.pbproxystate.HeaderMutation.request_header_remove:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.RequestHeaderRemove
	4, // 2: hashicorp.consul.mesh.v2beta1.pbproxystate.HeaderMutation.response_header_add:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.ResponseHeaderAdd
	5, // 3: hashicorp.consul.mesh.v2beta1.pbproxystate.HeaderMutation.response_header_remove:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.ResponseHeaderRemove
	6, // 4: hashicorp.consul.mesh.v2beta1.pbproxystate.RequestHeaderAdd.header:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.Header
	0, // 5: hashicorp.consul.mesh.v2beta1.pbproxystate.RequestHeaderAdd.append_action:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.AppendAction
	6, // 6: hashicorp.consul.mesh.v2beta1.pbproxystate.ResponseHeaderAdd.header:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.Header
	0, // 7: hashicorp.consul.mesh.v2beta1.pbproxystate.ResponseHeaderAdd.append_action:type_name -> hashicorp.consul.mesh.v2beta1.pbproxystate.AppendAction
	8, // [8:8] is the sub-list for method output_type
	8, // [8:8] is the sub-list for method input_type
	8, // [8:8] is the sub-list for extension type_name
	8, // [8:8] is the sub-list for extension extendee
	0, // [0:8] is the sub-list for field type_name
}

func init() { file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_init() }
func file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_init() {
	if File_pbmesh_v2beta1_pbproxystate_header_mutations_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HeaderMutation); i {
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
		file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RequestHeaderAdd); i {
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
		file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RequestHeaderRemove); i {
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
		file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ResponseHeaderAdd); i {
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
		file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ResponseHeaderRemove); i {
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
		file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Header); i {
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
	file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*HeaderMutation_RequestHeaderAdd)(nil),
		(*HeaderMutation_RequestHeaderRemove)(nil),
		(*HeaderMutation_ResponseHeaderAdd)(nil),
		(*HeaderMutation_ResponseHeaderRemove)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_goTypes,
		DependencyIndexes: file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_depIdxs,
		EnumInfos:         file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_enumTypes,
		MessageInfos:      file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_msgTypes,
	}.Build()
	File_pbmesh_v2beta1_pbproxystate_header_mutations_proto = out.File
	file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_rawDesc = nil
	file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_goTypes = nil
	file_pbmesh_v2beta1_pbproxystate_header_mutations_proto_depIdxs = nil
}

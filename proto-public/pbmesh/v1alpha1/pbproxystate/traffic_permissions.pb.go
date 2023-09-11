// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v1alpha1/pbproxystate/traffic_permissions.proto

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

type TrafficPermissionAction int32

const (
	TrafficPermissionAction_TRAFFIC_PERMISSION_ACTION_UNSPECIFIED TrafficPermissionAction = 0
	TrafficPermissionAction_TRAFFIC_PERMISSION_ACTION_DENY        TrafficPermissionAction = 1
	TrafficPermissionAction_TRAFFIC_PERMISSION_ACTION_ALLOW       TrafficPermissionAction = 2
)

// Enum value maps for TrafficPermissionAction.
var (
	TrafficPermissionAction_name = map[int32]string{
		0: "TRAFFIC_PERMISSION_ACTION_UNSPECIFIED",
		1: "TRAFFIC_PERMISSION_ACTION_DENY",
		2: "TRAFFIC_PERMISSION_ACTION_ALLOW",
	}
	TrafficPermissionAction_value = map[string]int32{
		"TRAFFIC_PERMISSION_ACTION_UNSPECIFIED": 0,
		"TRAFFIC_PERMISSION_ACTION_DENY":        1,
		"TRAFFIC_PERMISSION_ACTION_ALLOW":       2,
	}
)

func (x TrafficPermissionAction) Enum() *TrafficPermissionAction {
	p := new(TrafficPermissionAction)
	*p = x
	return p
}

func (x TrafficPermissionAction) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (TrafficPermissionAction) Descriptor() protoreflect.EnumDescriptor {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_enumTypes[0].Descriptor()
}

func (TrafficPermissionAction) Type() protoreflect.EnumType {
	return &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_enumTypes[0]
}

func (x TrafficPermissionAction) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use TrafficPermissionAction.Descriptor instead.
func (TrafficPermissionAction) EnumDescriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP(), []int{0}
}

type L7TrafficPermissions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *L7TrafficPermissions) Reset() {
	*x = L7TrafficPermissions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *L7TrafficPermissions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*L7TrafficPermissions) ProtoMessage() {}

func (x *L7TrafficPermissions) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use L7TrafficPermissions.ProtoReflect.Descriptor instead.
func (*L7TrafficPermissions) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP(), []int{0}
}

type L4TrafficPermissions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DefaultAction    TrafficPermissionAction `protobuf:"varint,1,opt,name=default_action,json=defaultAction,proto3,enum=hashicorp.consul.mesh.v1alpha1.pbproxystate.TrafficPermissionAction" json:"default_action,omitempty"`
	AllowPermissions []*L4Permission         `protobuf:"bytes,2,rep,name=allow_permissions,json=allowPermissions,proto3" json:"allow_permissions,omitempty"`
	DenyPermissions  []*L4Permission         `protobuf:"bytes,3,rep,name=deny_permissions,json=denyPermissions,proto3" json:"deny_permissions,omitempty"`
}

func (x *L4TrafficPermissions) Reset() {
	*x = L4TrafficPermissions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *L4TrafficPermissions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*L4TrafficPermissions) ProtoMessage() {}

func (x *L4TrafficPermissions) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use L4TrafficPermissions.ProtoReflect.Descriptor instead.
func (*L4TrafficPermissions) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP(), []int{1}
}

func (x *L4TrafficPermissions) GetDefaultAction() TrafficPermissionAction {
	if x != nil {
		return x.DefaultAction
	}
	return TrafficPermissionAction_TRAFFIC_PERMISSION_ACTION_UNSPECIFIED
}

func (x *L4TrafficPermissions) GetAllowPermissions() []*L4Permission {
	if x != nil {
		return x.AllowPermissions
	}
	return nil
}

func (x *L4TrafficPermissions) GetDenyPermissions() []*L4Permission {
	if x != nil {
		return x.DenyPermissions
	}
	return nil
}

type L4Permission struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Principals []*L4Principal `protobuf:"bytes,1,rep,name=principals,proto3" json:"principals,omitempty"`
}

func (x *L4Permission) Reset() {
	*x = L4Permission{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *L4Permission) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*L4Permission) ProtoMessage() {}

func (x *L4Permission) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use L4Permission.ProtoReflect.Descriptor instead.
func (*L4Permission) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP(), []int{2}
}

func (x *L4Permission) GetPrincipals() []*L4Principal {
	if x != nil {
		return x.Principals
	}
	return nil
}

// L4Principal maps into Source. We first convert this to Source before generating Envoy resources.
type L4Principal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SpiffeRegex          string   `protobuf:"bytes,1,opt,name=spiffe_regex,json=spiffeRegex,proto3" json:"spiffe_regex,omitempty"`
	ExcludeSpiffeRegexes []string `protobuf:"bytes,2,rep,name=exclude_spiffe_regexes,json=excludeSpiffeRegexes,proto3" json:"exclude_spiffe_regexes,omitempty"`
}

func (x *L4Principal) Reset() {
	*x = L4Principal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *L4Principal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*L4Principal) ProtoMessage() {}

func (x *L4Principal) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use L4Principal.ProtoReflect.Descriptor instead.
func (*L4Principal) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP(), []int{3}
}

func (x *L4Principal) GetSpiffeRegex() string {
	if x != nil {
		return x.SpiffeRegex
	}
	return ""
}

func (x *L4Principal) GetExcludeSpiffeRegexes() []string {
	if x != nil {
		return x.ExcludeSpiffeRegexes
	}
	return nil
}

type L7Principal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Spiffes        []*Spiffe `protobuf:"bytes,1,rep,name=spiffes,proto3" json:"spiffes,omitempty"`
	ExcludeSpiffes []*Spiffe `protobuf:"bytes,2,rep,name=exclude_spiffes,json=excludeSpiffes,proto3" json:"exclude_spiffes,omitempty"`
}

func (x *L7Principal) Reset() {
	*x = L7Principal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *L7Principal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*L7Principal) ProtoMessage() {}

func (x *L7Principal) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use L7Principal.ProtoReflect.Descriptor instead.
func (*L7Principal) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP(), []int{4}
}

func (x *L7Principal) GetSpiffes() []*Spiffe {
	if x != nil {
		return x.Spiffes
	}
	return nil
}

func (x *L7Principal) GetExcludeSpiffes() []*Spiffe {
	if x != nil {
		return x.ExcludeSpiffes
	}
	return nil
}

type Spiffe struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// regex is the regular expression for matching spiffe ids.
	Regex string `protobuf:"bytes,1,opt,name=regex,proto3" json:"regex,omitempty"`
	// xfcc specifies that Envoy needs to find the spiffe id in an xfcc header.
	// It is currently unused, but considering this is important for to avoid breaking changes.
	Xfcc bool `protobuf:"varint,2,opt,name=xfcc,proto3" json:"xfcc,omitempty"`
}

func (x *Spiffe) Reset() {
	*x = Spiffe{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Spiffe) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Spiffe) ProtoMessage() {}

func (x *Spiffe) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Spiffe.ProtoReflect.Descriptor instead.
func (*Spiffe) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP(), []int{5}
}

func (x *Spiffe) GetRegex() string {
	if x != nil {
		return x.Regex
	}
	return ""
}

func (x *Spiffe) GetXfcc() bool {
	if x != nil {
		return x.Xfcc
	}
	return false
}

var File_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto protoreflect.FileDescriptor

var file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDesc = []byte{
	0x0a, 0x36, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x2f, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2f, 0x74,
	0x72, 0x61, 0x66, 0x66, 0x69, 0x63, 0x5f, 0x70, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f,
	0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x2b, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79,
	0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0x16, 0x0a, 0x14, 0x4c, 0x37, 0x54, 0x72, 0x61, 0x66, 0x66,
	0x69, 0x63, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x22, 0xd1, 0x02,
	0x0a, 0x14, 0x4c, 0x34, 0x54, 0x72, 0x61, 0x66, 0x66, 0x69, 0x63, 0x50, 0x65, 0x72, 0x6d, 0x69,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x6b, 0x0a, 0x0e, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c,
	0x74, 0x5f, 0x61, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x44,
	0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e,
	0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x54, 0x72, 0x61,
	0x66, 0x66, 0x69, 0x63, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x41, 0x63,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0d, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x41, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x12, 0x66, 0x0a, 0x11, 0x61, 0x6c, 0x6c, 0x6f, 0x77, 0x5f, 0x70, 0x65, 0x72,
	0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x39,
	0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e,
	0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x4c, 0x34, 0x50,
	0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x10, 0x61, 0x6c, 0x6c, 0x6f, 0x77,
	0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x64, 0x0a, 0x10, 0x64,
	0x65, 0x6e, 0x79, 0x5f, 0x70, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x18,
	0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x39, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74,
	0x61, 0x74, 0x65, 0x2e, 0x4c, 0x34, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e,
	0x52, 0x0f, 0x64, 0x65, 0x6e, 0x79, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e,
	0x73, 0x22, 0x68, 0x0a, 0x0c, 0x4c, 0x34, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f,
	0x6e, 0x12, 0x58, 0x0a, 0x0a, 0x70, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61, 0x6c, 0x73, 0x18,
	0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x38, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74,
	0x61, 0x74, 0x65, 0x2e, 0x4c, 0x34, 0x50, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61, 0x6c, 0x52,
	0x0a, 0x70, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61, 0x6c, 0x73, 0x22, 0x66, 0x0a, 0x0b, 0x4c,
	0x34, 0x50, 0x72, 0x69, 0x6e, 0x63, 0x69, 0x70, 0x61, 0x6c, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x70,
	0x69, 0x66, 0x66, 0x65, 0x5f, 0x72, 0x65, 0x67, 0x65, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0b, 0x73, 0x70, 0x69, 0x66, 0x66, 0x65, 0x52, 0x65, 0x67, 0x65, 0x78, 0x12, 0x34, 0x0a,
	0x16, 0x65, 0x78, 0x63, 0x6c, 0x75, 0x64, 0x65, 0x5f, 0x73, 0x70, 0x69, 0x66, 0x66, 0x65, 0x5f,
	0x72, 0x65, 0x67, 0x65, 0x78, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x14, 0x65,
	0x78, 0x63, 0x6c, 0x75, 0x64, 0x65, 0x53, 0x70, 0x69, 0x66, 0x66, 0x65, 0x52, 0x65, 0x67, 0x65,
	0x78, 0x65, 0x73, 0x22, 0xba, 0x01, 0x0a, 0x0b, 0x4c, 0x37, 0x50, 0x72, 0x69, 0x6e, 0x63, 0x69,
	0x70, 0x61, 0x6c, 0x12, 0x4d, 0x0a, 0x07, 0x73, 0x70, 0x69, 0x66, 0x66, 0x65, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x2e, 0x53, 0x70, 0x69, 0x66, 0x66, 0x65, 0x52, 0x07, 0x73, 0x70, 0x69, 0x66, 0x66,
	0x65, 0x73, 0x12, 0x5c, 0x0a, 0x0f, 0x65, 0x78, 0x63, 0x6c, 0x75, 0x64, 0x65, 0x5f, 0x73, 0x70,
	0x69, 0x66, 0x66, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70,
	0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x53, 0x70, 0x69, 0x66, 0x66, 0x65,
	0x52, 0x0e, 0x65, 0x78, 0x63, 0x6c, 0x75, 0x64, 0x65, 0x53, 0x70, 0x69, 0x66, 0x66, 0x65, 0x73,
	0x22, 0x32, 0x0a, 0x06, 0x53, 0x70, 0x69, 0x66, 0x66, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x72, 0x65,
	0x67, 0x65, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x72, 0x65, 0x67, 0x65, 0x78,
	0x12, 0x12, 0x0a, 0x04, 0x78, 0x66, 0x63, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x04,
	0x78, 0x66, 0x63, 0x63, 0x2a, 0x8d, 0x01, 0x0a, 0x17, 0x54, 0x72, 0x61, 0x66, 0x66, 0x69, 0x63,
	0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x41, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x29, 0x0a, 0x25, 0x54, 0x52, 0x41, 0x46, 0x46, 0x49, 0x43, 0x5f, 0x50, 0x45, 0x52, 0x4d,
	0x49, 0x53, 0x53, 0x49, 0x4f, 0x4e, 0x5f, 0x41, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x55, 0x4e,
	0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x22, 0x0a, 0x1e, 0x54,
	0x52, 0x41, 0x46, 0x46, 0x49, 0x43, 0x5f, 0x50, 0x45, 0x52, 0x4d, 0x49, 0x53, 0x53, 0x49, 0x4f,
	0x4e, 0x5f, 0x41, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x44, 0x45, 0x4e, 0x59, 0x10, 0x01, 0x12,
	0x23, 0x0a, 0x1f, 0x54, 0x52, 0x41, 0x46, 0x46, 0x49, 0x43, 0x5f, 0x50, 0x45, 0x52, 0x4d, 0x49,
	0x53, 0x53, 0x49, 0x4f, 0x4e, 0x5f, 0x41, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x41, 0x4c, 0x4c,
	0x4f, 0x57, 0x10, 0x02, 0x42, 0xe3, 0x02, 0x0a, 0x2f, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73,
	0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65,
	0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x70, 0x62, 0x70, 0x72,
	0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x42, 0x17, 0x54, 0x72, 0x61, 0x66, 0x66, 0x69,
	0x63, 0x50, 0x65, 0x72, 0x6d, 0x69, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x73, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x50, 0x01, 0x5a, 0x45, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x2f, 0x70, 0x62,
	0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x70, 0x62,
	0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0xa2, 0x02, 0x05, 0x48, 0x43, 0x4d,
	0x56, 0x50, 0xaa, 0x02, 0x2b, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73, 0x68, 0x2e, 0x56, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x2e, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65,
	0xca, 0x02, 0x2b, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x5c, 0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0xe2, 0x02,
	0x37, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75,
	0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x5c,
	0x50, 0x62, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x5c, 0x47, 0x50, 0x42,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x2f, 0x48, 0x61, 0x73, 0x68, 0x69,
	0x63, 0x6f, 0x72, 0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x65,
	0x73, 0x68, 0x3a, 0x3a, 0x56, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x3a, 0x3a, 0x50, 0x62,
	0x70, 0x72, 0x6f, 0x78, 0x79, 0x73, 0x74, 0x61, 0x74, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescOnce sync.Once
	file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescData = file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDesc
)

func file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescGZIP() []byte {
	file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescOnce.Do(func() {
		file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescData)
	})
	return file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDescData
}

var file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_goTypes = []interface{}{
	(TrafficPermissionAction)(0), // 0: hashicorp.consul.mesh.v1alpha1.pbproxystate.TrafficPermissionAction
	(*L7TrafficPermissions)(nil), // 1: hashicorp.consul.mesh.v1alpha1.pbproxystate.L7TrafficPermissions
	(*L4TrafficPermissions)(nil), // 2: hashicorp.consul.mesh.v1alpha1.pbproxystate.L4TrafficPermissions
	(*L4Permission)(nil),         // 3: hashicorp.consul.mesh.v1alpha1.pbproxystate.L4Permission
	(*L4Principal)(nil),          // 4: hashicorp.consul.mesh.v1alpha1.pbproxystate.L4Principal
	(*L7Principal)(nil),          // 5: hashicorp.consul.mesh.v1alpha1.pbproxystate.L7Principal
	(*Spiffe)(nil),               // 6: hashicorp.consul.mesh.v1alpha1.pbproxystate.Spiffe
}
var file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_depIdxs = []int32{
	0, // 0: hashicorp.consul.mesh.v1alpha1.pbproxystate.L4TrafficPermissions.default_action:type_name -> hashicorp.consul.mesh.v1alpha1.pbproxystate.TrafficPermissionAction
	3, // 1: hashicorp.consul.mesh.v1alpha1.pbproxystate.L4TrafficPermissions.allow_permissions:type_name -> hashicorp.consul.mesh.v1alpha1.pbproxystate.L4Permission
	3, // 2: hashicorp.consul.mesh.v1alpha1.pbproxystate.L4TrafficPermissions.deny_permissions:type_name -> hashicorp.consul.mesh.v1alpha1.pbproxystate.L4Permission
	4, // 3: hashicorp.consul.mesh.v1alpha1.pbproxystate.L4Permission.principals:type_name -> hashicorp.consul.mesh.v1alpha1.pbproxystate.L4Principal
	6, // 4: hashicorp.consul.mesh.v1alpha1.pbproxystate.L7Principal.spiffes:type_name -> hashicorp.consul.mesh.v1alpha1.pbproxystate.Spiffe
	6, // 5: hashicorp.consul.mesh.v1alpha1.pbproxystate.L7Principal.exclude_spiffes:type_name -> hashicorp.consul.mesh.v1alpha1.pbproxystate.Spiffe
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_init() }
func file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_init() {
	if File_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*L7TrafficPermissions); i {
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
		file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*L4TrafficPermissions); i {
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
		file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*L4Permission); i {
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
		file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*L4Principal); i {
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
		file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*L7Principal); i {
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
		file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Spiffe); i {
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
			RawDescriptor: file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_goTypes,
		DependencyIndexes: file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_depIdxs,
		EnumInfos:         file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_enumTypes,
		MessageInfos:      file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_msgTypes,
	}.Build()
	File_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto = out.File
	file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_rawDesc = nil
	file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_goTypes = nil
	file_pbmesh_v1alpha1_pbproxystate_traffic_permissions_proto_depIdxs = nil
}

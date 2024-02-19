// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: private/pbdemo/v1/demo.proto

// This package contains fake resource types, which are useful for working on
// Consul's generic storage APIs.

package demov1

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

type Genre int32

const (
	Genre_GENRE_UNSPECIFIED Genre = 0
	Genre_GENRE_JAZZ        Genre = 1
	Genre_GENRE_FOLK        Genre = 2
	Genre_GENRE_POP         Genre = 3
	Genre_GENRE_METAL       Genre = 4
	Genre_GENRE_PUNK        Genre = 5
	Genre_GENRE_BLUES       Genre = 6
	Genre_GENRE_R_AND_B     Genre = 7
	Genre_GENRE_COUNTRY     Genre = 8
	Genre_GENRE_DISCO       Genre = 9
	Genre_GENRE_SKA         Genre = 10
	Genre_GENRE_HIP_HOP     Genre = 11
	Genre_GENRE_INDIE       Genre = 12
)

// Enum value maps for Genre.
var (
	Genre_name = map[int32]string{
		0:  "GENRE_UNSPECIFIED",
		1:  "GENRE_JAZZ",
		2:  "GENRE_FOLK",
		3:  "GENRE_POP",
		4:  "GENRE_METAL",
		5:  "GENRE_PUNK",
		6:  "GENRE_BLUES",
		7:  "GENRE_R_AND_B",
		8:  "GENRE_COUNTRY",
		9:  "GENRE_DISCO",
		10: "GENRE_SKA",
		11: "GENRE_HIP_HOP",
		12: "GENRE_INDIE",
	}
	Genre_value = map[string]int32{
		"GENRE_UNSPECIFIED": 0,
		"GENRE_JAZZ":        1,
		"GENRE_FOLK":        2,
		"GENRE_POP":         3,
		"GENRE_METAL":       4,
		"GENRE_PUNK":        5,
		"GENRE_BLUES":       6,
		"GENRE_R_AND_B":     7,
		"GENRE_COUNTRY":     8,
		"GENRE_DISCO":       9,
		"GENRE_SKA":         10,
		"GENRE_HIP_HOP":     11,
		"GENRE_INDIE":       12,
	}
)

func (x Genre) Enum() *Genre {
	p := new(Genre)
	*p = x
	return p
}

func (x Genre) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Genre) Descriptor() protoreflect.EnumDescriptor {
	return file_private_pbdemo_v1_demo_proto_enumTypes[0].Descriptor()
}

func (Genre) Type() protoreflect.EnumType {
	return &file_private_pbdemo_v1_demo_proto_enumTypes[0]
}

func (x Genre) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Genre.Descriptor instead.
func (Genre) EnumDescriptor() ([]byte, []int) {
	return file_private_pbdemo_v1_demo_proto_rawDescGZIP(), []int{0}
}

// Cluster scoped resource.
type Executive struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Position string `protobuf:"bytes,1,opt,name=position,proto3" json:"position,omitempty"`
}

func (x *Executive) Reset() {
	*x = Executive{}
	if protoimpl.UnsafeEnabled {
		mi := &file_private_pbdemo_v1_demo_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Executive) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Executive) ProtoMessage() {}

func (x *Executive) ProtoReflect() protoreflect.Message {
	mi := &file_private_pbdemo_v1_demo_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Executive.ProtoReflect.Descriptor instead.
func (*Executive) Descriptor() ([]byte, []int) {
	return file_private_pbdemo_v1_demo_proto_rawDescGZIP(), []int{0}
}

func (x *Executive) GetPosition() string {
	if x != nil {
		return x.Position
	}
	return ""
}

// Partition scoped resource
type RecordLabel struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name        string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Description string `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
}

func (x *RecordLabel) Reset() {
	*x = RecordLabel{}
	if protoimpl.UnsafeEnabled {
		mi := &file_private_pbdemo_v1_demo_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RecordLabel) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RecordLabel) ProtoMessage() {}

func (x *RecordLabel) ProtoReflect() protoreflect.Message {
	mi := &file_private_pbdemo_v1_demo_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RecordLabel.ProtoReflect.Descriptor instead.
func (*RecordLabel) Descriptor() ([]byte, []int) {
	return file_private_pbdemo_v1_demo_proto_rawDescGZIP(), []int{1}
}

func (x *RecordLabel) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *RecordLabel) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

// Namespace scoped resource.
type Artist struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name         string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Description  string `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
	Genre        Genre  `protobuf:"varint,3,opt,name=genre,proto3,enum=hashicorp.consul.internal.demo.v1.Genre" json:"genre,omitempty"`
	GroupMembers int32  `protobuf:"varint,4,opt,name=group_members,json=groupMembers,proto3" json:"group_members,omitempty"`
}

func (x *Artist) Reset() {
	*x = Artist{}
	if protoimpl.UnsafeEnabled {
		mi := &file_private_pbdemo_v1_demo_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Artist) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Artist) ProtoMessage() {}

func (x *Artist) ProtoReflect() protoreflect.Message {
	mi := &file_private_pbdemo_v1_demo_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Artist.ProtoReflect.Descriptor instead.
func (*Artist) Descriptor() ([]byte, []int) {
	return file_private_pbdemo_v1_demo_proto_rawDescGZIP(), []int{2}
}

func (x *Artist) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Artist) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *Artist) GetGenre() Genre {
	if x != nil {
		return x.Genre
	}
	return Genre_GENRE_UNSPECIFIED
}

func (x *Artist) GetGroupMembers() int32 {
	if x != nil {
		return x.GroupMembers
	}
	return 0
}

type Album struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name                string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	YearOfRelease       int32    `protobuf:"varint,2,opt,name=year_of_release,json=yearOfRelease,proto3" json:"year_of_release,omitempty"`
	CriticallyAcclaimed bool     `protobuf:"varint,3,opt,name=critically_acclaimed,json=criticallyAcclaimed,proto3" json:"critically_acclaimed,omitempty"`
	Tracks              []string `protobuf:"bytes,4,rep,name=tracks,proto3" json:"tracks,omitempty"`
}

func (x *Album) Reset() {
	*x = Album{}
	if protoimpl.UnsafeEnabled {
		mi := &file_private_pbdemo_v1_demo_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Album) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Album) ProtoMessage() {}

func (x *Album) ProtoReflect() protoreflect.Message {
	mi := &file_private_pbdemo_v1_demo_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Album.ProtoReflect.Descriptor instead.
func (*Album) Descriptor() ([]byte, []int) {
	return file_private_pbdemo_v1_demo_proto_rawDescGZIP(), []int{3}
}

func (x *Album) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Album) GetYearOfRelease() int32 {
	if x != nil {
		return x.YearOfRelease
	}
	return 0
}

func (x *Album) GetCriticallyAcclaimed() bool {
	if x != nil {
		return x.CriticallyAcclaimed
	}
	return false
}

func (x *Album) GetTracks() []string {
	if x != nil {
		return x.Tracks
	}
	return nil
}

type Concept struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *Concept) Reset() {
	*x = Concept{}
	if protoimpl.UnsafeEnabled {
		mi := &file_private_pbdemo_v1_demo_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Concept) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Concept) ProtoMessage() {}

func (x *Concept) ProtoReflect() protoreflect.Message {
	mi := &file_private_pbdemo_v1_demo_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Concept.ProtoReflect.Descriptor instead.
func (*Concept) Descriptor() ([]byte, []int) {
	return file_private_pbdemo_v1_demo_proto_rawDescGZIP(), []int{4}
}

var File_private_pbdemo_v1_demo_proto protoreflect.FileDescriptor

var file_private_pbdemo_v1_demo_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x2f, 0x70, 0x62, 0x64, 0x65, 0x6d, 0x6f,
	0x2f, 0x76, 0x31, 0x2f, 0x64, 0x65, 0x6d, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x21,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x64, 0x65, 0x6d, 0x6f, 0x2e, 0x76,
	0x31, 0x1a, 0x1c, 0x70, 0x62, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x61, 0x6e,
	0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0x2f, 0x0a, 0x09, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x76, 0x65, 0x12, 0x1a, 0x0a, 0x08,
	0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08,
	0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x3a, 0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x01,
	0x22, 0x4b, 0x0a, 0x0b, 0x52, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12,
	0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x3a, 0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x02, 0x22, 0xab, 0x01,
	0x0a, 0x06, 0x41, 0x72, 0x74, 0x69, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x20, 0x0a, 0x0b,
	0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x3e,
	0x0a, 0x05, 0x67, 0x65, 0x6e, 0x72, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x28, 0x2e,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x64, 0x65, 0x6d, 0x6f, 0x2e, 0x76,
	0x31, 0x2e, 0x47, 0x65, 0x6e, 0x72, 0x65, 0x52, 0x05, 0x67, 0x65, 0x6e, 0x72, 0x65, 0x12, 0x23,
	0x0a, 0x0d, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x5f, 0x6d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x73, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0c, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x4d, 0x65, 0x6d, 0x62,
	0x65, 0x72, 0x73, 0x3a, 0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x03, 0x22, 0x96, 0x01, 0x0a, 0x05,
	0x41, 0x6c, 0x62, 0x75, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x26, 0x0a, 0x0f, 0x79, 0x65, 0x61,
	0x72, 0x5f, 0x6f, 0x66, 0x5f, 0x72, 0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x05, 0x52, 0x0d, 0x79, 0x65, 0x61, 0x72, 0x4f, 0x66, 0x52, 0x65, 0x6c, 0x65, 0x61, 0x73,
	0x65, 0x12, 0x31, 0x0a, 0x14, 0x63, 0x72, 0x69, 0x74, 0x69, 0x63, 0x61, 0x6c, 0x6c, 0x79, 0x5f,
	0x61, 0x63, 0x63, 0x6c, 0x61, 0x69, 0x6d, 0x65, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x13, 0x63, 0x72, 0x69, 0x74, 0x69, 0x63, 0x61, 0x6c, 0x6c, 0x79, 0x41, 0x63, 0x63, 0x6c, 0x61,
	0x69, 0x6d, 0x65, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x18, 0x04,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x73, 0x3a, 0x06, 0xa2, 0x93,
	0x04, 0x02, 0x08, 0x03, 0x22, 0x11, 0x0a, 0x07, 0x43, 0x6f, 0x6e, 0x63, 0x65, 0x70, 0x74, 0x3a,
	0x06, 0xa2, 0x93, 0x04, 0x02, 0x08, 0x03, 0x2a, 0xe9, 0x01, 0x0a, 0x05, 0x47, 0x65, 0x6e, 0x72,
	0x65, 0x12, 0x15, 0x0a, 0x11, 0x47, 0x45, 0x4e, 0x52, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45,
	0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x0e, 0x0a, 0x0a, 0x47, 0x45, 0x4e, 0x52,
	0x45, 0x5f, 0x4a, 0x41, 0x5a, 0x5a, 0x10, 0x01, 0x12, 0x0e, 0x0a, 0x0a, 0x47, 0x45, 0x4e, 0x52,
	0x45, 0x5f, 0x46, 0x4f, 0x4c, 0x4b, 0x10, 0x02, 0x12, 0x0d, 0x0a, 0x09, 0x47, 0x45, 0x4e, 0x52,
	0x45, 0x5f, 0x50, 0x4f, 0x50, 0x10, 0x03, 0x12, 0x0f, 0x0a, 0x0b, 0x47, 0x45, 0x4e, 0x52, 0x45,
	0x5f, 0x4d, 0x45, 0x54, 0x41, 0x4c, 0x10, 0x04, 0x12, 0x0e, 0x0a, 0x0a, 0x47, 0x45, 0x4e, 0x52,
	0x45, 0x5f, 0x50, 0x55, 0x4e, 0x4b, 0x10, 0x05, 0x12, 0x0f, 0x0a, 0x0b, 0x47, 0x45, 0x4e, 0x52,
	0x45, 0x5f, 0x42, 0x4c, 0x55, 0x45, 0x53, 0x10, 0x06, 0x12, 0x11, 0x0a, 0x0d, 0x47, 0x45, 0x4e,
	0x52, 0x45, 0x5f, 0x52, 0x5f, 0x41, 0x4e, 0x44, 0x5f, 0x42, 0x10, 0x07, 0x12, 0x11, 0x0a, 0x0d,
	0x47, 0x45, 0x4e, 0x52, 0x45, 0x5f, 0x43, 0x4f, 0x55, 0x4e, 0x54, 0x52, 0x59, 0x10, 0x08, 0x12,
	0x0f, 0x0a, 0x0b, 0x47, 0x45, 0x4e, 0x52, 0x45, 0x5f, 0x44, 0x49, 0x53, 0x43, 0x4f, 0x10, 0x09,
	0x12, 0x0d, 0x0a, 0x09, 0x47, 0x45, 0x4e, 0x52, 0x45, 0x5f, 0x53, 0x4b, 0x41, 0x10, 0x0a, 0x12,
	0x11, 0x0a, 0x0d, 0x47, 0x45, 0x4e, 0x52, 0x45, 0x5f, 0x48, 0x49, 0x50, 0x5f, 0x48, 0x4f, 0x50,
	0x10, 0x0b, 0x12, 0x0f, 0x0a, 0x0b, 0x47, 0x45, 0x4e, 0x52, 0x45, 0x5f, 0x49, 0x4e, 0x44, 0x49,
	0x45, 0x10, 0x0c, 0x42, 0x97, 0x02, 0x0a, 0x25, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x69, 0x6e, 0x74,
	0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x64, 0x65, 0x6d, 0x6f, 0x2e, 0x76, 0x31, 0x42, 0x09, 0x44,
	0x65, 0x6d, 0x6f, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x3a, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x72,
	0x69, 0x76, 0x61, 0x74, 0x65, 0x2f, 0x70, 0x62, 0x64, 0x65, 0x6d, 0x6f, 0x2f, 0x76, 0x31, 0x3b,
	0x64, 0x65, 0x6d, 0x6f, 0x76, 0x31, 0xa2, 0x02, 0x04, 0x48, 0x43, 0x49, 0x44, 0xaa, 0x02, 0x21,
	0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x44, 0x65, 0x6d, 0x6f, 0x2e, 0x56,
	0x31, 0xca, 0x02, 0x21, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x5c, 0x43, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x5c, 0x44, 0x65,
	0x6d, 0x6f, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x2d, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61,
	0x6c, 0x5c, 0x44, 0x65, 0x6d, 0x6f, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74,
	0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x25, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x3a, 0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x49, 0x6e, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x3a, 0x3a, 0x44, 0x65, 0x6d, 0x6f, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_private_pbdemo_v1_demo_proto_rawDescOnce sync.Once
	file_private_pbdemo_v1_demo_proto_rawDescData = file_private_pbdemo_v1_demo_proto_rawDesc
)

func file_private_pbdemo_v1_demo_proto_rawDescGZIP() []byte {
	file_private_pbdemo_v1_demo_proto_rawDescOnce.Do(func() {
		file_private_pbdemo_v1_demo_proto_rawDescData = protoimpl.X.CompressGZIP(file_private_pbdemo_v1_demo_proto_rawDescData)
	})
	return file_private_pbdemo_v1_demo_proto_rawDescData
}

var file_private_pbdemo_v1_demo_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_private_pbdemo_v1_demo_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_private_pbdemo_v1_demo_proto_goTypes = []interface{}{
	(Genre)(0),          // 0: hashicorp.consul.internal.demo.v1.Genre
	(*Executive)(nil),   // 1: hashicorp.consul.internal.demo.v1.Executive
	(*RecordLabel)(nil), // 2: hashicorp.consul.internal.demo.v1.RecordLabel
	(*Artist)(nil),      // 3: hashicorp.consul.internal.demo.v1.Artist
	(*Album)(nil),       // 4: hashicorp.consul.internal.demo.v1.Album
	(*Concept)(nil),     // 5: hashicorp.consul.internal.demo.v1.Concept
}
var file_private_pbdemo_v1_demo_proto_depIdxs = []int32{
	0, // 0: hashicorp.consul.internal.demo.v1.Artist.genre:type_name -> hashicorp.consul.internal.demo.v1.Genre
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_private_pbdemo_v1_demo_proto_init() }
func file_private_pbdemo_v1_demo_proto_init() {
	if File_private_pbdemo_v1_demo_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_private_pbdemo_v1_demo_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Executive); i {
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
		file_private_pbdemo_v1_demo_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RecordLabel); i {
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
		file_private_pbdemo_v1_demo_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Artist); i {
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
		file_private_pbdemo_v1_demo_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Album); i {
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
		file_private_pbdemo_v1_demo_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Concept); i {
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
			RawDescriptor: file_private_pbdemo_v1_demo_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_private_pbdemo_v1_demo_proto_goTypes,
		DependencyIndexes: file_private_pbdemo_v1_demo_proto_depIdxs,
		EnumInfos:         file_private_pbdemo_v1_demo_proto_enumTypes,
		MessageInfos:      file_private_pbdemo_v1_demo_proto_msgTypes,
	}.Build()
	File_private_pbdemo_v1_demo_proto = out.File
	file_private_pbdemo_v1_demo_proto_rawDesc = nil
	file_private_pbdemo_v1_demo_proto_goTypes = nil
	file_private_pbdemo_v1_demo_proto_depIdxs = nil
}

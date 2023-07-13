// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: pbmesh/v1alpha1/upstreams.proto

package meshv1alpha1

import (
	v1alpha1 "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
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

type Upstreams struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Selection of workloads these upstreams should apply to.
	// These can be prefixes or specific workload names.
	Workloads *v1alpha1.WorkloadSelector `protobuf:"bytes,1,opt,name=workloads,proto3" json:"workloads,omitempty"`
	// upstreams is the list of explicit upstreams to define for the selected workloads.
	Upstreams []*Upstream `protobuf:"bytes,2,rep,name=upstreams,proto3" json:"upstreams,omitempty"`
	// pq_upstreams is the list of prepared query upstreams. This field is not supported directly in v2
	// and should only be used for migration reasons.
	PqUpstreams []*PreparedQueryUpstream `protobuf:"bytes,3,rep,name=pq_upstreams,json=pqUpstreams,proto3" json:"pq_upstreams,omitempty"`
}

func (x *Upstreams) Reset() {
	*x = Upstreams{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Upstreams) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Upstreams) ProtoMessage() {}

func (x *Upstreams) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Upstreams.ProtoReflect.Descriptor instead.
func (*Upstreams) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_upstreams_proto_rawDescGZIP(), []int{0}
}

func (x *Upstreams) GetWorkloads() *v1alpha1.WorkloadSelector {
	if x != nil {
		return x.Workloads
	}
	return nil
}

func (x *Upstreams) GetUpstreams() []*Upstream {
	if x != nil {
		return x.Upstreams
	}
	return nil
}

func (x *Upstreams) GetPqUpstreams() []*PreparedQueryUpstream {
	if x != nil {
		return x.PqUpstreams
	}
	return nil
}

type Upstream struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// destination_ref is the reference to an upstream service. This has to be pbcatalog.Service type.
	DestinationRef *pbresource.Reference `protobuf:"bytes,1,opt,name=destination_ref,json=destinationRef,proto3" json:"destination_ref,omitempty"`
	// destination_port is the port name of the upstream service. This should be the name
	// of the service's target port.
	DestinationPort string `protobuf:"bytes,2,opt,name=destination_port,json=destinationPort,proto3" json:"destination_port,omitempty"`
	// datacenter is the datacenter for where this upstream service lives.
	Datacenter string `protobuf:"bytes,3,opt,name=datacenter,proto3" json:"datacenter,omitempty"`
	// listen_addr is the address where Envoy will listen for requests to this upstream.
	// It can provided either as an ip:port or as a Unix domain socket.
	//
	// Types that are assignable to ListenAddr:
	//
	//	*Upstream_IpPort
	//	*Upstream_Unix
	ListenAddr isUpstream_ListenAddr `protobuf_oneof:"listen_addr"`
}

func (x *Upstream) Reset() {
	*x = Upstream{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Upstream) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Upstream) ProtoMessage() {}

func (x *Upstream) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Upstream.ProtoReflect.Descriptor instead.
func (*Upstream) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_upstreams_proto_rawDescGZIP(), []int{1}
}

func (x *Upstream) GetDestinationRef() *pbresource.Reference {
	if x != nil {
		return x.DestinationRef
	}
	return nil
}

func (x *Upstream) GetDestinationPort() string {
	if x != nil {
		return x.DestinationPort
	}
	return ""
}

func (x *Upstream) GetDatacenter() string {
	if x != nil {
		return x.Datacenter
	}
	return ""
}

func (m *Upstream) GetListenAddr() isUpstream_ListenAddr {
	if m != nil {
		return m.ListenAddr
	}
	return nil
}

func (x *Upstream) GetIpPort() *IPPortAddress {
	if x, ok := x.GetListenAddr().(*Upstream_IpPort); ok {
		return x.IpPort
	}
	return nil
}

func (x *Upstream) GetUnix() *UnixSocketAddress {
	if x, ok := x.GetListenAddr().(*Upstream_Unix); ok {
		return x.Unix
	}
	return nil
}

type isUpstream_ListenAddr interface {
	isUpstream_ListenAddr()
}

type Upstream_IpPort struct {
	IpPort *IPPortAddress `protobuf:"bytes,4,opt,name=ip_port,json=ipPort,proto3,oneof"`
}

type Upstream_Unix struct {
	Unix *UnixSocketAddress `protobuf:"bytes,5,opt,name=unix,proto3,oneof"`
}

func (*Upstream_IpPort) isUpstream_ListenAddr() {}

func (*Upstream_Unix) isUpstream_ListenAddr() {}

type IPPortAddress struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ip is an IPv4 or an IPv6 address.
	Ip string `protobuf:"bytes,1,opt,name=ip,proto3" json:"ip,omitempty"`
	// port is the port number.
	Port uint32 `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
}

func (x *IPPortAddress) Reset() {
	*x = IPPortAddress{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *IPPortAddress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IPPortAddress) ProtoMessage() {}

func (x *IPPortAddress) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IPPortAddress.ProtoReflect.Descriptor instead.
func (*IPPortAddress) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_upstreams_proto_rawDescGZIP(), []int{2}
}

func (x *IPPortAddress) GetIp() string {
	if x != nil {
		return x.Ip
	}
	return ""
}

func (x *IPPortAddress) GetPort() uint32 {
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
		mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnixSocketAddress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnixSocketAddress) ProtoMessage() {}

func (x *UnixSocketAddress) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[3]
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
	return file_pbmesh_v1alpha1_upstreams_proto_rawDescGZIP(), []int{3}
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

type PreparedQueryUpstream struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// name is the name of the prepared query to use as an upstream.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// datacenter is the datacenter for where this upstream service lives.
	Datacenter string `protobuf:"bytes,2,opt,name=datacenter,proto3" json:"datacenter,omitempty"`
	// listen_addr is the address where Envoy will listen for requests to this upstream.
	// It can provided either as an ip:port or as a Unix domain socket.
	//
	// Types that are assignable to ListenAddr:
	//
	//	*PreparedQueryUpstream_Tcp
	//	*PreparedQueryUpstream_Unix
	ListenAddr     isPreparedQueryUpstream_ListenAddr `protobuf_oneof:"listen_addr"`
	UpstreamConfig *UpstreamConfig                    `protobuf:"bytes,6,opt,name=upstream_config,json=upstreamConfig,proto3" json:"upstream_config,omitempty"`
}

func (x *PreparedQueryUpstream) Reset() {
	*x = PreparedQueryUpstream{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PreparedQueryUpstream) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PreparedQueryUpstream) ProtoMessage() {}

func (x *PreparedQueryUpstream) ProtoReflect() protoreflect.Message {
	mi := &file_pbmesh_v1alpha1_upstreams_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PreparedQueryUpstream.ProtoReflect.Descriptor instead.
func (*PreparedQueryUpstream) Descriptor() ([]byte, []int) {
	return file_pbmesh_v1alpha1_upstreams_proto_rawDescGZIP(), []int{4}
}

func (x *PreparedQueryUpstream) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *PreparedQueryUpstream) GetDatacenter() string {
	if x != nil {
		return x.Datacenter
	}
	return ""
}

func (m *PreparedQueryUpstream) GetListenAddr() isPreparedQueryUpstream_ListenAddr {
	if m != nil {
		return m.ListenAddr
	}
	return nil
}

func (x *PreparedQueryUpstream) GetTcp() *IPPortAddress {
	if x, ok := x.GetListenAddr().(*PreparedQueryUpstream_Tcp); ok {
		return x.Tcp
	}
	return nil
}

func (x *PreparedQueryUpstream) GetUnix() *UnixSocketAddress {
	if x, ok := x.GetListenAddr().(*PreparedQueryUpstream_Unix); ok {
		return x.Unix
	}
	return nil
}

func (x *PreparedQueryUpstream) GetUpstreamConfig() *UpstreamConfig {
	if x != nil {
		return x.UpstreamConfig
	}
	return nil
}

type isPreparedQueryUpstream_ListenAddr interface {
	isPreparedQueryUpstream_ListenAddr()
}

type PreparedQueryUpstream_Tcp struct {
	Tcp *IPPortAddress `protobuf:"bytes,4,opt,name=tcp,proto3,oneof"`
}

type PreparedQueryUpstream_Unix struct {
	Unix *UnixSocketAddress `protobuf:"bytes,5,opt,name=unix,proto3,oneof"`
}

func (*PreparedQueryUpstream_Tcp) isPreparedQueryUpstream_ListenAddr() {}

func (*PreparedQueryUpstream_Unix) isPreparedQueryUpstream_ListenAddr() {}

var File_pbmesh_v1alpha1_upstreams_proto protoreflect.FileDescriptor

var file_pbmesh_v1alpha1_upstreams_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x2f, 0x75, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x1e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x1a, 0x21, 0x70, 0x62, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2f, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x2d, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x75, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x73, 0x5f,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x19, 0x70, 0x62, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f,
	0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x80,
	0x02, 0x0a, 0x09, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x73, 0x12, 0x51, 0x0a, 0x09,
	0x77, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x33, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x2e, 0x63, 0x61, 0x74, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x2e, 0x57, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x53, 0x65, 0x6c, 0x65,
	0x63, 0x74, 0x6f, 0x72, 0x52, 0x09, 0x77, 0x6f, 0x72, 0x6b, 0x6c, 0x6f, 0x61, 0x64, 0x73, 0x12,
	0x46, 0x0a, 0x09, 0x75, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x28, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63,
	0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x2e, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x52, 0x09, 0x75, 0x70,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x73, 0x12, 0x58, 0x0a, 0x0c, 0x70, 0x71, 0x5f, 0x75, 0x70,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x35, 0x2e,
	0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c,
	0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x50,
	0x72, 0x65, 0x70, 0x61, 0x72, 0x65, 0x64, 0x51, 0x75, 0x65, 0x72, 0x79, 0x55, 0x70, 0x73, 0x74,
	0x72, 0x65, 0x61, 0x6d, 0x52, 0x0b, 0x70, 0x71, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d,
	0x73, 0x22, 0xc6, 0x02, 0x0a, 0x08, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x4d,
	0x0a, 0x0f, 0x64, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x72, 0x65,
	0x66, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x2e, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x0e, 0x64,
	0x65, 0x73, 0x74, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x66, 0x12, 0x29, 0x0a,
	0x10, 0x64, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x70, 0x6f, 0x72,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x64, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x64, 0x61, 0x74, 0x61,
	0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x64, 0x61,
	0x74, 0x61, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x12, 0x48, 0x0a, 0x07, 0x69, 0x70, 0x5f, 0x70,
	0x6f, 0x72, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x68, 0x61, 0x73, 0x68,
	0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73,
	0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x49, 0x50, 0x50, 0x6f, 0x72,
	0x74, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x48, 0x00, 0x52, 0x06, 0x69, 0x70, 0x50, 0x6f,
	0x72, 0x74, 0x12, 0x47, 0x0a, 0x04, 0x75, 0x6e, 0x69, 0x78, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x31, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e,
	0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x2e, 0x55, 0x6e, 0x69, 0x78, 0x53, 0x6f, 0x63, 0x6b, 0x65, 0x74, 0x41, 0x64, 0x64, 0x72,
	0x65, 0x73, 0x73, 0x48, 0x00, 0x52, 0x04, 0x75, 0x6e, 0x69, 0x78, 0x42, 0x0d, 0x0a, 0x0b, 0x6c,
	0x69, 0x73, 0x74, 0x65, 0x6e, 0x5f, 0x61, 0x64, 0x64, 0x72, 0x22, 0x33, 0x0a, 0x0d, 0x49, 0x50,
	0x50, 0x6f, 0x72, 0x74, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x70,
	0x6f, 0x72, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x22,
	0x3b, 0x0a, 0x11, 0x55, 0x6e, 0x69, 0x78, 0x53, 0x6f, 0x63, 0x6b, 0x65, 0x74, 0x41, 0x64, 0x64,
	0x72, 0x65, 0x73, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x12, 0x0a, 0x04, 0x6d, 0x6f, 0x64, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x22, 0xbf, 0x02, 0x0a,
	0x15, 0x50, 0x72, 0x65, 0x70, 0x61, 0x72, 0x65, 0x64, 0x51, 0x75, 0x65, 0x72, 0x79, 0x55, 0x70,
	0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x64, 0x61,
	0x74, 0x61, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a,
	0x64, 0x61, 0x74, 0x61, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x12, 0x41, 0x0a, 0x03, 0x74, 0x63,
	0x70, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63,
	0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x49, 0x50, 0x50, 0x6f, 0x72, 0x74, 0x41,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x48, 0x00, 0x52, 0x03, 0x74, 0x63, 0x70, 0x12, 0x47, 0x0a,
	0x04, 0x75, 0x6e, 0x69, 0x78, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x31, 0x2e, 0x68, 0x61,
	0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d,
	0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x55, 0x6e, 0x69,
	0x78, 0x53, 0x6f, 0x63, 0x6b, 0x65, 0x74, 0x41, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x48, 0x00,
	0x52, 0x04, 0x75, 0x6e, 0x69, 0x78, 0x12, 0x57, 0x0a, 0x0f, 0x75, 0x70, 0x73, 0x74, 0x72, 0x65,
	0x61, 0x6d, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x2e, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2e, 0x63, 0x6f, 0x6e, 0x73,
	0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x2e, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52,
	0x0e, 0x75, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x42,
	0x0d, 0x0a, 0x0b, 0x6c, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x5f, 0x61, 0x64, 0x64, 0x72, 0x42, 0x96,
	0x02, 0x0a, 0x22, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2e, 0x63, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x6d, 0x65, 0x73, 0x68, 0x2e, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x42, 0x0e, 0x55, 0x70, 0x73, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x73,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x45, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x2f, 0x63, 0x6f,
	0x6e, 0x73, 0x75, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2d, 0x70, 0x75, 0x62, 0x6c, 0x69,
	0x63, 0x2f, 0x70, 0x62, 0x6d, 0x65, 0x73, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61,
	0x31, 0x3b, 0x6d, 0x65, 0x73, 0x68, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xa2, 0x02,
	0x03, 0x48, 0x43, 0x4d, 0xaa, 0x02, 0x1e, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70,
	0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x2e, 0x4d, 0x65, 0x73, 0x68, 0x2e, 0x56, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0xca, 0x02, 0x1e, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72,
	0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0xe2, 0x02, 0x2a, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f,
	0x72, 0x70, 0x5c, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x5c, 0x4d, 0x65, 0x73, 0x68, 0x5c, 0x56,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0xea, 0x02, 0x21, 0x48, 0x61, 0x73, 0x68, 0x69, 0x63, 0x6f, 0x72, 0x70, 0x3a,
	0x3a, 0x43, 0x6f, 0x6e, 0x73, 0x75, 0x6c, 0x3a, 0x3a, 0x4d, 0x65, 0x73, 0x68, 0x3a, 0x3a, 0x56,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pbmesh_v1alpha1_upstreams_proto_rawDescOnce sync.Once
	file_pbmesh_v1alpha1_upstreams_proto_rawDescData = file_pbmesh_v1alpha1_upstreams_proto_rawDesc
)

func file_pbmesh_v1alpha1_upstreams_proto_rawDescGZIP() []byte {
	file_pbmesh_v1alpha1_upstreams_proto_rawDescOnce.Do(func() {
		file_pbmesh_v1alpha1_upstreams_proto_rawDescData = protoimpl.X.CompressGZIP(file_pbmesh_v1alpha1_upstreams_proto_rawDescData)
	})
	return file_pbmesh_v1alpha1_upstreams_proto_rawDescData
}

var file_pbmesh_v1alpha1_upstreams_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_pbmesh_v1alpha1_upstreams_proto_goTypes = []interface{}{
	(*Upstreams)(nil),                 // 0: hashicorp.consul.mesh.v1alpha1.Upstreams
	(*Upstream)(nil),                  // 1: hashicorp.consul.mesh.v1alpha1.Upstream
	(*IPPortAddress)(nil),             // 2: hashicorp.consul.mesh.v1alpha1.IPPortAddress
	(*UnixSocketAddress)(nil),         // 3: hashicorp.consul.mesh.v1alpha1.UnixSocketAddress
	(*PreparedQueryUpstream)(nil),     // 4: hashicorp.consul.mesh.v1alpha1.PreparedQueryUpstream
	(*v1alpha1.WorkloadSelector)(nil), // 5: hashicorp.consul.catalog.v1alpha1.WorkloadSelector
	(*pbresource.Reference)(nil),      // 6: hashicorp.consul.resource.Reference
	(*UpstreamConfig)(nil),            // 7: hashicorp.consul.mesh.v1alpha1.UpstreamConfig
}
var file_pbmesh_v1alpha1_upstreams_proto_depIdxs = []int32{
	5, // 0: hashicorp.consul.mesh.v1alpha1.Upstreams.workloads:type_name -> hashicorp.consul.catalog.v1alpha1.WorkloadSelector
	1, // 1: hashicorp.consul.mesh.v1alpha1.Upstreams.upstreams:type_name -> hashicorp.consul.mesh.v1alpha1.Upstream
	4, // 2: hashicorp.consul.mesh.v1alpha1.Upstreams.pq_upstreams:type_name -> hashicorp.consul.mesh.v1alpha1.PreparedQueryUpstream
	6, // 3: hashicorp.consul.mesh.v1alpha1.Upstream.destination_ref:type_name -> hashicorp.consul.resource.Reference
	2, // 4: hashicorp.consul.mesh.v1alpha1.Upstream.ip_port:type_name -> hashicorp.consul.mesh.v1alpha1.IPPortAddress
	3, // 5: hashicorp.consul.mesh.v1alpha1.Upstream.unix:type_name -> hashicorp.consul.mesh.v1alpha1.UnixSocketAddress
	2, // 6: hashicorp.consul.mesh.v1alpha1.PreparedQueryUpstream.tcp:type_name -> hashicorp.consul.mesh.v1alpha1.IPPortAddress
	3, // 7: hashicorp.consul.mesh.v1alpha1.PreparedQueryUpstream.unix:type_name -> hashicorp.consul.mesh.v1alpha1.UnixSocketAddress
	7, // 8: hashicorp.consul.mesh.v1alpha1.PreparedQueryUpstream.upstream_config:type_name -> hashicorp.consul.mesh.v1alpha1.UpstreamConfig
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_pbmesh_v1alpha1_upstreams_proto_init() }
func file_pbmesh_v1alpha1_upstreams_proto_init() {
	if File_pbmesh_v1alpha1_upstreams_proto != nil {
		return
	}
	file_pbmesh_v1alpha1_upstreams_configuration_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_pbmesh_v1alpha1_upstreams_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Upstreams); i {
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
		file_pbmesh_v1alpha1_upstreams_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Upstream); i {
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
		file_pbmesh_v1alpha1_upstreams_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*IPPortAddress); i {
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
		file_pbmesh_v1alpha1_upstreams_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
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
		file_pbmesh_v1alpha1_upstreams_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PreparedQueryUpstream); i {
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
	file_pbmesh_v1alpha1_upstreams_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*Upstream_IpPort)(nil),
		(*Upstream_Unix)(nil),
	}
	file_pbmesh_v1alpha1_upstreams_proto_msgTypes[4].OneofWrappers = []interface{}{
		(*PreparedQueryUpstream_Tcp)(nil),
		(*PreparedQueryUpstream_Unix)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pbmesh_v1alpha1_upstreams_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pbmesh_v1alpha1_upstreams_proto_goTypes,
		DependencyIndexes: file_pbmesh_v1alpha1_upstreams_proto_depIdxs,
		MessageInfos:      file_pbmesh_v1alpha1_upstreams_proto_msgTypes,
	}.Build()
	File_pbmesh_v1alpha1_upstreams_proto = out.File
	file_pbmesh_v1alpha1_upstreams_proto_rawDesc = nil
	file_pbmesh_v1alpha1_upstreams_proto_goTypes = nil
	file_pbmesh_v1alpha1_upstreams_proto_depIdxs = nil
}

// Code generated by protoc-gen-go. DO NOT EDIT.
// source: envoy/service/event_reporting/v2alpha/event_reporting_service.proto

package envoy_service_event_reporting_v2alpha

import (
	context "context"
	fmt "fmt"
	_ "github.com/cncf/udpa/go/udpa/annotations"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	proto "github.com/golang/protobuf/proto"
	any "github.com/golang/protobuf/ptypes/any"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type StreamEventsRequest struct {
	Identifier           *StreamEventsRequest_Identifier `protobuf:"bytes,1,opt,name=identifier,proto3" json:"identifier,omitempty"`
	Events               []*any.Any                      `protobuf:"bytes,2,rep,name=events,proto3" json:"events,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                        `json:"-"`
	XXX_unrecognized     []byte                          `json:"-"`
	XXX_sizecache        int32                           `json:"-"`
}

func (m *StreamEventsRequest) Reset()         { *m = StreamEventsRequest{} }
func (m *StreamEventsRequest) String() string { return proto.CompactTextString(m) }
func (*StreamEventsRequest) ProtoMessage()    {}
func (*StreamEventsRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_1a37fe38e985d5f4, []int{0}
}

func (m *StreamEventsRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StreamEventsRequest.Unmarshal(m, b)
}
func (m *StreamEventsRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StreamEventsRequest.Marshal(b, m, deterministic)
}
func (m *StreamEventsRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StreamEventsRequest.Merge(m, src)
}
func (m *StreamEventsRequest) XXX_Size() int {
	return xxx_messageInfo_StreamEventsRequest.Size(m)
}
func (m *StreamEventsRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_StreamEventsRequest.DiscardUnknown(m)
}

var xxx_messageInfo_StreamEventsRequest proto.InternalMessageInfo

func (m *StreamEventsRequest) GetIdentifier() *StreamEventsRequest_Identifier {
	if m != nil {
		return m.Identifier
	}
	return nil
}

func (m *StreamEventsRequest) GetEvents() []*any.Any {
	if m != nil {
		return m.Events
	}
	return nil
}

type StreamEventsRequest_Identifier struct {
	Node                 *core.Node `protobuf:"bytes,1,opt,name=node,proto3" json:"node,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *StreamEventsRequest_Identifier) Reset()         { *m = StreamEventsRequest_Identifier{} }
func (m *StreamEventsRequest_Identifier) String() string { return proto.CompactTextString(m) }
func (*StreamEventsRequest_Identifier) ProtoMessage()    {}
func (*StreamEventsRequest_Identifier) Descriptor() ([]byte, []int) {
	return fileDescriptor_1a37fe38e985d5f4, []int{0, 0}
}

func (m *StreamEventsRequest_Identifier) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StreamEventsRequest_Identifier.Unmarshal(m, b)
}
func (m *StreamEventsRequest_Identifier) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StreamEventsRequest_Identifier.Marshal(b, m, deterministic)
}
func (m *StreamEventsRequest_Identifier) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StreamEventsRequest_Identifier.Merge(m, src)
}
func (m *StreamEventsRequest_Identifier) XXX_Size() int {
	return xxx_messageInfo_StreamEventsRequest_Identifier.Size(m)
}
func (m *StreamEventsRequest_Identifier) XXX_DiscardUnknown() {
	xxx_messageInfo_StreamEventsRequest_Identifier.DiscardUnknown(m)
}

var xxx_messageInfo_StreamEventsRequest_Identifier proto.InternalMessageInfo

func (m *StreamEventsRequest_Identifier) GetNode() *core.Node {
	if m != nil {
		return m.Node
	}
	return nil
}

type StreamEventsResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StreamEventsResponse) Reset()         { *m = StreamEventsResponse{} }
func (m *StreamEventsResponse) String() string { return proto.CompactTextString(m) }
func (*StreamEventsResponse) ProtoMessage()    {}
func (*StreamEventsResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_1a37fe38e985d5f4, []int{1}
}

func (m *StreamEventsResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StreamEventsResponse.Unmarshal(m, b)
}
func (m *StreamEventsResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StreamEventsResponse.Marshal(b, m, deterministic)
}
func (m *StreamEventsResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StreamEventsResponse.Merge(m, src)
}
func (m *StreamEventsResponse) XXX_Size() int {
	return xxx_messageInfo_StreamEventsResponse.Size(m)
}
func (m *StreamEventsResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_StreamEventsResponse.DiscardUnknown(m)
}

var xxx_messageInfo_StreamEventsResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*StreamEventsRequest)(nil), "envoy.service.event_reporting.v2alpha.StreamEventsRequest")
	proto.RegisterType((*StreamEventsRequest_Identifier)(nil), "envoy.service.event_reporting.v2alpha.StreamEventsRequest.Identifier")
	proto.RegisterType((*StreamEventsResponse)(nil), "envoy.service.event_reporting.v2alpha.StreamEventsResponse")
}

func init() {
	proto.RegisterFile("envoy/service/event_reporting/v2alpha/event_reporting_service.proto", fileDescriptor_1a37fe38e985d5f4)
}

var fileDescriptor_1a37fe38e985d5f4 = []byte{
	// 413 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x91, 0x41, 0x8b, 0xd3, 0x40,
	0x14, 0xc7, 0x7d, 0xd1, 0x2d, 0xcb, 0xac, 0x87, 0x25, 0xae, 0xee, 0x1a, 0x54, 0x4a, 0x40, 0xe8,
	0x69, 0x46, 0x52, 0xf4, 0xa0, 0x27, 0xb3, 0xec, 0xc1, 0x8b, 0x94, 0xf4, 0x03, 0x94, 0x69, 0xf3,
	0x1a, 0x07, 0xda, 0x99, 0x71, 0x66, 0x12, 0xcc, 0xcd, 0x93, 0x88, 0x20, 0x82, 0x27, 0xbf, 0x81,
	0xdf, 0xc1, 0x4f, 0xe0, 0xd5, 0xaf, 0xe2, 0x4d, 0x0f, 0x22, 0x4d, 0x26, 0x5a, 0x6b, 0x61, 0x4b,
	0x6f, 0xc9, 0x7b, 0xff, 0xf7, 0x9b, 0xff, 0xfb, 0x3f, 0x72, 0x8e, 0xb2, 0x52, 0x35, 0xb3, 0x68,
	0x2a, 0x31, 0x43, 0x86, 0x15, 0x4a, 0x37, 0x31, 0xa8, 0x95, 0x71, 0x42, 0x16, 0xac, 0x4a, 0xf8,
	0x42, 0xbf, 0xe0, 0x9b, 0xf5, 0x89, 0xd7, 0x53, 0x6d, 0x94, 0x53, 0xe1, 0xfd, 0x06, 0x42, 0xbb,
	0xe2, 0x86, 0x98, 0x7a, 0x48, 0x74, 0xa7, 0x7d, 0x8b, 0x6b, 0xc1, 0xaa, 0x84, 0xcd, 0x94, 0x41,
	0x36, 0xe5, 0xd6, 0x43, 0xa2, 0xdb, 0x85, 0x52, 0xc5, 0x02, 0x59, 0xf3, 0x37, 0x2d, 0xe7, 0x8c,
	0xcb, 0xda, 0xb7, 0xee, 0x95, 0xb9, 0xe6, 0x8c, 0x4b, 0xa9, 0x1c, 0x77, 0x42, 0x49, 0xcb, 0x96,
	0xa2, 0x30, 0xdc, 0x75, 0xa3, 0x77, 0xff, 0xeb, 0x5b, 0xc7, 0x5d, 0x69, 0x7d, 0xfb, 0xb4, 0xe2,
	0x0b, 0x91, 0x73, 0x87, 0xac, 0xfb, 0x68, 0x1b, 0xf1, 0x0f, 0x20, 0x37, 0xc6, 0xce, 0x20, 0x5f,
	0x5e, 0xac, 0x2c, 0xdb, 0x0c, 0x5f, 0x96, 0x68, 0x5d, 0x88, 0x84, 0x88, 0x1c, 0xa5, 0x13, 0x73,
	0x81, 0xe6, 0x0c, 0xfa, 0x30, 0x38, 0x4a, 0x2e, 0xe8, 0x4e, 0x4b, 0xd2, 0x2d, 0x3c, 0xfa, 0xec,
	0x0f, 0x2c, 0x5b, 0x03, 0x87, 0x8f, 0x48, 0xaf, 0xa1, 0xd8, 0xb3, 0xa0, 0x7f, 0x75, 0x70, 0x94,
	0x9c, 0xd0, 0x36, 0x02, 0xda, 0x45, 0x40, 0x9f, 0xca, 0x3a, 0x3d, 0xfc, 0x99, 0x1e, 0x7c, 0x84,
	0xe0, 0x10, 0x32, 0xaf, 0x8e, 0xce, 0x09, 0xf9, 0x4b, 0x0c, 0x1f, 0x92, 0x6b, 0x52, 0xe5, 0xe8,
	0x6d, 0x9e, 0x7a, 0x9b, 0x5c, 0x0b, 0x5a, 0x25, 0x74, 0x15, 0x32, 0x7d, 0xae, 0x72, 0x6c, 0x30,
	0xef, 0x20, 0x38, 0x86, 0xac, 0x91, 0xc7, 0xb7, 0xc8, 0xc9, 0xbf, 0x56, 0xad, 0x56, 0xd2, 0x62,
	0xf2, 0x19, 0xc8, 0xcd, 0xa6, 0x94, 0x75, 0xab, 0x8d, 0xdb, 0x8d, 0xc3, 0xf7, 0x40, 0xae, 0xaf,
	0x8f, 0x84, 0x8f, 0xf7, 0x8f, 0x24, 0x7a, 0xb2, 0xd7, 0x6c, 0xeb, 0x31, 0xbe, 0x32, 0x80, 0x07,
	0x90, 0xbe, 0x81, 0xef, 0x9f, 0x7e, 0x7d, 0x38, 0x88, 0xc3, 0xfe, 0x25, 0xa8, 0xe1, 0x97, 0xd7,
	0x5f, 0xbf, 0xf5, 0x82, 0x63, 0x20, 0x43, 0xa1, 0xda, 0x77, 0xb5, 0x51, 0xaf, 0xea, 0xdd, 0x2c,
	0xa4, 0xd1, 0xd6, 0x38, 0x46, 0xab, 0x1b, 0x8d, 0xe0, 0x2d, 0xc0, 0xb4, 0xd7, 0xdc, 0x6b, 0xf8,
	0x3b, 0x00, 0x00, 0xff, 0xff, 0x7d, 0xad, 0x79, 0x2d, 0x4c, 0x03, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// EventReportingServiceClient is the client API for EventReportingService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type EventReportingServiceClient interface {
	StreamEvents(ctx context.Context, opts ...grpc.CallOption) (EventReportingService_StreamEventsClient, error)
}

type eventReportingServiceClient struct {
	cc *grpc.ClientConn
}

func NewEventReportingServiceClient(cc *grpc.ClientConn) EventReportingServiceClient {
	return &eventReportingServiceClient{cc}
}

func (c *eventReportingServiceClient) StreamEvents(ctx context.Context, opts ...grpc.CallOption) (EventReportingService_StreamEventsClient, error) {
	stream, err := c.cc.NewStream(ctx, &_EventReportingService_serviceDesc.Streams[0], "/envoy.service.event_reporting.v2alpha.EventReportingService/StreamEvents", opts...)
	if err != nil {
		return nil, err
	}
	x := &eventReportingServiceStreamEventsClient{stream}
	return x, nil
}

type EventReportingService_StreamEventsClient interface {
	Send(*StreamEventsRequest) error
	Recv() (*StreamEventsResponse, error)
	grpc.ClientStream
}

type eventReportingServiceStreamEventsClient struct {
	grpc.ClientStream
}

func (x *eventReportingServiceStreamEventsClient) Send(m *StreamEventsRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *eventReportingServiceStreamEventsClient) Recv() (*StreamEventsResponse, error) {
	m := new(StreamEventsResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// EventReportingServiceServer is the server API for EventReportingService service.
type EventReportingServiceServer interface {
	StreamEvents(EventReportingService_StreamEventsServer) error
}

// UnimplementedEventReportingServiceServer can be embedded to have forward compatible implementations.
type UnimplementedEventReportingServiceServer struct {
}

func (*UnimplementedEventReportingServiceServer) StreamEvents(srv EventReportingService_StreamEventsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamEvents not implemented")
}

func RegisterEventReportingServiceServer(s *grpc.Server, srv EventReportingServiceServer) {
	s.RegisterService(&_EventReportingService_serviceDesc, srv)
}

func _EventReportingService_StreamEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(EventReportingServiceServer).StreamEvents(&eventReportingServiceStreamEventsServer{stream})
}

type EventReportingService_StreamEventsServer interface {
	Send(*StreamEventsResponse) error
	Recv() (*StreamEventsRequest, error)
	grpc.ServerStream
}

type eventReportingServiceStreamEventsServer struct {
	grpc.ServerStream
}

func (x *eventReportingServiceStreamEventsServer) Send(m *StreamEventsResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *eventReportingServiceStreamEventsServer) Recv() (*StreamEventsRequest, error) {
	m := new(StreamEventsRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _EventReportingService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "envoy.service.event_reporting.v2alpha.EventReportingService",
	HandlerType: (*EventReportingServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamEvents",
			Handler:       _EventReportingService_StreamEvents_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "envoy/service/event_reporting/v2alpha/event_reporting_service.proto",
}

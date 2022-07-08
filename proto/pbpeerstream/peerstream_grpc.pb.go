// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: proto/pbpeerstream/peerstream.proto

package pbpeerstream

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// PeerStreamServiceClient is the client API for PeerStreamService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PeerStreamServiceClient interface {
	// StreamResources opens an event stream for resources to share between peers, such as services.
	// Events are streamed as they happen.
	// buf:lint:ignore RPC_REQUEST_STANDARD_NAME
	// buf:lint:ignore RPC_RESPONSE_STANDARD_NAME
	// buf:lint:ignore RPC_REQUEST_RESPONSE_UNIQUE
	StreamResources(ctx context.Context, opts ...grpc.CallOption) (PeerStreamService_StreamResourcesClient, error)
}

type peerStreamServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPeerStreamServiceClient(cc grpc.ClientConnInterface) PeerStreamServiceClient {
	return &peerStreamServiceClient{cc}
}

func (c *peerStreamServiceClient) StreamResources(ctx context.Context, opts ...grpc.CallOption) (PeerStreamService_StreamResourcesClient, error) {
	stream, err := c.cc.NewStream(ctx, &PeerStreamService_ServiceDesc.Streams[0], "/peerstream.PeerStreamService/StreamResources", opts...)
	if err != nil {
		return nil, err
	}
	x := &peerStreamServiceStreamResourcesClient{stream}
	return x, nil
}

type PeerStreamService_StreamResourcesClient interface {
	Send(*ReplicationMessage) error
	Recv() (*ReplicationMessage, error)
	grpc.ClientStream
}

type peerStreamServiceStreamResourcesClient struct {
	grpc.ClientStream
}

func (x *peerStreamServiceStreamResourcesClient) Send(m *ReplicationMessage) error {
	return x.ClientStream.SendMsg(m)
}

func (x *peerStreamServiceStreamResourcesClient) Recv() (*ReplicationMessage, error) {
	m := new(ReplicationMessage)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// PeerStreamServiceServer is the server API for PeerStreamService service.
// All implementations should embed UnimplementedPeerStreamServiceServer
// for forward compatibility
type PeerStreamServiceServer interface {
	// StreamResources opens an event stream for resources to share between peers, such as services.
	// Events are streamed as they happen.
	// buf:lint:ignore RPC_REQUEST_STANDARD_NAME
	// buf:lint:ignore RPC_RESPONSE_STANDARD_NAME
	// buf:lint:ignore RPC_REQUEST_RESPONSE_UNIQUE
	StreamResources(PeerStreamService_StreamResourcesServer) error
}

// UnimplementedPeerStreamServiceServer should be embedded to have forward compatible implementations.
type UnimplementedPeerStreamServiceServer struct {
}

func (UnimplementedPeerStreamServiceServer) StreamResources(PeerStreamService_StreamResourcesServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamResources not implemented")
}

// UnsafePeerStreamServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PeerStreamServiceServer will
// result in compilation errors.
type UnsafePeerStreamServiceServer interface {
	mustEmbedUnimplementedPeerStreamServiceServer()
}

func RegisterPeerStreamServiceServer(s grpc.ServiceRegistrar, srv PeerStreamServiceServer) {
	s.RegisterService(&PeerStreamService_ServiceDesc, srv)
}

func _PeerStreamService_StreamResources_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(PeerStreamServiceServer).StreamResources(&peerStreamServiceStreamResourcesServer{stream})
}

type PeerStreamService_StreamResourcesServer interface {
	Send(*ReplicationMessage) error
	Recv() (*ReplicationMessage, error)
	grpc.ServerStream
}

type peerStreamServiceStreamResourcesServer struct {
	grpc.ServerStream
}

func (x *peerStreamServiceStreamResourcesServer) Send(m *ReplicationMessage) error {
	return x.ServerStream.SendMsg(m)
}

func (x *peerStreamServiceStreamResourcesServer) Recv() (*ReplicationMessage, error) {
	m := new(ReplicationMessage)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// PeerStreamService_ServiceDesc is the grpc.ServiceDesc for PeerStreamService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PeerStreamService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "peerstream.PeerStreamService",
	HandlerType: (*PeerStreamServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamResources",
			Handler:       _PeerStreamService_StreamResources_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "proto/pbpeerstream/peerstream.proto",
}

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: proto-public/pbdataplane/dataplane.proto

package pbdataplane

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

// DataplaneServiceClient is the client API for DataplaneService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type DataplaneServiceClient interface {
	GetSupportedDataplaneFeatures(ctx context.Context, in *GetSupportedDataplaneFeaturesRequest, opts ...grpc.CallOption) (*GetSupportedDataplaneFeaturesResponse, error)
	GetEnvoyBootstrapParams(ctx context.Context, in *GetEnvoyBootstrapParamsRequest, opts ...grpc.CallOption) (*GetEnvoyBootstrapParamsResponse, error)
}

type dataplaneServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDataplaneServiceClient(cc grpc.ClientConnInterface) DataplaneServiceClient {
	return &dataplaneServiceClient{cc}
}

func (c *dataplaneServiceClient) GetSupportedDataplaneFeatures(ctx context.Context, in *GetSupportedDataplaneFeaturesRequest, opts ...grpc.CallOption) (*GetSupportedDataplaneFeaturesResponse, error) {
	out := new(GetSupportedDataplaneFeaturesResponse)
	err := c.cc.Invoke(ctx, "/dataplane.DataplaneService/GetSupportedDataplaneFeatures", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *dataplaneServiceClient) GetEnvoyBootstrapParams(ctx context.Context, in *GetEnvoyBootstrapParamsRequest, opts ...grpc.CallOption) (*GetEnvoyBootstrapParamsResponse, error) {
	out := new(GetEnvoyBootstrapParamsResponse)
	err := c.cc.Invoke(ctx, "/dataplane.DataplaneService/GetEnvoyBootstrapParams", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DataplaneServiceServer is the server API for DataplaneService service.
// All implementations should embed UnimplementedDataplaneServiceServer
// for forward compatibility
type DataplaneServiceServer interface {
	GetSupportedDataplaneFeatures(context.Context, *GetSupportedDataplaneFeaturesRequest) (*GetSupportedDataplaneFeaturesResponse, error)
	GetEnvoyBootstrapParams(context.Context, *GetEnvoyBootstrapParamsRequest) (*GetEnvoyBootstrapParamsResponse, error)
}

// UnimplementedDataplaneServiceServer should be embedded to have forward compatible implementations.
type UnimplementedDataplaneServiceServer struct {
}

func (UnimplementedDataplaneServiceServer) GetSupportedDataplaneFeatures(context.Context, *GetSupportedDataplaneFeaturesRequest) (*GetSupportedDataplaneFeaturesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSupportedDataplaneFeatures not implemented")
}
func (UnimplementedDataplaneServiceServer) GetEnvoyBootstrapParams(context.Context, *GetEnvoyBootstrapParamsRequest) (*GetEnvoyBootstrapParamsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetEnvoyBootstrapParams not implemented")
}

// UnsafeDataplaneServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to DataplaneServiceServer will
// result in compilation errors.
type UnsafeDataplaneServiceServer interface {
	mustEmbedUnimplementedDataplaneServiceServer()
}

func RegisterDataplaneServiceServer(s grpc.ServiceRegistrar, srv DataplaneServiceServer) {
	s.RegisterService(&DataplaneService_ServiceDesc, srv)
}

func _DataplaneService_GetSupportedDataplaneFeatures_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSupportedDataplaneFeaturesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataplaneServiceServer).GetSupportedDataplaneFeatures(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/dataplane.DataplaneService/GetSupportedDataplaneFeatures",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataplaneServiceServer).GetSupportedDataplaneFeatures(ctx, req.(*GetSupportedDataplaneFeaturesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DataplaneService_GetEnvoyBootstrapParams_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetEnvoyBootstrapParamsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DataplaneServiceServer).GetEnvoyBootstrapParams(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/dataplane.DataplaneService/GetEnvoyBootstrapParams",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DataplaneServiceServer).GetEnvoyBootstrapParams(ctx, req.(*GetEnvoyBootstrapParamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// DataplaneService_ServiceDesc is the grpc.ServiceDesc for DataplaneService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var DataplaneService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "dataplane.DataplaneService",
	HandlerType: (*DataplaneServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetSupportedDataplaneFeatures",
			Handler:    _DataplaneService_GetSupportedDataplaneFeatures_Handler,
		},
		{
			MethodName: "GetEnvoyBootstrapParams",
			Handler:    _DataplaneService_GetEnvoyBootstrapParams_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto-public/pbdataplane/dataplane.proto",
}

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: proto-public/pbacl/acl.proto

package pbacl

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

// ACLServiceClient is the client API for ACLService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ACLServiceClient interface {
	// Login exchanges the presented bearer token for a Consul ACL token using a
	// configured auth method.
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error)
	// Logout destroys the given ACL token once the caller is done with it.
	Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutResponse, error)
}

type aCLServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewACLServiceClient(cc grpc.ClientConnInterface) ACLServiceClient {
	return &aCLServiceClient{cc}
}

func (c *aCLServiceClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error) {
	out := new(LoginResponse)
	err := c.cc.Invoke(ctx, "/hashicorp.consul.acl.ACLService/Login", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *aCLServiceClient) Logout(ctx context.Context, in *LogoutRequest, opts ...grpc.CallOption) (*LogoutResponse, error) {
	out := new(LogoutResponse)
	err := c.cc.Invoke(ctx, "/hashicorp.consul.acl.ACLService/Logout", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ACLServiceServer is the server API for ACLService service.
// All implementations should embed UnimplementedACLServiceServer
// for forward compatibility
type ACLServiceServer interface {
	// Login exchanges the presented bearer token for a Consul ACL token using a
	// configured auth method.
	Login(context.Context, *LoginRequest) (*LoginResponse, error)
	// Logout destroys the given ACL token once the caller is done with it.
	Logout(context.Context, *LogoutRequest) (*LogoutResponse, error)
}

// UnimplementedACLServiceServer should be embedded to have forward compatible implementations.
type UnimplementedACLServiceServer struct {
}

func (UnimplementedACLServiceServer) Login(context.Context, *LoginRequest) (*LoginResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Login not implemented")
}
func (UnimplementedACLServiceServer) Logout(context.Context, *LogoutRequest) (*LogoutResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Logout not implemented")
}

// UnsafeACLServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ACLServiceServer will
// result in compilation errors.
type UnsafeACLServiceServer interface {
	mustEmbedUnimplementedACLServiceServer()
}

func RegisterACLServiceServer(s grpc.ServiceRegistrar, srv ACLServiceServer) {
	s.RegisterService(&ACLService_ServiceDesc, srv)
}

func _ACLService_Login_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ACLServiceServer).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/hashicorp.consul.acl.ACLService/Login",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ACLServiceServer).Login(ctx, req.(*LoginRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ACLService_Logout_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogoutRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ACLServiceServer).Logout(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/hashicorp.consul.acl.ACLService/Logout",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ACLServiceServer).Logout(ctx, req.(*LogoutRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ACLService_ServiceDesc is the grpc.ServiceDesc for ACLService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ACLService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "hashicorp.consul.acl.ACLService",
	HandlerType: (*ACLServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Login",
			Handler:    _ACLService_Login_Handler,
		},
		{
			MethodName: "Logout",
			Handler:    _ACLService_Logout_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto-public/pbacl/acl.proto",
}

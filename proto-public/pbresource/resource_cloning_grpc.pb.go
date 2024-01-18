// Code generated by protoc-gen-grpc-inmem. DO NOT EDIT.

package pbresource

import (
	"context"

	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// compile-time check to ensure that the generator is implementing all
// of the grpc client interfaces methods.
var _ ResourceServiceClient = CloningResourceServiceClient{}

// IsCloningResourceServiceClient is an interface that can be used to detect
// that a ResourceServiceClient is using the in-memory transport and has already
// been wrapped with a with a CloningResourceServiceClient.
type IsCloningResourceServiceClient interface {
	IsCloningResourceServiceClient() bool
}

// CloningResourceServiceClient implements the ResourceServiceClient interface by wrapping
// another implementation and copying all protobuf messages that pass through the client.
// This is mainly useful to wrap the an in-process client to insulate users of that
// client from having to care about potential immutability of data they receive or having
// the server implementation mutate their internal memory.
type CloningResourceServiceClient struct {
	ResourceServiceClient
}

func NewCloningResourceServiceClient(client ResourceServiceClient) ResourceServiceClient {
	if cloner, ok := client.(IsCloningResourceServiceClient); ok && cloner.IsCloningResourceServiceClient() {
		// prevent a double clone if the underlying client is already the cloning client.
		return client
	}

	return CloningResourceServiceClient{
		ResourceServiceClient: client,
	}
}

// IsCloningResourceServiceClient implements the IsCloningResourceServiceClient interface. This
// is only used to detect wrapped clients that would be double cloning data and prevent that.
func (c CloningResourceServiceClient) IsCloningResourceServiceClient() bool {
	return true
}

func (c CloningResourceServiceClient) Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*ReadResponse, error) {
	in = proto.Clone(in).(*ReadRequest)

	out, err := c.ResourceServiceClient.Read(ctx, in)
	if err != nil {
		return nil, err
	}

	return proto.Clone(out).(*ReadResponse), nil
}

func (c CloningResourceServiceClient) Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteResponse, error) {
	in = proto.Clone(in).(*WriteRequest)

	out, err := c.ResourceServiceClient.Write(ctx, in)
	if err != nil {
		return nil, err
	}

	return proto.Clone(out).(*WriteResponse), nil
}

func (c CloningResourceServiceClient) WriteStatus(ctx context.Context, in *WriteStatusRequest, opts ...grpc.CallOption) (*WriteStatusResponse, error) {
	in = proto.Clone(in).(*WriteStatusRequest)

	out, err := c.ResourceServiceClient.WriteStatus(ctx, in)
	if err != nil {
		return nil, err
	}

	return proto.Clone(out).(*WriteStatusResponse), nil
}

func (c CloningResourceServiceClient) List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListResponse, error) {
	in = proto.Clone(in).(*ListRequest)

	out, err := c.ResourceServiceClient.List(ctx, in)
	if err != nil {
		return nil, err
	}

	return proto.Clone(out).(*ListResponse), nil
}

func (c CloningResourceServiceClient) ListByOwner(ctx context.Context, in *ListByOwnerRequest, opts ...grpc.CallOption) (*ListByOwnerResponse, error) {
	in = proto.Clone(in).(*ListByOwnerRequest)

	out, err := c.ResourceServiceClient.ListByOwner(ctx, in)
	if err != nil {
		return nil, err
	}

	return proto.Clone(out).(*ListByOwnerResponse), nil
}

func (c CloningResourceServiceClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error) {
	in = proto.Clone(in).(*DeleteRequest)

	out, err := c.ResourceServiceClient.Delete(ctx, in)
	if err != nil {
		return nil, err
	}

	return proto.Clone(out).(*DeleteResponse), nil
}

func (c CloningResourceServiceClient) WatchList(ctx context.Context, in *WatchListRequest, opts ...grpc.CallOption) (ResourceService_WatchListClient, error) {
	in = proto.Clone(in).(*WatchListRequest)

	st, err := c.ResourceServiceClient.WatchList(ctx, in)
	if err != nil {
		return nil, err
	}

	return newCloningStream[*WatchEvent](st), nil
}

// Code generated by protoc-gen-grpc-inmem. DO NOT EDIT.

package pbconnectca

import (
	"context"

	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// compile-time check to ensure that the generator is implementing all
// of the grpc client interfaces methods.
var _ ConnectCAServiceClient = CloningConnectCAServiceClient{}

// IsCloningConnectCAServiceClient is an interface that can be used to detect
// that a ConnectCAServiceClient is using the in-memory transport and has already
// been wrapped with a with a CloningConnectCAServiceClient.
type IsCloningConnectCAServiceClient interface {
	IsCloningConnectCAServiceClient() bool
}

// CloningConnectCAServiceClient implements the ConnectCAServiceClient interface by wrapping
// another implementation and copying all protobuf messages that pass through the client.
// This is mainly useful to wrap the an in-process client to insulate users of that
// client from having to care about potential immutability of data they receive or having
// the server implementation mutate their internal memory.
type CloningConnectCAServiceClient struct {
	ConnectCAServiceClient
}

func NewCloningConnectCAServiceClient(client ConnectCAServiceClient) ConnectCAServiceClient {
	if cloner, ok := client.(IsCloningConnectCAServiceClient); ok && cloner.IsCloningConnectCAServiceClient() {
		// prevent a double clone if the underlying client is already the cloning client.
		return client
	}

	return CloningConnectCAServiceClient{
		ConnectCAServiceClient: client,
	}
}

// IsCloningConnectCAServiceClient implements the IsCloningConnectCAServiceClient interface. This
// is only used to detect wrapped clients that would be double cloning data and prevent that.
func (c CloningConnectCAServiceClient) IsCloningConnectCAServiceClient() bool {
	return true
}

func (c CloningConnectCAServiceClient) Sign(ctx context.Context, in *SignRequest, opts ...grpc.CallOption) (*SignResponse, error) {
	in = proto.Clone(in).(*SignRequest)

	out, err := c.ConnectCAServiceClient.Sign(ctx, in)
	if err != nil {
		return nil, err
	}

	return proto.Clone(out).(*SignResponse), nil
}

func (c CloningConnectCAServiceClient) WatchRoots(ctx context.Context, in *WatchRootsRequest, opts ...grpc.CallOption) (ConnectCAService_WatchRootsClient, error) {
	in = proto.Clone(in).(*WatchRootsRequest)

	st, err := c.ConnectCAServiceClient.WatchRoots(ctx, in)
	if err != nil {
		return nil, err
	}

	return newCloningStream[*WatchRootsResponse](st), nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package e2e

import (
	context "context"
	"testing"

	proto "github.com/hashicorp/consul/internal/tools/protoc-gen-grpc-clone/e2e/proto"
	"github.com/hashicorp/consul/proto/private/prototest"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	grpc "google.golang.org/grpc"
)

func TestCloningClient_Unary(t *testing.T) {
	mclient := NewSimpleClient(t)

	expectedRequest := &proto.Req{Foo: "foo"}
	expectedResponse := &proto.Resp{Bar: "bar"}

	mclient.EXPECT().
		Something(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, actualRequest *proto.Req, _ ...grpc.CallOption) (*proto.Resp, error) {
			// The request object should have been cloned
			prototest.AssertDeepEqual(t, expectedRequest, actualRequest)
			require.NotSame(t, expectedRequest, actualRequest)
			return expectedResponse, nil
		}).
		Once()

	cloner := proto.NewCloningSimpleClient(mclient)

	actualResponse, err := cloner.Something(context.Background(), expectedRequest)
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, expectedResponse, actualResponse)
	require.NotSame(t, expectedResponse, actualResponse)
}

func TestCloningClient_StreamFromServer(t *testing.T) {
	expectedRequest := &proto.Req{Foo: "foo"}
	expectedResponse := &proto.Resp{Bar: "bar"}

	mstream := NewSimple_FlowClient(t)
	mclient := NewSimpleClient(t)

	mclient.EXPECT().
		Flow(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, actualRequest *proto.Req, _ ...grpc.CallOption) (proto.Simple_FlowClient, error) {
			// The request object should have been cloned
			prototest.AssertDeepEqual(t, expectedRequest, actualRequest)
			require.NotSame(t, expectedRequest, actualRequest)

			return mstream, nil
		}).
		Once()
	mstream.EXPECT().
		Recv().
		Return(expectedResponse, nil)

	cloner := proto.NewCloningSimpleClient(mclient)

	stream, err := cloner.Flow(context.Background(), expectedRequest)
	require.NoError(t, err)
	require.NotNil(t, stream)

	actualResponse, err := stream.Recv()
	require.NoError(t, err)
	prototest.AssertDeepEqual(t, expectedResponse, actualResponse)
	require.NotSame(t, expectedResponse, actualResponse)
}

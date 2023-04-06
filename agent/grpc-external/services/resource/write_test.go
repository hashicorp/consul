package resource

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource/demo"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbdemov2 "github.com/hashicorp/consul/proto/private/pbdemo/v2"
)

func TestWrite_InputValidation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	testCases := map[string]func(*pbresource.WriteRequest){
		"no resource": func(req *pbresource.WriteRequest) { req.Resource = nil },
		"no id":       func(req *pbresource.WriteRequest) { req.Resource.Id = nil },
		"no type":     func(req *pbresource.WriteRequest) { req.Resource.Id.Type = nil },
		"no tenancy":  func(req *pbresource.WriteRequest) { req.Resource.Id.Tenancy = nil },
		"no name":     func(req *pbresource.WriteRequest) { req.Resource.Id.Name = "" },
		"no data":     func(req *pbresource.WriteRequest) { req.Resource.Data = nil },
		"wrong data type": func(req *pbresource.WriteRequest) {
			var err error
			req.Resource.Data, err = anypb.New(&pbdemov2.Album{})
			require.NoError(t, err)
		},
	}
	for desc, modFn := range testCases {
		t.Run(desc, func(t *testing.T) {
			res, err := demo.GenerateV2Artist()
			require.NoError(t, err)

			req := &pbresource.WriteRequest{Resource: res}
			modFn(req)

			_, err = client.Write(testContext(t), req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
		})
	}
}

func TestWrite_TypeNotFound(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "resource type demo.v2.artist not registered")
}

func TestWrite_ResourceCreation(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	require.NotEmpty(t, rsp.Resource.Version, "resource should have version")
	require.NotEmpty(t, rsp.Resource.Id.Uid, "resource id should have uid")
	require.NotEmpty(t, rsp.Resource.Generation, "resource should have generation")
}

func TestWrite_CASUpdate_Success(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{
		Resource: modifyArtist(t, rsp1.Resource),
	})
	require.NoError(t, err)

	require.Equal(t, rsp1.Resource.Id.Uid, rsp2.Resource.Id.Uid)
	require.NotEqual(t, rsp1.Resource.Version, rsp2.Resource.Version)
	require.NotEqual(t, rsp1.Resource.Generation, rsp2.Resource.Generation)
}

func TestWrite_CASUpdate_Failure(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Version = "wrong-version"

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.Aborted.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "CAS operation failed")
}

func TestWrite_Update_WrongUid(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Id.Uid = "wrong-uid"

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition.String(), status.Code(err).String())
	require.Contains(t, err.Error(), "uid doesn't match")
}

func TestWrite_Update_NoUid(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Id.Uid = ""

	_, err = client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
}

func TestWrite_NonCASUpdate_Success(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	res = modifyArtist(t, rsp1.Resource)
	res.Version = ""

	rsp2, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)
	require.NotEmpty(t, rsp2.Resource.Version)
	require.NotEqual(t, rsp1.Resource.Version, rsp2.Resource.Version)
}

func TestWrite_NonCASUpdate_Retry(t *testing.T) {
	server := testServer(t)
	client := testClient(t, server)

	demo.Register(server.Registry)

	res, err := demo.GenerateV2Artist()
	require.NoError(t, err)

	rsp1, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
	require.NoError(t, err)

	// Simulate conflicting writes by blocking the RPC after it has read the
	// current version of the resource, but before it tries to make a write.
	backend := &blockOnceBackend{
		Backend: server.Backend,

		readCh:  make(chan struct{}),
		blockCh: make(chan struct{}),
	}
	server.Backend = backend

	errCh := make(chan error)
	go func() {
		res := modifyArtist(t, rsp1.Resource)
		res.Version = ""

		_, err := client.Write(testContext(t), &pbresource.WriteRequest{Resource: res})
		errCh <- err
	}()

	// Wait for the read, to ensure the Write in the goroutine above has read the
	// current version of the resource.
	<-backend.readCh

	// Update the resource.
	res = modifyArtist(t, rsp1.Resource)
	_, err = backend.WriteCAS(testContext(t), res)
	require.NoError(t, err)

	// Unblock the read.
	close(backend.blockCh)

	// Check that the write succeeded anyway because of a retry.
	require.NoError(t, <-errCh)
}

type blockOnceBackend struct {
	storage.Backend

	once    sync.Once
	readCh  chan struct{}
	blockCh chan struct{}
}

func (b *blockOnceBackend) Read(ctx context.Context, consistency storage.ReadConsistency, id *pbresource.ID) (*pbresource.Resource, error) {
	res, err := b.Backend.Read(ctx, consistency, id)

	b.once.Do(func() {
		close(b.readCh)
		<-b.blockCh
	})

	return res, err
}

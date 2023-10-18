package resourcetest

import (
	"context"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

type resourceBuilder struct {
	resource    *pbresource.Resource
	statuses    map[string]*pbresource.Status
	dontCleanup bool
}

func Resource(rtype *pbresource.Type, name string) *resourceBuilder {
	return &resourceBuilder{
		resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type: &pbresource.Type{
					Group:        rtype.Group,
					GroupVersion: rtype.GroupVersion,
					Kind:         rtype.Kind,
				},
				Tenancy: &pbresource.Tenancy{
					Partition: "default",
					Namespace: "default",
					PeerName:  "local",
				},
				Name: name,
			},
		},
	}
}

func (b *resourceBuilder) WithData(t T, data protoreflect.ProtoMessage) *resourceBuilder {
	t.Helper()

	anyData, err := anypb.New(data)
	require.NoError(t, err)
	b.resource.Data = anyData
	return b
}

func (b *resourceBuilder) WithMeta(key string, value string) *resourceBuilder {
	if b.resource.Metadata == nil {
		b.resource.Metadata = make(map[string]string)
	}

	b.resource.Metadata[key] = value
	return b
}

func (b *resourceBuilder) WithOwner(id *pbresource.ID) *resourceBuilder {
	b.resource.Owner = id
	return b
}

func (b *resourceBuilder) WithStatus(key string, status *pbresource.Status) *resourceBuilder {
	if b.statuses == nil {
		b.statuses = make(map[string]*pbresource.Status)
	}
	b.statuses[key] = status
	return b
}

func (b *resourceBuilder) WithoutCleanup() *resourceBuilder {
	b.dontCleanup = true
	return b
}

func (b *resourceBuilder) WithGeneration(gen string) *resourceBuilder {
	b.resource.Generation = gen
	return b
}

func (b *resourceBuilder) Build() *pbresource.Resource {
	// clone the resource so we can add on status information
	res := proto.Clone(b.resource).(*pbresource.Resource)

	// fill in the generation if empty to make it look like
	// a real managed resource
	if res.Generation == "" {
		res.Generation = ulid.Make().String()
	}

	// Now create the status map
	res.Status = make(map[string]*pbresource.Status)
	for key, original := range b.statuses {
		status := &pbresource.Status{
			ObservedGeneration: res.Generation,
			Conditions:         original.Conditions,
		}
		res.Status[key] = status
	}

	return res
}

func (b *resourceBuilder) ID() *pbresource.ID {
	return b.resource.Id
}

func (b *resourceBuilder) Write(t T, client pbresource.ResourceServiceClient) *pbresource.Resource {
	t.Helper()

	res := b.resource

	rsp, err := client.Write(context.Background(), &pbresource.WriteRequest{
		Resource: res,
	})

	require.NoError(t, err)

	if !b.dontCleanup {
		cleaner, ok := t.(CleanupT)
		require.True(t, ok, "T does not implement a Cleanup method and cannot be used with automatic resource cleanup")
		cleaner.Cleanup(func() {
			_, err := client.Delete(context.Background(), &pbresource.DeleteRequest{
				Id: rsp.Resource.Id,
			})
			require.NoError(t, err)
		})
	}

	if len(b.statuses) == 0 {
		return rsp.Resource
	}

	for key, original := range b.statuses {
		status := &pbresource.Status{
			ObservedGeneration: rsp.Resource.Generation,
			Conditions:         original.Conditions,
		}
		_, err := client.WriteStatus(context.Background(), &pbresource.WriteStatusRequest{
			Id:     rsp.Resource.Id,
			Key:    key,
			Status: status,
		})
		require.NoError(t, err)
	}

	readResp, err := client.Read(context.Background(), &pbresource.ReadRequest{
		Id: rsp.Resource.Id,
	})

	require.NoError(t, err)

	return readResp.Resource
}

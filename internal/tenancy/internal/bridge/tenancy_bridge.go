// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bridge

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

// V2TenancyBridge is used by the resource service to access V2 implementations of
// partitions and namespaces.
type V2TenancyBridge struct {
	client pbresource.ResourceServiceClient
}

// WithClient inject a ResourceServiceClient in the V2TenancyBridge.
// This is needed to break a circular dependency between
// the ResourceServiceServer, ResourceServiceClient and the TenancyBridge
func (b *V2TenancyBridge) WithClient(client pbresource.ResourceServiceClient) *V2TenancyBridge {
	b.client = client
	return b
}

func NewV2TenancyBridge() *V2TenancyBridge {
	return &V2TenancyBridge{}
}

func (b *V2TenancyBridge) NamespaceExists(partition, namespace string) (bool, error) {
	if namespace == resource.DefaultNamespaceName {
		// The default namespace implicitly exists in all partitions regardless of whether
		// the resource has actually been created yet. Therefore all we need to do is check
		// if the partition exists to know whether the namespace exists.
		return b.PartitionExists(partition)
	}

	_, err := b.client.Read(context.Background(), &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name: namespace,
			Tenancy: &pbresource.Tenancy{
				Partition: partition,
			},
			Type: pbtenancy.NamespaceType,
		},
	})
	switch {
	case err == nil:
		return true, nil
	case status.Code(err) == codes.NotFound:
		return false, nil
	default:
		return false, err
	}
}

func (b *V2TenancyBridge) IsNamespaceMarkedForDeletion(partition, namespace string) (bool, error) {
	rsp, err := b.client.Read(context.Background(), &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name: namespace,
			Tenancy: &pbresource.Tenancy{
				Partition: partition,
			},
			Type: pbtenancy.NamespaceType,
		},
	})
	if err != nil {
		return false, err
	}
	return resource.IsMarkedForDeletion(rsp.Resource), nil
}

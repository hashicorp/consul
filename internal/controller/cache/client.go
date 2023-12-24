// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cache

import (
	"context"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc"
)

type writeThroughCacheClient struct {
	cache WriteCache
	pbresource.ResourceServiceClient
}

func NewCachedClient(cache WriteCache, client pbresource.ResourceServiceClient) pbresource.ResourceServiceClient {
	return &writeThroughCacheClient{
		cache:                 cache,
		ResourceServiceClient: client,
	}
}

func (c *writeThroughCacheClient) Write(ctx context.Context, in *pbresource.WriteRequest, opts ...grpc.CallOption) (*pbresource.WriteResponse, error) {
	rsp, err := c.ResourceServiceClient.Write(ctx, in, opts...)
	if err != nil {
		return rsp, err
	}

	// There was no error so insert the resource into the cache
	c.cache.Insert(rsp.Resource)
	return rsp, err
}

func (c *writeThroughCacheClient) WriteStatus(ctx context.Context, in *pbresource.WriteStatusRequest, opts ...grpc.CallOption) (*pbresource.WriteStatusResponse, error) {
	rsp, err := c.ResourceServiceClient.WriteStatus(ctx, in, opts...)
	if err != nil {
		return rsp, err
	}

	// There was no error so insert the resource into the cache
	c.cache.Insert(rsp.Resource)
	return rsp, err
}

func (c *writeThroughCacheClient) Delete(ctx context.Context, in *pbresource.DeleteRequest, opts ...grpc.CallOption) (*pbresource.DeleteResponse, error) {
	rsp, err := c.ResourceServiceClient.Delete(ctx, in, opts...)
	if err != nil {
		return rsp, err
	}

	// There was no error so delete the resource from the cache
	c.cache.Delete(&pbresource.Resource{Id: in.Id})
	return rsp, err
}

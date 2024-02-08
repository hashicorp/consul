// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tenancy

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// This duplicates a subset of internal/resource/resourcetest/client.go so
// we're not importing consul internals integration tests.
//
// TODO: Move to a general package if used more widely.

type ClientOption func(*Client)

func WithACLToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

// Client decorates a resource service client with helper functions to assist
// with integration testing.
type Client struct {
	pbresource.ResourceServiceClient

	timeout time.Duration
	wait    time.Duration
	token   string
}

func NewClient(client pbresource.ResourceServiceClient, opts ...ClientOption) *Client {
	c := &Client{
		ResourceServiceClient: client,
		timeout:               7 * time.Second,
		wait:                  50 * time.Millisecond,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func NewClientWithACLToken(client pbresource.ResourceServiceClient, token string) *Client {
	return NewClient(client, WithACLToken(token))
}

func (client *Client) SetRetryerConfig(timeout time.Duration, wait time.Duration) {
	client.timeout = timeout
	client.wait = wait
}

func (client *Client) retry(t testutil.TestingTB, fn func(r *retry.R)) {
	t.Helper()
	retryer := &retry.Timer{Timeout: client.timeout, Wait: client.wait}
	retry.RunWith(retryer, t, fn)
}

func (client *Client) Context(t testutil.TestingTB) context.Context {
	ctx := testutil.TestContext(t)

	if client.token != "" {
		md := metadata.New(map[string]string{
			"x-consul-token": client.token,
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return ctx
}

func (client *Client) RequireResourceNotFound(t testutil.TestingTB, id *pbresource.ID) {
	t.Helper()

	rsp, err := client.Read(client.Context(t), &pbresource.ReadRequest{Id: id})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
	require.Nil(t, rsp)
}

func (client *Client) RequireResourceExists(t testutil.TestingTB, id *pbresource.ID) *pbresource.Resource {
	t.Helper()

	rsp, err := client.Read(client.Context(t), &pbresource.ReadRequest{Id: id})
	require.NoError(t, err, "error reading %s with type %s", id.Name, ToGVK(id.Type))
	require.NotNil(t, rsp)
	return rsp.Resource
}

func ToGVK(resourceType *pbresource.Type) string {
	return fmt.Sprintf("%s.%s.%s", resourceType.Group, resourceType.GroupVersion, resourceType.Kind)
}

func (client *Client) WaitForResourceExists(t testutil.TestingTB, id *pbresource.ID) *pbresource.Resource {
	t.Helper()

	var res *pbresource.Resource
	client.retry(t, func(r *retry.R) {
		res = client.RequireResourceExists(r, id)
	})

	return res
}

func (client *Client) WaitForDeletion(t testutil.TestingTB, id *pbresource.ID) {
	t.Helper()

	client.retry(t, func(r *retry.R) {
		client.RequireResourceNotFound(r, id)
	})
}

// MustDelete will delete a resource by its id, retrying if necessary and fail the test
// if it cannot delete it within the timeout. The clients request delay settings are
// taken into account with this operation.
func (client *Client) MustDelete(t testutil.TestingTB, id *pbresource.ID) {
	t.Helper()
	client.retryDelete(t, id)
}

func (client *Client) retryDelete(t testutil.TestingTB, id *pbresource.ID) {
	t.Helper()
	ctx := client.Context(t)

	client.retry(t, func(r *retry.R) {
		_, err := client.Delete(ctx, &pbresource.DeleteRequest{Id: id})
		if status.Code(err) == codes.NotFound {
			return
		}

		// codes.Aborted indicates a CAS failure and that the delete request should
		// be retried. Anything else should be considered an unrecoverable error.
		if err != nil && status.Code(err) != codes.Aborted {
			r.Stop(fmt.Errorf("failed to delete the resource: %w", err))
			return
		}

		require.NoError(r, err)
	})
}

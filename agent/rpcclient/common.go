// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rpcclient

import (
	"context"

	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/submatview"
)

// NetRPC reprents an interface for making RPC requests
type NetRPC interface {
	RPC(ctx context.Context, method string, args interface{}, reply interface{}) error
}

// CacheGetter represents an interface for interacting with the cache
type CacheGetter interface {
	Get(ctx context.Context, t string, r cache.Request) (interface{}, cache.ResultMeta, error)
	NotifyCallback(ctx context.Context, t string, r cache.Request, cID string, cb cache.Callback) error
}

// MaterializedViewStore represents an interface for interacting with the material view store
type MaterializedViewStore interface {
	Get(ctx context.Context, req submatview.Request) (submatview.Result, error)
	NotifyCallback(ctx context.Context, req submatview.Request, cID string, cb cache.Callback) error
}

// MaterializerDeps include the dependencies for the materializer
type MaterializerDeps struct {
	Conn   *grpc.ClientConn
	Logger hclog.Logger
}

// Client represents a rpc client, a new Client is created in each sub-package
// embedding this object. Methods are therefore implemented in the subpackages
// as well.
type Client struct {
	NetRPC              NetRPC
	Cache               CacheGetter
	ViewStore           MaterializedViewStore
	MaterializerDeps    MaterializerDeps
	CacheName           string
	UseStreamingBackend bool
	QueryOptionDefaults func(options *structs.QueryOptions)
}

// Close any underlying connections used by the client.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.MaterializerDeps.Conn.Close()
}

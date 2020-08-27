package kv

import (
	"context"

	"github.com/hashicorp/consul/agent/structs"
)

type Client struct {
	NetRPC NetRPC
}

type NetRPC interface {
	RPC(method string, args interface{}, reply interface{}) error
}

func (c *Client) Get(_ context.Context, req structs.KeyRequest) (structs.IndexedDirEntries, error) {
	var out structs.IndexedDirEntries
	err := c.NetRPC.RPC("KVS.Get", &req, &out)
	return out, err
}

func (c *Client) Apply(_ context.Context, req structs.KVSRequest) (bool, error) {
	var out bool
	err := c.NetRPC.RPC("KVS.Apply", &req, &out)
	return out, err
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package leafcert

import (
	"fmt"
	"net"
	"time"

	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// ConnectCALeafRequest is the cache.Request implementation for the
// ConnectCALeaf cache type. This is implemented here and not in structs
// since this is only used for cache-related requests and not forwarded
// directly to any Consul servers.
type ConnectCALeafRequest struct {
	Token         string
	Datacenter    string
	DNSSAN        []string
	IPSAN         []net.IP
	MinQueryIndex uint64
	MaxQueryTime  time.Duration
	acl.EnterpriseMeta
	MustRevalidate bool

	// The following flags indicate the entity we are requesting a cert for.
	// Only one of these must be specified.
	Service string              // Given a Service name, not ID, the request is for a SpiffeIDService.
	Agent   string              // Given an Agent name, not ID, the request is for a SpiffeIDAgent.
	Kind    structs.ServiceKind // Given "mesh-gateway", the request is for a SpiffeIDMeshGateway. No other kinds supported.
	Server  bool                // If true, the request is for a SpiffeIDServer.
}

func (r *ConnectCALeafRequest) Key() string {
	r.EnterpriseMeta.Normalize()

	switch {
	case r.Agent != "":
		v, err := hashstructure.Hash([]any{
			r.Agent,
			r.PartitionOrDefault(),
		}, nil)
		if err == nil {
			return fmt.Sprintf("agent:%d", v)
		}
	case r.Kind == structs.ServiceKindMeshGateway:
		v, err := hashstructure.Hash([]any{
			r.PartitionOrDefault(),
			r.DNSSAN,
			r.IPSAN,
		}, nil)
		if err == nil {
			return fmt.Sprintf("kind:%d", v)
		}
	case r.Kind != "":
		// this is not valid
	case r.Server:
		v, err := hashstructure.Hash([]any{
			"server",
			r.Datacenter,
		}, nil)
		if err == nil {
			return fmt.Sprintf("server:%d", v)
		}
	default:
		v, err := hashstructure.Hash([]any{
			r.Service,
			r.EnterpriseMeta,
			r.DNSSAN,
			r.IPSAN,
		}, nil)
		if err == nil {
			return fmt.Sprintf("service:%d", v)
		}
	}

	// If there is an error, we don't set the key. A blank key forces
	// no cache for this request so the request is forwarded directly
	// to the server.
	return ""
}

func (req *ConnectCALeafRequest) TargetNamespace() string {
	return req.NamespaceOrDefault()
}

func (req *ConnectCALeafRequest) TargetPartition() string {
	return req.PartitionOrDefault()
}

func (r *ConnectCALeafRequest) CacheInfo() cache.RequestInfo {
	return cache.RequestInfo{
		Token:          r.Token,
		Key:            r.Key(),
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MustRevalidate: r.MustRevalidate,
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

type DiscoveryChain struct {
	srv *Server
}

func (c *DiscoveryChain) Get(args *structs.DiscoveryChainRequest, reply *structs.DiscoveryChainResponse) error {
	// Exit early if Connect hasn't been enabled.
	if !c.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := c.srv.ForwardRPC("DiscoveryChain.Get", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"discovery_chain", "get"}, time.Now())

	// Fetch the ACL token, if any.
	entMeta := args.GetEnterpriseMeta()
	var authzContext acl.AuthorizerContext
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, entMeta, &authzContext)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().ServiceReadAllowed(args.Name, &authzContext); err != nil {
		return err
	}

	if args.Name == "" {
		return fmt.Errorf("Must provide service name")
	}

	evalDC := args.EvaluateInDatacenter
	if evalDC == "" {
		evalDC = c.srv.config.Datacenter
	}

	var (
		priorHash uint64
		ranOnce   bool
	)
	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			req := discoverychain.CompileRequest{
				ServiceName:            args.Name,
				EvaluateInNamespace:    entMeta.NamespaceOrDefault(),
				EvaluateInPartition:    entMeta.PartitionOrDefault(),
				EvaluateInDatacenter:   evalDC,
				OverrideMeshGateway:    args.OverrideMeshGateway,
				OverrideProtocol:       args.OverrideProtocol,
				OverrideConnectTimeout: args.OverrideConnectTimeout,
			}
			index, chain, entries, err := state.ServiceDiscoveryChain(ws, args.Name, entMeta, req)
			if err != nil {
				return err
			}

			// Generate a hash of the config entry content driving this
			// response. Use it to determine if the response is identical to a
			// prior wakeup.
			newHash := computeDiscoveryChainHash(entries, req, chain)

			if ranOnce && priorHash == newHash {
				priorHash = newHash
				reply.Index = index
				// NOTE: the prior response is still alive inside of *reply, which
				// is desirable
				return errNotChanged
			} else {
				priorHash = newHash
				ranOnce = true
			}

			reply.Index = index
			reply.Chain = chain

			if entries.IsEmpty() {
				return errNotFound
			}

			return nil
		})
}

func computeDiscoveryChainHash(
	entries *configentry.DiscoveryChainSet,
	req discoverychain.CompileRequest,
	chain *structs.CompiledDiscoveryChain,
) uint64 {
	h := fnv.New64a()

	// Helper to write a uint64 to the hash
	writeUint64 := func(v uint64) {
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], v)
		h.Write(buf[:])
	}

	// Helper to write a string to the hash
	writeString := func(s string) {
		h.Write([]byte(s))
		h.Write([]byte{0}) // null terminator as separator
	}

	// 1. Hash all config entries using their pre-computed hashes
	if entries != nil {
		writeUint64(entries.Hash())
	}

	// 2. Hash compile request parameters that affect the output
	writeString(req.ServiceName)
	writeString(req.EvaluateInNamespace)
	writeString(req.EvaluateInPartition)
	writeString(req.EvaluateInDatacenter)
	writeString(req.EvaluateInTrustDomain)
	writeString(req.OverrideProtocol)
	writeUint64(uint64(req.OverrideConnectTimeout))
	writeString(string(req.OverrideMeshGateway.Mode))

	// 3. Hash virtual IPs which also affect the result
	if chain != nil {
		for _, vip := range chain.AutoVirtualIPs {
			writeString(vip)
		}
		for _, vip := range chain.ManualVirtualIPs {
			writeString(vip)
		}
	}

	return h.Sum64()
}

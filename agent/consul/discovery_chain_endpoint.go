// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
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
			newHash, err := hashstructure_v2.Hash(chain, hashstructure_v2.FormatV2, nil)
			if err != nil {
				return fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
			}

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

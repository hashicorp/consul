// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"sync"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"
	"golang.org/x/sync/singleflight"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

type DiscoveryChain struct {
	srv *Server
	// Materialized view cache to store compiled discovery chains
	cacheMu sync.RWMutex
	cache   map[string]*discoveryChainCacheEntry
	// Single-flight group to coalesce concurrent compilation requests
	sfGroup singleflight.Group
}

type discoveryChainCacheEntry struct {
	chain     *structs.CompiledDiscoveryChain
	index     uint64
	hash      uint64
	timestamp time.Time
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
	// Generate cache key for this request
	cacheKey := fmt.Sprintf("%s/%s/%s/%s", args.Name, entMeta.NamespaceOrDefault(), entMeta.PartitionOrDefault(), evalDC)
	c.srv.logger.Trace("cachekey: ", cacheKey)

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

			// Use singleflight to coalesce concurrent compilation requests
			result, err, shared := c.sfGroup.Do(cacheKey, func() (interface{}, error) {
				if c.srv.logger != nil {
					c.srv.logger.Trace("[DISCOVERY_CHAIN_COMPILE_START]", "service", args.Name)
				}

				// Add 10 second delay to simulate expensive compilation
				time.Sleep(10 * time.Second)

				index, chain, entries, err := state.ServiceDiscoveryChain(ws, args.Name, entMeta, req)
				if err != nil {
					return nil, err
				}

				newHash, err := hashstructure_v2.Hash(chain, hashstructure_v2.FormatV2, nil)
				if err != nil {
					return nil, fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
				}

				// Cache the result
				c.cacheMu.Lock()
				if c.cache == nil {
					c.cache = make(map[string]*discoveryChainCacheEntry)
				}
				c.cache[cacheKey] = &discoveryChainCacheEntry{
					chain:     chain,
					index:     index,
					hash:      newHash,
					timestamp: time.Now(),
				}
				c.cacheMu.Unlock()

				return &struct {
					chain   *structs.CompiledDiscoveryChain
					index   uint64
					hash    uint64
					isEmpty bool
				}{chain, index, newHash, entries.IsEmpty()}, nil
			})

			if c.srv.logger != nil {
				c.srv.logger.Info("[DISCOVERY_CHAIN_COMPILED]", "service", args.Name, "shared_request", shared)
			}

			if err != nil {
				return err
			}

			compiled := result.(*struct {
				chain   *structs.CompiledDiscoveryChain
				index   uint64
				hash    uint64
				isEmpty bool
			})

			if ranOnce && priorHash == compiled.hash {
				priorHash = compiled.hash
				reply.Index = compiled.index
				return errNotChanged
			} else {
				priorHash = compiled.hash
				ranOnce = true
			}

			reply.Index = compiled.index
			reply.Chain = compiled.chain

			if compiled.isEmpty {
				return errNotFound
			}

			return nil
		})
}

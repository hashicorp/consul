// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package router

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"
)

// FloodAddrFn gets the address and port to use for a given server when
// flood-joining. This will return false if it doesn't have one.
type FloodAddrFn func(*metadata.Server) (string, error)

// FloodJoins attempts to make sure all Consul servers in the src Serf
// instance are joined in the dst Serf instance. It assumes names in the
// src area are of the form <node> and those in the dst area are of the
// form <node>.<dc> as is done for WAN and general network areas in Consul
// Enterprise.
func FloodJoins(logger hclog.Logger, addrFn FloodAddrFn,
	localDatacenter string, srcSerf *serf.Serf, dstSerf *serf.Serf) {

	// Names in the dst Serf have the datacenter suffixed.
	suffix := fmt.Sprintf(".%s", localDatacenter)

	// Index the dst side so we can do one pass through the src side
	// with cheap lookups.
	index := make(map[string]*metadata.Server)
	for _, m := range dstSerf.Members() {
		ok, server := metadata.IsConsulServer(m)
		if !ok {
			continue
		}

		if server.Datacenter != localDatacenter {
			continue
		}

		srcName := strings.TrimSuffix(server.Name, suffix)
		index[srcName] = server
	}

	// Now run through the src side and look for joins.
	for _, m := range srcSerf.Members() {
		if m.Status != serf.StatusAlive {
			continue
		}

		ok, server := metadata.IsConsulServer(m)
		if !ok {
			continue
		}

		if _, ok := index[server.Name]; ok {
			continue
		}

		addr, err := addrFn(server)
		if err != nil {
			logger.Debug("Failed to flood-join server", "server",
				server.Name, "address", server.Addr.String(),
				"error", err,
			)
			continue
		}

		dstServerName := fmt.Sprintf("%s.%s", server.Name, server.Datacenter)

		// Do the join!
		n, err := dstSerf.Join([]string{dstServerName + "/" + addr}, true)
		if err != nil {
			logger.Debug("Failed to flood-join server at address",
				"server", dstServerName,
				"address", addr,
				"error", err,
			)
		} else if n > 0 {
			logger.Debug("Successfully performed flood-join for server at address",
				"server", dstServerName,
				"address", addr,
			)
		}
	}
}

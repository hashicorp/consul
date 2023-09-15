// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package router

import (
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/types"
)

// ServerTracker is called when Router is notified of a server being added or
// removed.
type ServerTracker interface {
	NewRebalancer(dc string) func()
	AddServer(types.AreaID, *metadata.Server)
	RemoveServer(types.AreaID, *metadata.Server)
}

// Rebalancer is called periodically to re-order the servers so that the load on the
// servers is evenly balanced.
type Rebalancer func()

// NoOpServerTracker is a ServerTracker that does nothing. Used when gRPC is not
// enabled.
type NoOpServerTracker struct{}

// Rebalance does nothing
func (NoOpServerTracker) NewRebalancer(string) func() {
	return func() {}
}

// AddServer does nothing
func (NoOpServerTracker) AddServer(types.AreaID, *metadata.Server) {}

// RemoveServer does nothing
func (NoOpServerTracker) RemoveServer(types.AreaID, *metadata.Server) {}

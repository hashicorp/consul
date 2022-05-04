package router

import (
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/types"
)

// ServerTracker is called when Router is notified of a server being added or
// removed.
type ServerTracker interface {
	AddServer(types.AreaID, *metadata.Server)
	RemoveServer(types.AreaID, *metadata.Server)
}

// NoOpServerTracker is a ServerTracker that does nothing. Used when gRPC is not
// enabled.
type NoOpServerTracker struct{}

// AddServer does nothing
func (NoOpServerTracker) AddServer(types.AreaID, *metadata.Server) {}

// RemoveServer does nothing
func (NoOpServerTracker) RemoveServer(types.AreaID, *metadata.Server) {}

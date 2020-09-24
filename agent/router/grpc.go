package router

import "github.com/hashicorp/consul/agent/metadata"

// ServerTracker is called when Router is notified of a server being added or
// removed.
type ServerTracker interface {
	NewRebalancer(dc string) func()
	AddServer(*metadata.Server)
	RemoveServer(*metadata.Server)
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
func (NoOpServerTracker) AddServer(*metadata.Server) {}

// RemoveServer does nothing
func (NoOpServerTracker) RemoveServer(*metadata.Server) {}

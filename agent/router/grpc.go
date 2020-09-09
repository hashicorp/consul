package router

import "github.com/hashicorp/consul/agent/metadata"

// ServerTracker is a wrapper around consul.ServerResolverBuilder to prevent a
// cyclic import dependency.
type ServerTracker interface {
	AddServer(*metadata.Server)
	RemoveServer(*metadata.Server)
}

// NoOpServerTracker is a ServerTracker that does nothing. Used when gRPC is not
// enabled.
type NoOpServerTracker struct{}

// AddServer implements ServerTracker
func (NoOpServerTracker) AddServer(*metadata.Server) {}

// RemoveServer implements ServerTracker
func (NoOpServerTracker) RemoveServer(*metadata.Server) {}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package balancer

import (
	"fmt"
	"sync"

	gbalancer "google.golang.org/grpc/balancer"
)

// BuilderName should be given in gRPC service configuration to enable our
// custom balancer. It refers to this package's global registry, rather than
// an instance of Builder to enable us to add and remove builders at runtime,
// specifically during tests.
const BuilderName = "consul-internal"

// gRPC's balancer.Register method is thread-unsafe because it mutates a global
// map without holding a lock. As such, it's expected that you register custom
// balancers once at the start of your program (e.g. a package init function).
//
// In production, this is fine. Agents register a single instance of our builder
// and use it for the duration. Tests are where this becomes problematic, as we
// spin up several agents in-memory and register/deregister a builder for each,
// with its own agent-specific state, logger, etc.
//
// To avoid data races, we call gRPC's Register method once, on-package init,
// with a global registry struct that implements the Builder interface but
// delegates the building to N instances of our Builder that are registered and
// deregistered at runtime. We the dial target's host (aka "authority") which
// is unique per-agent to pick the correct builder.
func init() {
	gbalancer.Register(globalRegistry)
}

var globalRegistry = &registry{
	byAuthority: make(map[string]*Builder),
}

type registry struct {
	mu          sync.RWMutex
	byAuthority map[string]*Builder
}

func (r *registry) Build(cc gbalancer.ClientConn, opts gbalancer.BuildOptions) gbalancer.Balancer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	auth := opts.Target.URL.Host
	builder, ok := r.byAuthority[auth]
	if !ok {
		panic(fmt.Sprintf("no gRPC balancer builder registered for authority: %q", auth))
	}
	return builder.Build(cc, opts)
}

func (r *registry) Name() string { return BuilderName }

func (r *registry) register(auth string, builder *Builder) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.byAuthority[auth] = builder
}

func (r *registry) deregister(auth string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.byAuthority, auth)
}

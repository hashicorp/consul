// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Manager is responsible for scheduling the execution of controllers.
type Manager struct {
	client     pbresource.ResourceServiceClient
	registry   resource.Registry
	logger     hclog.Logger
	raftLeader atomic.Bool

	mu          sync.Mutex
	running     bool
	controllers []Controller
	leaseChans  []chan struct{}
}

// NewManager creates a Manager. logger will be used by the Manager, and as the
// base logger for controllers when one is not specified using WithLogger.
func NewManager(client pbresource.ResourceServiceClient, registry resource.Registry, logger hclog.Logger) *Manager {
	return &Manager{
		client:   client,
		registry: registry,
		logger:   logger,
	}
}

// Register the given controller to be executed by the Manager. Cannot be called
// once the Manager is running.
func (m *Manager) Register(ctrl Controller) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		panic("cannot register additional controllers after calling Run")
	}

	if ctrl.reconciler == nil {
		panic(fmt.Sprintf("cannot register controller without a reconciler %s", ctrl))
	}

	m.controllers = append(m.controllers, ctrl)
}

// Run the Manager and start executing controllers until the given context is
// canceled. Cannot be called more than once.
func (m *Manager) Run(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		panic("cannot call Run more than once")
	}
	m.running = true

	for _, desc := range m.controllers {
		logger := desc.logger
		if logger == nil {
			logger = m.logger.With("managed_type", resource.ToGVK(desc.managedType))
		}

		runner := &controllerRunner{
			ctrl:     desc,
			client:   m.client,
			registry: m.registry,
			logger:   logger,
		}
		go newSupervisor(runner.run, m.newLeaseLocked(desc)).run(ctx)
	}
}

// SetRaftLeader notifies the Manager of Raft leadership changes. Controllers
// are currently only executed on the Raft leader, so calling this method will
// cause the Manager to spin them up/down accordingly.
func (m *Manager) SetRaftLeader(leader bool) {
	m.raftLeader.Store(leader)

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ch := range m.leaseChans {
		select {
		case ch <- struct{}{}:
		default:
			// Do not block if there's nothing receiving on ch (because the supervisor is
			// busy doing something else). Note that ch has a buffer of 1, so we'll never
			// miss the notification that something has changed so we need to re-evaluate
			// the lease.
		}
	}
}

func (m *Manager) newLeaseLocked(ctrl Controller) Lease {
	if ctrl.placement == PlacementEachServer {
		return eternalLease{}
	}

	ch := make(chan struct{}, 1)
	m.leaseChans = append(m.leaseChans, ch)
	return &raftLease{m: m, ch: ch}
}

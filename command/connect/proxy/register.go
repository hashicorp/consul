// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/api"
)

const (
	// RegisterReconcilePeriod is how often the monitor will attempt to
	// reconcile the expected service state with the remote Consul server.
	RegisterReconcilePeriod = 30 * time.Second

	// RegisterTTLPeriod is the TTL setting for the health check of the
	// service. The monitor will automatically pass the health check
	// three times per this period to be more resilient to failures.
	RegisterTTLPeriod = 30 * time.Second
)

// RegisterMonitor registers the proxy with the local Consul agent with a TTL
// health check that is kept alive.
//
// This struct should be initialized with NewRegisterMonitor instead of being
// allocated directly. Using this struct without calling NewRegisterMonitor
// will result in panics.
type RegisterMonitor struct {
	// Logger is the logger for the monitor.
	Logger hclog.Logger

	// Client is the API client to a specific Consul agent. This agent is
	// where the service will be registered.
	Client *api.Client

	// Service is the name of the service being proxied.
	Service string

	// LocalAddress and LocalPort are the address and port of the proxy
	// itself, NOT the service being proxied.
	LocalAddress string
	LocalPort    int

	// IDSuffix is a unique ID that is appended to the end of the service
	// name. This helps the service be unique. By default the service ID
	// is just the proxied service name followed by "-proxy".
	IDSuffix string

	// The fields below are related to timing settings. See the default
	// constants for more documentation on what they set.
	ReconcilePeriod time.Duration
	TTLPeriod       time.Duration

	// lock is held while reading/writing any internal state of the monitor.
	// cond is a condition variable on lock that is broadcasted for runState
	// changes.
	lock *sync.Mutex
	cond *sync.Cond

	// runState is the current state of the monitor. To read this the
	// lock must be held. The condition variable cond can be waited on
	// for changes to this value.
	runState registerRunState
}

// registerState is the state of the RegisterMonitor.
//
// This is a basic state machine with the following transitions:
//
//   - idle     => running, stopped
//   - running  => stopping, stopped
//   - stopping => stopped
//   - stopped  => <>
type registerRunState uint8

const (
	registerStateIdle registerRunState = iota
	registerStateRunning
	registerStateStopping
	registerStateStopped
)

// NewRegisterMonitor initializes a RegisterMonitor. After initialization,
// the exported fields should be configured as desired. To start the monitor,
// execute Run in a goroutine.
func NewRegisterMonitor(logger hclog.Logger) *RegisterMonitor {
	var lock sync.Mutex

	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{})
	}
	return &RegisterMonitor{
		Logger:          logger, // default logger
		ReconcilePeriod: RegisterReconcilePeriod,
		TTLPeriod:       RegisterTTLPeriod,
		lock:            &lock,
		cond:            sync.NewCond(&lock),
	}
}

// Run should be started in a goroutine and will keep Consul updated
// in the background with the state of this proxy. If registration fails
// this will continue to retry.
func (r *RegisterMonitor) Run() {
	// Grab the lock and set our state. If we're not idle, then we return
	// immediately since the monitor is only allowed to run once.
	r.lock.Lock()
	if r.runState != registerStateIdle {
		r.lock.Unlock()
		return
	}
	r.runState = registerStateRunning
	r.lock.Unlock()

	// Start a goroutine that just waits for a stop request
	stopCh := make(chan struct{})
	go func() {
		defer close(stopCh)
		r.lock.Lock()
		defer r.lock.Unlock()

		// We wait for anything not running, just so we're more resilient
		// in the face of state machine issues. Basically any state change
		// will cause us to quit.
		for r.runState == registerStateRunning {
			r.cond.Wait()
		}
	}()

	// When we exit, we set the state to stopped and broadcast to any
	// waiting Close functions that they can return.
	defer func() {
		r.lock.Lock()
		r.runState = registerStateStopped
		r.cond.Broadcast()
		r.lock.Unlock()
	}()

	// Run the first registration optimistically. If this fails then its
	// okay since we'll just retry shortly.
	r.register()

	// Create the timers for trigger events. We don't use tickers because
	// we don't want the events to pile on.
	reconcileTimer := time.NewTimer(r.ReconcilePeriod)
	heartbeatTimer := time.NewTimer(r.TTLPeriod / 3)

	for {
		select {
		case <-reconcileTimer.C:
			r.register()
			reconcileTimer.Reset(r.ReconcilePeriod)

		case <-heartbeatTimer.C:
			r.heartbeat()
			heartbeatTimer.Reset(r.TTLPeriod / 3)

		case <-stopCh:
			r.Logger.Info("stop request received, deregistering")
			r.deregister()
			return
		}
	}
}

// register queries the Consul agent to determine if we've already registered.
// If we haven't or the registered service differs from what we're trying to
// register, then we attempt to register our service.
func (r *RegisterMonitor) register() {
	catalog := r.Client.Catalog()
	serviceID := r.serviceID()
	serviceName := r.serviceName()

	// Determine the current state of this service in Consul
	var currentService *api.CatalogService
	services, _, err := catalog.Service(
		serviceName, "",
		&api.QueryOptions{AllowStale: true})
	if err == nil {
		for _, service := range services {
			if serviceID == service.ServiceID {
				currentService = service
				break
			}
		}
	}

	// If we have a matching service, then we verify if we need to reregister
	// by comparing if it matches what we expect.
	if currentService != nil &&
		currentService.ServiceAddress == r.LocalAddress &&
		currentService.ServicePort == r.LocalPort {
		r.Logger.Debug("service already registered, not re-registering")
		return
	}

	// If we're here, then we're registering the service.
	err = r.Client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Kind:    api.ServiceKindConnectProxy,
		ID:      serviceID,
		Name:    serviceName,
		Address: r.LocalAddress,
		Port:    r.LocalPort,
		Proxy: &api.AgentServiceConnectProxyConfig{
			DestinationServiceName: r.Service,
		},
		Check: &api.AgentServiceCheck{
			CheckID: r.checkID(),
			Name:    "proxy heartbeat",
			TTL:     "30s",
			Notes:   "Built-in proxy will heartbeat this check.",
			Status:  "passing",
		},
	})
	if err != nil {
		r.Logger.Warn("Failed to register Consul service", "error", err)
		return
	}

	r.Logger.Info("registered Consul service", "service", serviceID)
}

// heartbeat just pings the TTL check for our service.
func (r *RegisterMonitor) heartbeat() {
	// Trigger the health check passing. We don't need to retry this
	// since we do a couple tries within the TTL period.
	if err := r.Client.Agent().PassTTL(r.checkID(), ""); err != nil {
		if !strings.Contains(err.Error(), "does not have associated") {
			r.Logger.Warn("heartbeat failed", "error", err)
		}
	}
}

// deregister deregisters the service.
func (r *RegisterMonitor) deregister() {
	// Basic retry loop, no backoff for now. But we want to retry a few
	// times just in case there are basic ephemeral issues.
	for i := 0; i < 3; i++ {
		err := r.Client.Agent().ServiceDeregister(r.serviceID())
		if err == nil {
			return
		}

		r.Logger.Warn("service deregister failed", "error", err)
		time.Sleep(500 * time.Millisecond)
	}
}

// Close stops the register goroutines and deregisters the service. Once
// Close is called, the monitor can no longer be used again. It is safe to
// call Close multiple times and concurrently.
func (r *RegisterMonitor) Close() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	for {
		switch r.runState {
		case registerStateIdle:
			// Idle so just set it to stopped and return. We notify
			// the condition variable in case others are waiting.
			r.runState = registerStateStopped
			r.cond.Broadcast()
			return nil

		case registerStateRunning:
			// Set the state to stopping and broadcast to all waiters,
			// since Run is sitting on cond.Wait.
			r.runState = registerStateStopping
			r.cond.Broadcast()
			r.cond.Wait() // Wait on the stopping event

		case registerStateStopping:
			// Still stopping, wait...
			r.cond.Wait()

		case registerStateStopped:
			// Stopped, target state reached
			return nil
		}
	}
}

// serviceID returns the unique ID for this proxy service.
func (r *RegisterMonitor) serviceID() string {
	id := fmt.Sprintf("%s-proxy", r.Service)
	if r.IDSuffix != "" {
		id += "-" + r.IDSuffix
	}

	return id
}

// serviceName returns the non-unique name of this proxy service.
func (r *RegisterMonitor) serviceName() string {
	return fmt.Sprintf("%s-proxy", r.Service)
}

// checkID is the unique ID for the registered health check.
func (r *RegisterMonitor) checkID() string {
	return fmt.Sprintf("%s-ttl", r.serviceID())
}

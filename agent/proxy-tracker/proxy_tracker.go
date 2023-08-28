// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxytracker

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-hclog"
	"sync"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh"
	"github.com/hashicorp/consul/internal/resource"

	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Proxy implements the queue.ItemType interface so that it can be used in a controller.Event.
// It is sent on the newProxyConnectionCh channel.
// TODO(ProxyState): needs to support tenancy in the future.
// Key() is current resourceID.Name.
type ProxyConnection struct {
	ProxyID *pbresource.ID
}

func (e *ProxyConnection) Key() string {
	return e.ProxyID.GetName()
}

// proxyWatchData is a handle on all of the relevant bits that is created by calling Watch().
// It is meant to be stored in the proxies cache by proxyID so that watches can be notified
// when the ProxyState for that proxyID has changed.
type proxyWatchData struct {
	// notifyCh is the channel that the watcher receives updates from ProxyTracker.
	notifyCh chan proxycfg.ProxySnapshot
	// state is the current/last updated ProxyState for a given proxy.
	state *mesh.ProxyState
	// token is the ACL token provided by the watcher.
	token string
	// nodeName is the node where the given proxy resides.
	nodeName string
}

type ProxyTrackerConfig struct {
	// logger will be used to write log messages.
	Logger hclog.Logger

	// sessionLimiter is used to enforce xDS concurrency limits.
	SessionLimiter SessionLimiter
}

// ProxyTracker implements the Watcher and Updater interfaces. The Watcher is used by the xds server to add a new proxy
// to this server, and get back a channel for updates. The Updater is used by the ProxyState controller running on the
// server to push ProxyState updates to the notify channel.
type ProxyTracker struct {
	config ProxyTrackerConfig
	// proxies is a cache of the proxies connected to this server and configuration information for each one.
	proxies map[resource.ReferenceKey]*proxyWatchData
	// newProxyConnectionCh is the channel that the "updater" retains to receive messages from ProxyTracker that a new
	// proxy has connected to ProxyTracker and a signal the "updater" should call PushChanges with a new state.
	newProxyConnectionCh chan controller.Event
	// shutdownCh is a channel that closes when ProxyTracker is shutdown. ShutdownChannel is never written to, only closed to
	// indicate a shutdown has been initiated.
	shutdownCh chan struct{}
	// mu is a mutex that is used internally for locking when reading and modifying ProxyTracker state, namely the proxies map.
	mu sync.Mutex
}

// NewProxyTracker returns a ProxyTracker instance given a configuration.
func NewProxyTracker(cfg ProxyTrackerConfig) *ProxyTracker {
	return &ProxyTracker{
		config:  cfg,
		proxies: make(map[resource.ReferenceKey]*proxyWatchData),
		// buffering this channel since ProxyTracker will be registering watches for all proxies.
		// using the buffer will limit errors related to controller and the proxy are both running
		// but the controllers listening function is not blocking on the particular receive line.
		// This channel is meant to error when the controller is "not ready" which means up and alive.
		// This buffer will try to reduce false negatives and limit unnecessary erroring.
		newProxyConnectionCh: make(chan controller.Event, 1000),
		shutdownCh:           make(chan struct{}),
	}
}

// Watch connects a proxy with ProxyTracker and returns the consumer a channel to receive updates,
// a channel to notify of xDS terminated session, and a cancel function to cancel the watch.
func (pt *ProxyTracker) Watch(proxyID *pbresource.ID,
	nodeName string, token string) (<-chan proxycfg.ProxySnapshot,
	limiter.SessionTerminatedChan, proxycfg.CancelFunc, error) {
	pt.config.Logger.Trace("watch initiated", "proxyID", proxyID, "nodeName", nodeName)
	if err := pt.validateWatchArgs(proxyID, nodeName); err != nil {
		pt.config.Logger.Error("args failed validation", err)
		return nil, nil, nil, err
	}
	// Begin a session with the xDS session concurrency limiter.
	//
	// See: https://github.com/hashicorp/consul/issues/15753
	session, err := pt.config.SessionLimiter.BeginSession()
	if err != nil {
		pt.config.Logger.Error("failed to begin session with xDS session concurrency limiter", err)
		return nil, nil, nil, err
	}

	// This buffering is crucial otherwise we'd block immediately trying to
	// deliver the current snapshot below if we already have one.

	proxyStateChan := make(chan proxycfg.ProxySnapshot, 1)
	watchData := &proxyWatchData{
		notifyCh: proxyStateChan,
		state:    nil,
		token:    token,
		nodeName: nodeName,
	}

	proxyReferenceKey := resource.NewReferenceKey(proxyID)
	cancel := func() {
		pt.mu.Lock()
		defer pt.mu.Unlock()
		pt.cancelWatchLocked(proxyReferenceKey, proxyStateChan, session)
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.proxies[proxyReferenceKey] = watchData

	//Send an event to the controller
	err = pt.notifyNewProxyChannel(proxyID)
	if err != nil {
		pt.config.Logger.Error("failed to notify controller of new proxy connection", err)
		pt.cancelWatchLocked(proxyReferenceKey, watchData.notifyCh, session)
		return nil, nil, nil, err
	}
	pt.config.Logger.Trace("controller notified of watch created", "proxyID", proxyID, "nodeName", nodeName)

	return proxyStateChan, session.Terminated(), cancel, nil
}

// notifyNewProxyChannel attempts to send a message to newProxyConnectionCh and will return an error if there's no receiver.
// This will handle conditions where a proxy is connected but there's no controller for some reason to receive the event.
// This will error back to the proxy's Watch call and will cause the proxy call Watch again to retry connection until the controller
// is available.
func (pt *ProxyTracker) notifyNewProxyChannel(proxyID *pbresource.ID) error {
	controllerEvent := controller.Event{
		Obj: &ProxyConnection{
			ProxyID: proxyID,
		},
	}
	select {
	case pt.newProxyConnectionCh <- controllerEvent:
		return nil
		// using default here to return errors is only safe when we have a large buffer.
		// the receiver is on a loop to read from the channel.  If the sequence of
		// sender blocks on the channel and then the receiver blocks on the channel is not
		// aligned, then extraneous errors could be returned to the proxy that are just
		// false negatives and the controller could be up and healthy.
	default:
		return fmt.Errorf("failed to notify the controller of the proxy connecting")
	}
}

// cancelWatchLocked does the following:
// - deletes the key from the proxies array.
// - ends the session with xDS session limiter.
// - closes the proxy state channel assigned to the proxy.
// This function assumes the state lock is already held.
func (pt *ProxyTracker) cancelWatchLocked(proxyReferenceKey resource.ReferenceKey, proxyStateChan chan proxycfg.ProxySnapshot, session limiter.Session) {
	delete(pt.proxies, proxyReferenceKey)
	session.End()
	close(proxyStateChan)
	pt.config.Logger.Trace("watch cancelled", "proxyReferenceKey", proxyReferenceKey)
}

// validateWatchArgs checks the proxyIDand nodeName passed to Watch
// and returns an error if the args are not properly constructed.
func (pt *ProxyTracker) validateWatchArgs(proxyID *pbresource.ID,
	nodeName string) error {
	if proxyID == nil {
		return errors.New("proxyID is required")
	} else if proxyID.GetType().GetKind() != mesh.ProxyStateTemplateConfigurationType.Kind {
		return fmt.Errorf("proxyID must be a %s", mesh.ProxyStateTemplateConfigurationType.GetKind())
	} else if nodeName == "" {
		return errors.New("nodeName is required")
	}

	return nil
}

// PushChange allows pushing a computed ProxyState to xds for xds resource generation to send to a proxy.
func (pt *ProxyTracker) PushChange(proxyID *pbresource.ID, proxyState *mesh.ProxyState) error {
	pt.config.Logger.Trace("push change called for proxy", "proxyID", proxyID)
	proxyReferenceKey := resource.NewReferenceKey(proxyID)
	pt.mu.Lock()
	defer pt.mu.Unlock()
	if data, ok := pt.proxies[proxyReferenceKey]; ok {
		data.state = proxyState

		pt.deliverLatest(proxyID, proxyState, data.notifyCh)
	} else {
		return errors.New("proxyState change could not be sent because proxy is not connected")
	}

	return nil
}

func (pt *ProxyTracker) deliverLatest(proxyID *pbresource.ID, proxyState *mesh.ProxyState, ch chan proxycfg.ProxySnapshot) {
	pt.config.Logger.Trace("delivering latest proxy snapshot to proxy", "proxyID", proxyID)
	// Send if chan is empty
	select {
	case ch <- proxyState:
		return
	default:
	}

	// Not empty, drain the chan of older snapshots and redeliver. For now we only
	// use 1-buffered chans but this will still work if we change that later.
OUTER:
	for {
		select {
		case <-ch:
			continue
		default:
			break OUTER
		}
	}

	// Now send again
	select {
	case ch <- proxyState:
		return
	default:
		// This should not be possible since we should be the only sender, enforced
		// by m.mu but error and drop the update rather than panic.
		pt.config.Logger.Error("failed to deliver proxyState to proxy",
			"proxy", proxyID.String(),
		)
	}
}

// EventChannel returns an event channel that sends controller events when a proxy connects to a server.
func (pt *ProxyTracker) EventChannel() chan controller.Event {
	return pt.newProxyConnectionCh
}

// ShutdownChannel returns a channel that closes when ProxyTracker is shutdown. ShutdownChannel is never written to, only closed to
// indicate a shutdown has been initiated.
func (pt *ProxyTracker) ShutdownChannel() chan struct{} {
	return pt.shutdownCh
}

// ProxyConnectedToServer returns whether this id is connected to this server.
func (pt *ProxyTracker) ProxyConnectedToServer(proxyID *pbresource.ID) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	proxyReferenceKey := resource.NewReferenceKey(proxyID)
	_, ok := pt.proxies[proxyReferenceKey]
	return ok
}

// Shutdown removes all state and close all channels.
func (pt *ProxyTracker) Shutdown() {
	pt.config.Logger.Info("proxy tracker shutdown initiated")
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Close all current watchers first
	for proxyID, watchData := range pt.proxies {
		close(watchData.notifyCh)
		delete(pt.proxies, proxyID)
	}

	close(pt.newProxyConnectionCh)
	close(pt.shutdownCh)
}

//go:generate mockery --name SessionLimiter --inpackage
type SessionLimiter interface {
	BeginSession() (limiter.Session, error)
}

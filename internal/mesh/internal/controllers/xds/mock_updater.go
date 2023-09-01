// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	proxysnapshot "github.com/hashicorp/consul/internal/mesh/proxy-snapshot"
	proxytracker "github.com/hashicorp/consul/internal/mesh/proxy-tracker"
	"sync"

	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// mockUpdater mocks the updater functions, and stores ProxyStates from calls to PushChange, so we can assert it was
// computed correctly in the controller.
type mockUpdater struct {
	lock sync.Mutex
	// latestPs is a map from a ProxyStateTemplate's id.Name in string form to the last computed ProxyState for that
	// ProxyStateTemplate.
	latestPs        map[string]proxysnapshot.ProxySnapshot
	notConnected    bool
	pushChangeError bool
}

func newMockUpdater() *mockUpdater {
	return &mockUpdater{
		latestPs: make(map[string]proxysnapshot.ProxySnapshot),
	}
}

func (m *mockUpdater) SetPushChangeErrorTrue() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.pushChangeError = true
}

func (m *mockUpdater) SetProxyAsNotConnected() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.notConnected = true
}

func (m *mockUpdater) PushChange(id *pbresource.ID, snapshot proxysnapshot.ProxySnapshot) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.pushChangeError {
		return fmt.Errorf("mock push change error")
	} else {
		m.setUnsafe(id.Name, snapshot.(*proxytracker.ProxyState))
	}
	return nil
}

func (m *mockUpdater) ProxyConnectedToServer(_ *pbresource.ID) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.notConnected {
		return false
	}
	return true
}

func (p *mockUpdater) Get(name string) *proxytracker.ProxyState {
	p.lock.Lock()
	defer p.lock.Unlock()
	ps, ok := p.latestPs[name]
	if ok {
		return ps.(*proxytracker.ProxyState)
	}
	return nil
}

func (p *mockUpdater) GetEndpoints(name string) map[string]*pbproxystate.Endpoints {
	p.lock.Lock()
	defer p.lock.Unlock()
	ps, ok := p.latestPs[name]
	if ok {
		return ps.(*proxytracker.ProxyState).Endpoints
	}
	return nil
}

func (p *mockUpdater) GetTrustBundle(name string) map[string]*pbproxystate.TrustBundle {
	p.lock.Lock()
	defer p.lock.Unlock()
	ps, ok := p.latestPs[name]
	if ok {
		return ps.(*proxytracker.ProxyState).TrustBundles
	}
	return nil
}

func (p *mockUpdater) Set(name string, ps *proxytracker.ProxyState) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.setUnsafe(name, ps)
}

func (p *mockUpdater) setUnsafe(name string, ps *proxytracker.ProxyState) {
	p.latestPs[name] = ps
}

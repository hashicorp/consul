// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"sync"

	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// mockUpdater mocks the updater functions, and stores ProxyStates from calls to PushChange, so we can assert it was
// computed correctly in the controller.
type mockUpdater struct {
	lock sync.Mutex
	// latestPs is a map from a ProxyStateTemplate's id.Name in string form to the last computed ProxyState for that
	// ProxyStateTemplate.
	latestPs        map[string]*pbmesh.ProxyState
	notConnected    bool
	pushChangeError bool
}

func NewMockUpdater() *mockUpdater {
	return &mockUpdater{
		latestPs: make(map[string]*pbmesh.ProxyState),
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

func (m *mockUpdater) PushChange(id *pbresource.ID, snapshot *pbmesh.ProxyState) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.pushChangeError {
		return fmt.Errorf("mock push change error")
	} else {
		m.setUnsafe(id.Name, snapshot)
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

func (p *mockUpdater) Get(name string) *pbmesh.ProxyState {
	p.lock.Lock()
	defer p.lock.Unlock()
	ps, ok := p.latestPs[name]
	if ok {
		return ps
	}
	return nil
}

func (p *mockUpdater) GetEndpoints(name string) map[string]*pbproxystate.Endpoints {
	p.lock.Lock()
	defer p.lock.Unlock()
	ps, ok := p.latestPs[name]
	if ok {
		return ps.Endpoints
	}
	return nil
}

func (p *mockUpdater) GetTrustBundle(name string) map[string]*pbproxystate.TrustBundle {
	p.lock.Lock()
	defer p.lock.Unlock()
	ps, ok := p.latestPs[name]
	if ok {
		return ps.TrustBundles
	}
	return nil
}

func (p *mockUpdater) Set(name string, ps *pbmesh.ProxyState) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.setUnsafe(name, ps)
}

func (p *mockUpdater) setUnsafe(name string, ps *pbmesh.ProxyState) {
	p.latestPs[name] = ps
}

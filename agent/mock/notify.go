// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"fmt"
	"github.com/hashicorp/consul/agent/structs"
	"sync"
)

type Notify struct {
	updated chan int

	// A guard to protect an access to the internal attributes
	// of the notification mock in order to prevent panics
	// raised by the race conditions detector.
	sync.RWMutex
	state      map[structs.CheckID]string
	updates    map[structs.CheckID]int
	output     map[structs.CheckID]string
	serviceIDs map[structs.ServiceID]bool
}

func NewNotify() *Notify {
	return &Notify{
		state:      make(map[structs.CheckID]string),
		updates:    make(map[structs.CheckID]int),
		output:     make(map[structs.CheckID]string),
		serviceIDs: make(map[structs.ServiceID]bool),
	}
}

// ServiceExists mock
func (c *Notify) ServiceExists(serviceID structs.ServiceID) bool {
	return c.serviceIDs[serviceID]
}

// AddServiceID will mock a service being present locally
func (c *Notify) AddServiceID(serviceID structs.ServiceID) {
	c.serviceIDs[serviceID] = true
}

func NewNotifyChan() (*Notify, chan int) {
	n := &Notify{
		updated: make(chan int),
		state:   make(map[structs.CheckID]string),
		updates: make(map[structs.CheckID]int),
		output:  make(map[structs.CheckID]string),
	}
	return n, n.updated
}

func (m *Notify) sprintf(v interface{}) string {
	m.RLock()
	defer m.RUnlock()
	return fmt.Sprintf("%v", v)
}

func (m *Notify) StateMap() string   { return m.sprintf(m.state) }
func (m *Notify) UpdatesMap() string { return m.sprintf(m.updates) }
func (m *Notify) OutputMap() string  { return m.sprintf(m.output) }

func (m *Notify) UpdateCheck(id structs.CheckID, status, output string) {
	m.Lock()
	m.state[id] = status
	old := m.updates[id]
	m.updates[id] = old + 1
	m.output[id] = output
	m.Unlock()

	if m.updated != nil {
		m.updated <- 1
	}
}

// State returns the state of the specified health-check.
func (m *Notify) State(id structs.CheckID) string {
	m.RLock()
	defer m.RUnlock()
	return m.state[id]
}

// Updates returns the count of updates of the specified health-check.
func (m *Notify) Updates(id structs.CheckID) int {
	m.RLock()
	defer m.RUnlock()
	return m.updates[id]
}

// Output returns an output string of the specified health-check.
func (m *Notify) Output(id structs.CheckID) string {
	m.RLock()
	defer m.RUnlock()
	return m.output[id]
}

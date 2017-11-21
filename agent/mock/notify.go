package mock

import (
	"fmt"
	"sync"

	"github.com/hashicorp/consul/types"
)

type Notify struct {
	updated chan int

	// A guard to protect an access to the internal attributes
	// of the notification mock in order to prevent panics
	// raised by the race conditions detector.
	sync.RWMutex
	state   map[types.CheckID]string
	updates map[types.CheckID]int
	output  map[types.CheckID]string
}

func NewNotify() *Notify {
	return &Notify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
}

func NewNotifyChan() (*Notify, chan int) {
	n := &Notify{
		updated: make(chan int),
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
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

func (m *Notify) UpdateCheck(id types.CheckID, status, output string) {
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
func (m *Notify) State(id types.CheckID) string {
	m.RLock()
	defer m.RUnlock()
	return m.state[id]
}

// Updates returns the count of updates of the specified health-check.
func (m *Notify) Updates(id types.CheckID) int {
	m.RLock()
	defer m.RUnlock()
	return m.updates[id]
}

// Output returns an output string of the specified health-check.
func (m *Notify) Output(id types.CheckID) string {
	m.RLock()
	defer m.RUnlock()
	return m.output[id]
}

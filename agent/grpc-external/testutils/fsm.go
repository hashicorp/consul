// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutils

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/stretchr/testify/require"
)

func TestStateStore(t *testing.T, publisher state.EventPublisher) *state.Store {
	t.Helper()

	gc, err := state.NewTombstoneGC(time.Second, time.Millisecond)
	require.NoError(t, err)

	if publisher == nil {
		publisher = stream.NoOpEventPublisher{}
	}

	return state.NewStateStoreWithEventPublisher(gc, publisher)
}

type Registrar func(*FakeFSM, *stream.EventPublisher)

type FakeFSMConfig struct {
	Register  Registrar
	Refresh   []stream.Topic
	publisher *stream.EventPublisher
}

type FakeFSM struct {
	config FakeFSMConfig
	lock   sync.Mutex
	store  *state.Store
}

func newFakeFSM(t *testing.T, config FakeFSMConfig) *FakeFSM {
	t.Helper()

	store := TestStateStore(t, config.publisher)

	fsm := &FakeFSM{store: store, config: config}

	config.Register(fsm, fsm.config.publisher)

	return fsm
}

func (f *FakeFSM) GetStore() *state.Store {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.store
}

func (f *FakeFSM) ReplaceStore(store *state.Store) {
	f.lock.Lock()
	defer f.lock.Unlock()
	oldStore := f.store
	f.store = store
	oldStore.Abandon()
	for _, topic := range f.config.Refresh {
		f.config.publisher.RefreshTopic(topic)
	}
}

func SetupFSMAndPublisher(t *testing.T, config FakeFSMConfig) (*FakeFSM, state.EventPublisher) {
	t.Helper()
	config.publisher = stream.NewEventPublisher(10 * time.Second)

	fsm := newFakeFSM(t, config)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go config.publisher.Run(ctx)

	return fsm, config.publisher
}

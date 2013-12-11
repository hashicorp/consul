package consul

import (
	"github.com/hashicorp/raft"
	"io"
)

// consulFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type consulFSM struct {
	state *StateStore
}

// consulSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type consulSnapshot struct {
	fsm *consulFSM
}

// NewFSM is used to construct a new FSM with a blank state
func NewFSM() (*consulFSM, error) {
	state, err := NewStateStore()
	if err != nil {
		return nil, err
	}

	fsm := &consulFSM{
		state: state,
	}
	return fsm, nil
}

func (c *consulFSM) Apply([]byte) interface{} {
	// TODO: Decode

	// TODO: Execute
	return nil
}

func (c *consulFSM) Snapshot() (raft.FSMSnapshot, error) {
	snap := &consulSnapshot{fsm: c}
	return snap, nil
}

func (c *consulFSM) Restore(old io.ReadCloser) error {
	defer old.Close()

	// Create a new state store
	state, err := NewStateStore()
	if err != nil {
		return err
	}

	// TODO: Populate the new state

	// Do an atomic flip, safe since Apply is not called concurrently
	c.state = state
	return nil
}

func (s *consulSnapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (s *consulSnapshot) Release() {
}

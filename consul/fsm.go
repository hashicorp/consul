package consul

import (
	"github.com/hashicorp/raft"
	"io"
)

// consulFSM implements a finite state machine that is used
// along with Raft to provide strong consistency for various
// data that requires it. We implement this outside the Server
// to avoid exposing this outside the package.
type consulFSM struct {
	server *Server
}

// consulSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type consulSnapshot struct {
	fsm *consulFSM
}

func (c *consulFSM) Apply([]byte) interface{} {
	return nil
}

func (c *consulFSM) Snapshot() (raft.FSMSnapshot, error) {
	snap := &consulSnapshot{fsm: c}
	return snap, nil
}

func (c *consulFSM) Restore(io.ReadCloser) error {
	return nil
}

func (s *consulSnapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (s *consulSnapshot) Release() {
}

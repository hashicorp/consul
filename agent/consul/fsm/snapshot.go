package fsm

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
)

// snapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type snapshot struct {
	state *state.Snapshot
}

// snapshotHeader is the first entry in our snapshot
type snapshotHeader struct {
	// LastIndex is the last index that affects the data.
	// This is used when we do the restore for watchers.
	LastIndex uint64
}

// persister is a function used to help snapshot the FSM state.
type persister func(s *snapshot, sink raft.SnapshotSink, encoder *codec.Encoder) error

// persisters is a list of snapshot functions.
var persisters []persister

// registerPersister adds a new helper. This should be called at package
// init() time.
func registerPersister(fn persister) {
	persisters = append(persisters, fn)
}

// restorer is a function used to load back a snapshot of the FSM state.
type restorer func(header *snapshotHeader, restore *state.Restore, decoder *codec.Decoder) error

// restorers is a map of restore functions by message type.
var restorers map[structs.MessageType]restorer

// registerRestorer adds a new helper. This should be called at package
// init() time.
func registerRestorer(msg structs.MessageType, fn restorer) {
	if restorers == nil {
		restorers = make(map[structs.MessageType]restorer)
	}
	if restorers[msg] != nil {
		panic(fmt.Errorf("Message %d is already registered", msg))
	}
	restorers[msg] = fn
}

// Persist saves the FSM snapshot out to the given sink.
func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	defer metrics.MeasureSince([]string{"fsm", "persist"}, time.Now())

	// Write the header
	header := snapshotHeader{
		LastIndex: s.state.LastIndex(),
	}
	encoder := codec.NewEncoder(sink, msgpackHandle)
	if err := encoder.Encode(&header); err != nil {
		sink.Cancel()
		return err
	}

	// Run all the persisters to write the FSM state.
	for _, fn := range persisters {
		if err := fn(s, sink, encoder); err != nil {
			sink.Cancel()
			return err
		}
	}
	return nil
}

func (s *snapshot) Release() {
	s.state.Close()
}

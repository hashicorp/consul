package fsm

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
)

// msgpackHandle is a shared handle for encoding/decoding msgpack payloads
var msgpackHandle = &codec.MsgpackHandle{}

// command is a command method on the FSM.
type command func(buf []byte, index uint64) interface{}

// unboundCommand is a command method on the FSM, not yet bound to an FSM
// instance.
type unboundCommand func(c *FSM, buf []byte, index uint64) interface{}

// commands is a map from message type to unbound command.
var commands map[structs.MessageType]unboundCommand

// registerCommand registers a new command with the FSM, which should be done
// at package init() time.
func registerCommand(msg structs.MessageType, fn unboundCommand) {
	if commands == nil {
		commands = make(map[structs.MessageType]unboundCommand)
	}
	if commands[msg] != nil {
		panic(fmt.Errorf("Message %d is already registered", msg))
	}
	commands[msg] = fn
}

// FSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type FSM struct {
	logOutput io.Writer
	logger    *log.Logger
	path      string

	// apply is built off the commands global and is used to route apply
	// operations to their appropriate handlers.
	apply map[structs.MessageType]command

	// stateLock is only used to protect outside callers to State() from
	// racing with Restore(), which is called by Raft (it puts in a totally
	// new state store). Everything internal here is synchronized by the
	// Raft side, so doesn't need to lock this.
	stateLock sync.RWMutex
	state     *state.Store

	gc *state.TombstoneGC
}

// consulSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type consulSnapshot struct {
	state *state.Snapshot
}

// snapshotHeader is the first entry in our snapshot
type snapshotHeader struct {
	// LastIndex is the last index that affects the data.
	// This is used when we do the restore for watchers.
	LastIndex uint64
}

// New is used to construct a new FSM with a blank state.
func New(gc *state.TombstoneGC, logOutput io.Writer) (*FSM, error) {
	stateNew, err := state.NewStateStore(gc)
	if err != nil {
		return nil, err
	}

	fsm := &FSM{
		logOutput: logOutput,
		logger:    log.New(logOutput, "", log.LstdFlags),
		apply:     make(map[structs.MessageType]command),
		state:     stateNew,
		gc:        gc,
	}

	// Build out the apply dispatch table based on the registered commands.
	for msg, fn := range commands {
		thisFn := fn
		fsm.apply[msg] = func(buf []byte, index uint64) interface{} {
			return thisFn(fsm, buf, index)
		}
	}

	return fsm, nil
}

// State is used to return a handle to the current state
func (c *FSM) State() *state.Store {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.state
}

func (c *FSM) Apply(log *raft.Log) interface{} {
	buf := log.Data
	msgType := structs.MessageType(buf[0])

	// Check if this message type should be ignored when unknown. This is
	// used so that new commands can be added with developer control if older
	// versions can safely ignore the command, or if they should crash.
	ignoreUnknown := false
	if msgType&structs.IgnoreUnknownTypeFlag == structs.IgnoreUnknownTypeFlag {
		msgType &= ^structs.IgnoreUnknownTypeFlag
		ignoreUnknown = true
	}

	// Apply based on the dispatch table, if possible.
	if fn, ok := c.apply[msgType]; ok {
		return fn(buf[1:], log.Index)
	}

	// Otherwise, see if it's safe to ignore. If not, we have to panic so
	// that we crash and our state doesn't diverge.
	if ignoreUnknown {
		c.logger.Printf("[WARN] consul.fsm: ignoring unknown message type (%d), upgrade to newer version", msgType)
		return nil
	}
	panic(fmt.Errorf("failed to apply request: %#v", buf))
}

func (c *FSM) Snapshot() (raft.FSMSnapshot, error) {
	defer func(start time.Time) {
		c.logger.Printf("[INFO] consul.fsm: snapshot created in %v", time.Since(start))
	}(time.Now())

	return &consulSnapshot{c.state.Snapshot()}, nil
}

// Restore streams in the snapshot and replaces the current state store with a
// new one based on the snapshot if all goes OK during the restore.
func (c *FSM) Restore(old io.ReadCloser) error {
	defer old.Close()

	// Create a new state store.
	stateNew, err := state.NewStateStore(c.gc)
	if err != nil {
		return err
	}

	// Set up a new restore transaction
	restore := stateNew.Restore()
	defer restore.Abort()

	// Create a decoder
	dec := codec.NewDecoder(old, msgpackHandle)

	// Read in the header
	var header snapshotHeader
	if err := dec.Decode(&header); err != nil {
		return err
	}

	// Populate the new state
	msgType := make([]byte, 1)
	for {
		// Read the message type
		_, err := old.Read(msgType)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Decode
		switch structs.MessageType(msgType[0]) {
		case structs.RegisterRequestType:
			var req structs.RegisterRequest
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := restore.Registration(header.LastIndex, &req); err != nil {
				return err
			}

		case structs.KVSRequestType:
			var req structs.DirEntry
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := restore.KVS(&req); err != nil {
				return err
			}

		case structs.TombstoneRequestType:
			var req structs.DirEntry
			if err := dec.Decode(&req); err != nil {
				return err
			}

			// For historical reasons, these are serialized in the
			// snapshots as KV entries. We want to keep the snapshot
			// format compatible with pre-0.6 versions for now.
			stone := &state.Tombstone{
				Key:   req.Key,
				Index: req.ModifyIndex,
			}
			if err := restore.Tombstone(stone); err != nil {
				return err
			}

		case structs.SessionRequestType:
			var req structs.Session
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := restore.Session(&req); err != nil {
				return err
			}

		case structs.ACLRequestType:
			var req structs.ACL
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := restore.ACL(&req); err != nil {
				return err
			}

		case structs.ACLBootstrapRequestType:
			var req structs.ACLBootstrap
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := restore.ACLBootstrap(&req); err != nil {
				return err
			}

		case structs.CoordinateBatchUpdateType:
			var req structs.Coordinates
			if err := dec.Decode(&req); err != nil {
				return err

			}
			if err := restore.Coordinates(header.LastIndex, req); err != nil {
				return err
			}

		case structs.PreparedQueryRequestType:
			var req structs.PreparedQuery
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := restore.PreparedQuery(&req); err != nil {
				return err
			}

		case structs.AutopilotRequestType:
			var req structs.AutopilotConfig
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := restore.Autopilot(&req); err != nil {
				return err
			}

		default:
			return fmt.Errorf("Unrecognized msg type: %v", msgType)
		}
	}

	restore.Commit()

	// External code might be calling State(), so we need to synchronize
	// here to make sure we swap in the new state store atomically.
	c.stateLock.Lock()
	stateOld := c.state
	c.state = stateNew
	c.stateLock.Unlock()

	// Signal that the old state store has been abandoned. This is required
	// because we don't operate on it any more, we just throw it away, so
	// blocking queries won't see any changes and need to be woken up.
	stateOld.Abandon()
	return nil
}

func (s *consulSnapshot) Persist(sink raft.SnapshotSink) error {
	defer metrics.MeasureSince([]string{"consul", "fsm", "persist"}, time.Now())
	defer metrics.MeasureSince([]string{"fsm", "persist"}, time.Now())

	// Register the nodes
	encoder := codec.NewEncoder(sink, msgpackHandle)

	// Write the header
	header := snapshotHeader{
		LastIndex: s.state.LastIndex(),
	}
	if err := encoder.Encode(&header); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistNodes(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistSessions(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistACLs(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistKVs(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistTombstones(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistPreparedQueries(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistAutopilot(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	return nil
}

func (s *consulSnapshot) persistNodes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the nodes
	nodes, err := s.state.Nodes()
	if err != nil {
		return err
	}

	// Register each node
	for node := nodes.Next(); node != nil; node = nodes.Next() {
		n := node.(*structs.Node)
		req := structs.RegisterRequest{
			Node:            n.Node,
			Address:         n.Address,
			TaggedAddresses: n.TaggedAddresses,
		}

		// Register the node itself
		if _, err := sink.Write([]byte{byte(structs.RegisterRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(&req); err != nil {
			return err
		}

		// Register each service this node has
		services, err := s.state.Services(n.Node)
		if err != nil {
			return err
		}
		for service := services.Next(); service != nil; service = services.Next() {
			if _, err := sink.Write([]byte{byte(structs.RegisterRequestType)}); err != nil {
				return err
			}
			req.Service = service.(*structs.ServiceNode).ToNodeService()
			if err := encoder.Encode(&req); err != nil {
				return err
			}
		}

		// Register each check this node has
		req.Service = nil
		checks, err := s.state.Checks(n.Node)
		if err != nil {
			return err
		}
		for check := checks.Next(); check != nil; check = checks.Next() {
			if _, err := sink.Write([]byte{byte(structs.RegisterRequestType)}); err != nil {
				return err
			}
			req.Check = check.(*structs.HealthCheck)
			if err := encoder.Encode(&req); err != nil {
				return err
			}
		}
	}

	// Save the coordinates separately since they are not part of the
	// register request interface. To avoid copying them out, we turn
	// them into batches with a single coordinate each.
	coords, err := s.state.Coordinates()
	if err != nil {
		return err
	}
	for coord := coords.Next(); coord != nil; coord = coords.Next() {
		if _, err := sink.Write([]byte{byte(structs.CoordinateBatchUpdateType)}); err != nil {
			return err
		}
		updates := structs.Coordinates{coord.(*structs.Coordinate)}
		if err := encoder.Encode(&updates); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistSessions(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	sessions, err := s.state.Sessions()
	if err != nil {
		return err
	}

	for session := sessions.Next(); session != nil; session = sessions.Next() {
		if _, err := sink.Write([]byte{byte(structs.SessionRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(session.(*structs.Session)); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistACLs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	acls, err := s.state.ACLs()
	if err != nil {
		return err
	}

	for acl := acls.Next(); acl != nil; acl = acls.Next() {
		if _, err := sink.Write([]byte{byte(structs.ACLRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(acl.(*structs.ACL)); err != nil {
			return err
		}
	}

	bs, err := s.state.ACLBootstrap()
	if err != nil {
		return err
	}
	if bs != nil {
		if _, err := sink.Write([]byte{byte(structs.ACLBootstrapRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(bs); err != nil {
			return err
		}
	}

	return nil
}

func (s *consulSnapshot) persistKVs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	entries, err := s.state.KVs()
	if err != nil {
		return err
	}

	for entry := entries.Next(); entry != nil; entry = entries.Next() {
		if _, err := sink.Write([]byte{byte(structs.KVSRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(entry.(*structs.DirEntry)); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistTombstones(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	stones, err := s.state.Tombstones()
	if err != nil {
		return err
	}

	for stone := stones.Next(); stone != nil; stone = stones.Next() {
		if _, err := sink.Write([]byte{byte(structs.TombstoneRequestType)}); err != nil {
			return err
		}

		// For historical reasons, these are serialized in the snapshots
		// as KV entries. We want to keep the snapshot format compatible
		// with pre-0.6 versions for now.
		s := stone.(*state.Tombstone)
		fake := &structs.DirEntry{
			Key: s.Key,
			RaftIndex: structs.RaftIndex{
				ModifyIndex: s.Index,
			},
		}
		if err := encoder.Encode(fake); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistPreparedQueries(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	queries, err := s.state.PreparedQueries()
	if err != nil {
		return err
	}

	for _, query := range queries {
		if _, err := sink.Write([]byte{byte(structs.PreparedQueryRequestType)}); err != nil {
			return err
		}
		if err := encoder.Encode(query); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistAutopilot(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	autopilot, err := s.state.Autopilot()
	if err != nil {
		return err
	}
	if autopilot == nil {
		return nil
	}

	if _, err := sink.Write([]byte{byte(structs.AutopilotRequestType)}); err != nil {
		return err
	}
	if err := encoder.Encode(autopilot); err != nil {
		return err
	}

	return nil
}

func (s *consulSnapshot) Release() {
	s.state.Close()
}

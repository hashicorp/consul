package consul

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/consul/state"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
)

// msgpackHandle is a shared handle for encoding/decoding msgpack payloads
var msgpackHandle = &codec.MsgpackHandle{}

// consulFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type consulFSM struct {
	logOutput io.Writer
	logger    *log.Logger
	path      string
	stateNew  *state.StateStore
	state     *StateStore
	gc        *state.TombstoneGC
}

// consulSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type consulSnapshot struct {
	state    *StateSnapshot
	stateNew *state.StateSnapshot
}

// snapshotHeader is the first entry in our snapshot
type snapshotHeader struct {
	// LastIndex is the last index that affects the data.
	// This is used when we do the restore for watchers.
	LastIndex uint64
}

// NewFSMPath is used to construct a new FSM with a blank state
func NewFSM(gc *state.TombstoneGC, path string, logOutput io.Writer) (*consulFSM, error) {
	// Create the state store.
	stateNew, err := state.NewStateStore(gc)
	if err != nil {
		return nil, err
	}

	// Create a temporary path for the state store
	tmpPath, err := ioutil.TempDir(path, "state")
	if err != nil {
		return nil, err
	}

	// Create a state store
	state, err := NewStateStorePath(gc, tmpPath, logOutput)
	if err != nil {
		return nil, err
	}

	fsm := &consulFSM{
		logOutput: logOutput,
		logger:    log.New(logOutput, "", log.LstdFlags),
		path:      path,
		stateNew:  stateNew,
		state:     state,
		gc:        gc,
	}
	return fsm, nil
}

// Close is used to cleanup resources associated with the FSM
func (c *consulFSM) Close() error {
	return c.state.Close()
}

// StateNew is used to return a handle to the current state
func (c *consulFSM) StateNew() *state.StateStore {
	return c.stateNew
}

// State is used to return a handle to the current state
func (c *consulFSM) State() *StateStore {
	return c.state
}

func (c *consulFSM) Apply(log *raft.Log) interface{} {
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

	switch msgType {
	case structs.RegisterRequestType:
		return c.decodeRegister(buf[1:], log.Index)
	case structs.DeregisterRequestType:
		return c.applyDeregister(buf[1:], log.Index)
	case structs.KVSRequestType:
		return c.applyKVSOperation(buf[1:], log.Index)
	case structs.SessionRequestType:
		return c.applySessionOperation(buf[1:], log.Index)
	case structs.ACLRequestType:
		return c.applyACLOperation(buf[1:], log.Index)
	case structs.TombstoneRequestType:
		return c.applyTombstoneOperation(buf[1:], log.Index)
	default:
		if ignoreUnknown {
			c.logger.Printf("[WARN] consul.fsm: ignoring unknown message type (%d), upgrade to newer version", msgType)
			return nil
		} else {
			panic(fmt.Errorf("failed to apply request: %#v", buf))
		}
	}
}

func (c *consulFSM) decodeRegister(buf []byte, index uint64) interface{} {
	var req structs.RegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	return c.applyRegister(&req, index)
}

func (c *consulFSM) applyRegister(req *structs.RegisterRequest, index uint64) interface{} {
	// Apply all updates in a single transaction
	defer metrics.MeasureSince([]string{"consul", "fsm", "register"}, time.Now())
	if err := c.stateNew.EnsureRegistration(index, req); err != nil {
		c.logger.Printf("[INFO] consul.fsm: EnsureRegistration failed: %v", err)
		return err
	}
	return nil
}

func (c *consulFSM) applyDeregister(buf []byte, index uint64) interface{} {
	defer metrics.MeasureSince([]string{"consul", "fsm", "deregister"}, time.Now())
	var req structs.DeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Either remove the service entry or the whole node
	if req.ServiceID != "" {
		if err := c.stateNew.DeleteService(index, req.Node, req.ServiceID); err != nil {
			c.logger.Printf("[INFO] consul.fsm: DeleteNodeService failed: %v", err)
			return err
		}
	} else if req.CheckID != "" {
		if err := c.stateNew.DeleteCheck(index, req.Node, req.CheckID); err != nil {
			c.logger.Printf("[INFO] consul.fsm: DeleteNodeCheck failed: %v", err)
			return err
		}
	} else {
		if err := c.stateNew.DeleteNode(index, req.Node); err != nil {
			c.logger.Printf("[INFO] consul.fsm: DeleteNode failed: %v", err)
			return err
		}
	}
	return nil
}

func (c *consulFSM) applyKVSOperation(buf []byte, index uint64) interface{} {
	var req structs.KVSRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"consul", "fsm", "kvs", string(req.Op)}, time.Now())
	switch req.Op {
	case structs.KVSSet:
		return c.stateNew.KVSSet(index, &req.DirEnt)
	case structs.KVSDelete:
		return c.stateNew.KVSDelete(index, req.DirEnt.Key)
	case structs.KVSDeleteCAS:
		act, err := c.stateNew.KVSDeleteCAS(index, req.DirEnt.ModifyIndex, req.DirEnt.Key)
		if err != nil {
			return err
		} else {
			return act
		}
	case structs.KVSDeleteTree:
		return c.stateNew.KVSDeleteTree(index, req.DirEnt.Key)
	case structs.KVSCAS:
		act, err := c.stateNew.KVSSetCAS(index, &req.DirEnt)
		if err != nil {
			return err
		} else {
			return act
		}
	case structs.KVSLock:
		act, err := c.stateNew.KVSLock(index, &req.DirEnt)
		if err != nil {
			return err
		} else {
			return act
		}
	case structs.KVSUnlock:
		act, err := c.stateNew.KVSUnlock(index, &req.DirEnt)
		if err != nil {
			return err
		} else {
			return act
		}
	default:
		err := errors.New(fmt.Sprintf("Invalid KVS operation '%s'", req.Op))
		c.logger.Printf("[WARN] consul.fsm: %v", err)
		return err
	}
}

func (c *consulFSM) applySessionOperation(buf []byte, index uint64) interface{} {
	var req structs.SessionRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"consul", "fsm", "session", string(req.Op)}, time.Now())
	switch req.Op {
	case structs.SessionCreate:
		if err := c.stateNew.SessionCreate(index, &req.Session); err != nil {
			return err
		} else {
			return req.Session.ID
		}
	case structs.SessionDestroy:
		return c.stateNew.SessionDestroy(index, req.Session.ID)
	default:
		c.logger.Printf("[WARN] consul.fsm: Invalid Session operation '%s'", req.Op)
		return fmt.Errorf("Invalid Session operation '%s'", req.Op)
	}
}

func (c *consulFSM) applyACLOperation(buf []byte, index uint64) interface{} {
	var req structs.ACLRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"consul", "fsm", "acl", string(req.Op)}, time.Now())
	switch req.Op {
	case structs.ACLForceSet, structs.ACLSet:
		if err := c.stateNew.ACLSet(index, &req.ACL); err != nil {
			return err
		} else {
			return req.ACL.ID
		}
	case structs.ACLDelete:
		return c.stateNew.ACLDelete(index, req.ACL.ID)
	default:
		c.logger.Printf("[WARN] consul.fsm: Invalid ACL operation '%s'", req.Op)
		return fmt.Errorf("Invalid ACL operation '%s'", req.Op)
	}
}

func (c *consulFSM) applyTombstoneOperation(buf []byte, index uint64) interface{} {
	var req structs.TombstoneRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	defer metrics.MeasureSince([]string{"consul", "fsm", "tombstone", string(req.Op)}, time.Now())
	switch req.Op {
	case structs.TombstoneReap:
		return c.stateNew.ReapTombstones(req.ReapIndex)
	default:
		c.logger.Printf("[WARN] consul.fsm: Invalid Tombstone operation '%s'", req.Op)
		return fmt.Errorf("Invalid Tombstone operation '%s'", req.Op)
	}
}

func (c *consulFSM) Snapshot() (raft.FSMSnapshot, error) {
	defer func(start time.Time) {
		c.logger.Printf("[INFO] consul.fsm: snapshot created in %v", time.Now().Sub(start))
	}(time.Now())

	// Create a new snapshot
	snap, err := c.state.Snapshot()
	if err != nil {
		return nil, err
	}
	return &consulSnapshot{snap, c.stateNew.Snapshot()}, nil
}

func (c *consulFSM) Restore(old io.ReadCloser) error {
	defer old.Close()

	// Create a temporary path for the state store
	tmpPath, err := ioutil.TempDir(c.path, "state")
	if err != nil {
		return err
	}

	// Create a new state store
	store, err := NewStateStorePath(c.gc, tmpPath, c.logOutput)
	if err != nil {
		return err
	}
	c.state.Close()
	c.state = store

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
			c.applyRegister(&req, header.LastIndex)

		case structs.KVSRequestType:
			var req structs.DirEntry
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := c.stateNew.KVSRestore(&req); err != nil {
				return err
			}

		case structs.SessionRequestType:
			var req structs.Session
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := c.stateNew.SessionRestore(&req); err != nil {
				return err
			}

		case structs.ACLRequestType:
			var req structs.ACL
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := c.stateNew.ACLRestore(&req); err != nil {
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
			if err := c.stateNew.TombstoneRestore(stone); err != nil {
				return err
			}

		default:
			return fmt.Errorf("Unrecognized msg type: %v", msgType)
		}
	}

	return nil
}

func (s *consulSnapshot) Persist(sink raft.SnapshotSink) error {
	defer metrics.MeasureSince([]string{"consul", "fsm", "persist"}, time.Now())
	// Register the nodes
	encoder := codec.NewEncoder(sink, msgpackHandle)

	// Write the header
	header := snapshotHeader{
		LastIndex: s.stateNew.LastIndex(),
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

	if err := s.persistKV(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}

	if err := s.persistTombstones(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (s *consulSnapshot) persistNodes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the nodes
	nodes, err := s.stateNew.NodeDump()
	if err != nil {
		return err
	}

	// Register each node
	var req structs.RegisterRequest
	for i := 0; i < len(nodes); i++ {
		req = structs.RegisterRequest{
			Node:    nodes[i].Node,
			Address: nodes[i].Address,
		}

		// Register the node itself
		sink.Write([]byte{byte(structs.RegisterRequestType)})
		if err := encoder.Encode(&req); err != nil {
			return err
		}

		// Register each service this node has
		services, err := s.stateNew.ServiceDump(nodes[i].Node)
		if err != nil {
			return err
		}
		for _, srv := range services {
			req.Service = srv
			sink.Write([]byte{byte(structs.RegisterRequestType)})
			if err := encoder.Encode(&req); err != nil {
				return err
			}
		}

		// Register each check this node has
		req.Service = nil
		checks, err := s.stateNew.CheckDump(nodes[i].Node)
		if err != nil {
			return err
		}
		for _, check := range checks {
			req.Check = check
			sink.Write([]byte{byte(structs.RegisterRequestType)})
			if err := encoder.Encode(&req); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *consulSnapshot) persistSessions(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	sessions, err := s.stateNew.SessionDump()
	if err != nil {
		return err
	}

	for _, s := range sessions {
		sink.Write([]byte{byte(structs.SessionRequestType)})
		if err := encoder.Encode(s); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistACLs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	acls, err := s.stateNew.ACLDump()
	if err != nil {
		return err
	}

	for _, s := range acls {
		sink.Write([]byte{byte(structs.ACLRequestType)})
		if err := encoder.Encode(s); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistKV(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	entries, err := s.stateNew.KVSDump()
	if err != nil {
		return err
	}

	for _, e := range entries {
		sink.Write([]byte{byte(structs.KVSRequestType)})
		if err := encoder.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

func (s *consulSnapshot) persistTombstones(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	stones, err := s.stateNew.TombstoneDump()
	if err != nil {
		return err
	}

	for _, s := range stones {
		sink.Write([]byte{byte(structs.TombstoneRequestType)})

		// For historical reasons, these are serialized in the snapshots
		// as KV entries. We want to keep the snapshot format compatible
		// with pre-0.6 versions for now.
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

func (s *consulSnapshot) Release() {
	s.state.Close()
	s.stateNew.Close()
}

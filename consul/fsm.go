package consul

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/raft"
)

// consulFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type consulFSM struct {
	logOutput io.Writer
	logger    *log.Logger
	path      string
	state     *StateStore
	gc        *TombstoneGC
}

// consulSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type consulSnapshot struct {
	state *StateSnapshot
}

// snapshotHeader is the first entry in our snapshot
type snapshotHeader struct {
	// LastIndex is the last index that affects the data.
	// This is used when we do the restore for watchers.
	LastIndex uint64
}

// NewFSMPath is used to construct a new FSM with a blank state
func NewFSM(gc *TombstoneGC, path string, logOutput io.Writer) (*consulFSM, error) {
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
		state:     state,
		gc:        gc,
	}
	return fsm, nil
}

// Close is used to cleanup resources associated with the FSM
func (c *consulFSM) Close() error {
	return c.state.Close()
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
	defer metrics.MeasureSince([]string{"consul", "fsm", "register"}, time.Now())
	// Apply all updates in a single transaction
	if err := c.state.EnsureRegistration(index, req); err != nil {
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
		if err := c.state.DeleteNodeService(index, req.Node, req.ServiceID); err != nil {
			c.logger.Printf("[INFO] consul.fsm: DeleteNodeService failed: %v", err)
			return err
		}
	} else if req.CheckID != "" {
		if err := c.state.DeleteNodeCheck(index, req.Node, req.CheckID); err != nil {
			c.logger.Printf("[INFO] consul.fsm: DeleteNodeCheck failed: %v", err)
			return err
		}
	} else {
		if err := c.state.DeleteNode(index, req.Node); err != nil {
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
		return c.state.KVSSet(index, &req.DirEnt)
	case structs.KVSDelete:
		return c.state.KVSDelete(index, req.DirEnt.Key)
	case structs.KVSDeleteCAS:
		act, err := c.state.KVSDeleteCheckAndSet(index, req.DirEnt.Key, req.DirEnt.ModifyIndex)
		if err != nil {
			return err
		} else {
			return act
		}
	case structs.KVSDeleteTree:
		return c.state.KVSDeleteTree(index, req.DirEnt.Key)
	case structs.KVSCAS:
		act, err := c.state.KVSCheckAndSet(index, &req.DirEnt)
		if err != nil {
			return err
		} else {
			return act
		}
	case structs.KVSLock:
		act, err := c.state.KVSLock(index, &req.DirEnt)
		if err != nil {
			return err
		} else {
			return act
		}
	case structs.KVSUnlock:
		act, err := c.state.KVSUnlock(index, &req.DirEnt)
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
		if err := c.state.SessionCreate(index, &req.Session); err != nil {
			return err
		} else {
			return req.Session.ID
		}
	case structs.SessionDestroy:
		return c.state.SessionDestroy(index, req.Session.ID)
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
	case structs.ACLForceSet:
		fallthrough
	case structs.ACLSet:
		if err := c.state.ACLSet(index, &req.ACL); err != nil {
			return err
		} else {
			return req.ACL.ID
		}
	case structs.ACLDelete:
		return c.state.ACLDelete(index, req.ACL.ID)
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
		return c.state.ReapTombstones(req.ReapIndex)
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
	return &consulSnapshot{snap}, nil
}

func (c *consulFSM) Restore(old io.ReadCloser) error {
	defer old.Close()

	// Create a temporary path for the state store
	tmpPath, err := ioutil.TempDir(c.path, "state")
	if err != nil {
		return err
	}

	// Create a new state store
	state, err := NewStateStorePath(c.gc, tmpPath, c.logOutput)
	if err != nil {
		return err
	}
	c.state.Close()
	c.state = state

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
			if err := c.state.KVSRestore(&req); err != nil {
				return err
			}

		case structs.SessionRequestType:
			var req structs.Session
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := c.state.SessionRestore(&req); err != nil {
				return err
			}

		case structs.ACLRequestType:
			var req structs.ACL
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := c.state.ACLRestore(&req); err != nil {
				return err
			}

		case structs.TombstoneRequestType:
			var req structs.DirEntry
			if err := dec.Decode(&req); err != nil {
				return err
			}
			if err := c.state.TombstoneRestore(&req); err != nil {
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
	nodes := s.state.Nodes()

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
		services := s.state.NodeServices(nodes[i].Node)
		for _, srv := range services.Services {
			req.Service = srv
			sink.Write([]byte{byte(structs.RegisterRequestType)})
			if err := encoder.Encode(&req); err != nil {
				return err
			}
		}

		// Register each check this node has
		req.Service = nil
		checks := s.state.NodeChecks(nodes[i].Node)
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
	sessions, err := s.state.SessionList()
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
	acls, err := s.state.ACLList()
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
	streamCh := make(chan interface{}, 256)
	errorCh := make(chan error)
	go func() {
		if err := s.state.KVSDump(streamCh); err != nil {
			errorCh <- err
		}
	}()

	for {
		select {
		case raw := <-streamCh:
			if raw == nil {
				return nil
			}
			sink.Write([]byte{byte(structs.KVSRequestType)})
			if err := encoder.Encode(raw); err != nil {
				return err
			}

		case err := <-errorCh:
			return err
		}
	}
}

func (s *consulSnapshot) persistTombstones(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	streamCh := make(chan interface{}, 256)
	errorCh := make(chan error)
	go func() {
		if err := s.state.TombstoneDump(streamCh); err != nil {
			errorCh <- err
		}
	}()

	for {
		select {
		case raw := <-streamCh:
			if raw == nil {
				return nil
			}
			sink.Write([]byte{byte(structs.TombstoneRequestType)})
			if err := encoder.Encode(raw); err != nil {
				return err
			}

		case err := <-errorCh:
			return err
		}
	}
}

func (s *consulSnapshot) Release() {
	s.state.Close()
}

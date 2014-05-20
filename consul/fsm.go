package consul

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/raft"
	"github.com/ugorji/go/codec"
	"io"
	"log"
	"time"
)

// consulFSM implements a finite state machine that is used
// along with Raft to provide strong consistency. We implement
// this outside the Server to avoid exposing this outside the package.
type consulFSM struct {
	logOutput io.Writer
	logger    *log.Logger
	state     *StateStore
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

// NewFSM is used to construct a new FSM with a blank state
func NewFSM(logOutput io.Writer) (*consulFSM, error) {
	state, err := NewStateStore(logOutput)
	if err != nil {
		return nil, err
	}

	fsm := &consulFSM{
		logOutput: logOutput,
		logger:    log.New(logOutput, "", log.LstdFlags),
		state:     state,
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
	switch structs.MessageType(buf[0]) {
	case structs.RegisterRequestType:
		return c.decodeRegister(buf[1:], log.Index)
	case structs.DeregisterRequestType:
		return c.applyDeregister(buf[1:], log.Index)
	case structs.KVSRequestType:
		return c.applyKVSOperation(buf[1:], log.Index)
	case structs.SessionRequestType:
		return c.applySessionOperation(buf[1:], log.Index)
	default:
		panic(fmt.Errorf("failed to apply request: %#v", buf))
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
	// Ensure the node
	node := structs.Node{req.Node, req.Address}
	if err := c.state.EnsureNode(index, node); err != nil {
		c.logger.Printf("[INFO] consul.fsm: EnsureNode failed: %v", err)
		return err
	}

	// Ensure the service if provided
	if req.Service != nil {
		if err := c.state.EnsureService(index, req.Node, req.Service); err != nil {
			c.logger.Printf("[INFO] consul.fsm: EnsureService failed: %v", err)
			return err
		}
	}

	// Ensure the check if provided
	if req.Check != nil {
		if err := c.state.EnsureCheck(index, req.Check); err != nil {
			c.logger.Printf("[INFO] consul.fsm: EnsureCheck failed: %v", err)
			return err
		}
	}

	return nil
}

func (c *consulFSM) applyDeregister(buf []byte, index uint64) interface{} {
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
	switch req.Op {
	case structs.KVSSet:
		return c.state.KVSSet(index, &req.DirEnt)
	case structs.KVSDelete:
		return c.state.KVSDelete(index, req.DirEnt.Key)
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
		c.logger.Printf("[WARN] consul.fsm: Invalid KVS operation '%s'", req.Op)
		return fmt.Errorf("Invalid KVS operation '%s'", req.Op)
	}
	return nil
}

func (c *consulFSM) applySessionOperation(buf []byte, index uint64) interface{} {
	var req structs.SessionRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
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
	return nil
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

	// Create a new state store
	state, err := NewStateStore(c.logOutput)
	if err != nil {
		return err
	}
	c.state.Close()
	c.state = state

	// Create a decoder
	var handle codec.MsgpackHandle
	dec := codec.NewDecoder(old, &handle)

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

		default:
			return fmt.Errorf("Unrecognized msg type: %v", msgType)
		}
	}

	return nil
}

func (s *consulSnapshot) Persist(sink raft.SnapshotSink) error {
	// Register the nodes
	handle := codec.MsgpackHandle{}
	encoder := codec.NewEncoder(sink, &handle)

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

	if err := s.persistKV(sink, encoder); err != nil {
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
	return nil
}

func (s *consulSnapshot) Release() {
	s.state.Close()
}

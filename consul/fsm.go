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
	state *StateStore
}

// consulSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type consulSnapshot struct {
	state *StateSnapshot
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

// State is used to return a handle to the current state
func (c *consulFSM) State() *StateStore {
	return c.state
}

func (c *consulFSM) Apply(log *raft.Log) interface{} {
	buf := log.Data
	switch structs.MessageType(buf[0]) {
	case structs.RegisterRequestType:
		return c.decodeRegister(buf[1:])
	case structs.DeregisterRequestType:
		return c.applyDeregister(buf[1:])
	default:
		panic(fmt.Errorf("failed to apply request: %#v", buf))
	}
}

func (c *consulFSM) decodeRegister(buf []byte) interface{} {
	var req structs.RegisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}
	return c.applyRegister(&req)
}

func (c *consulFSM) applyRegister(req *structs.RegisterRequest) interface{} {
	// Ensure the node
	node := structs.Node{req.Node, req.Address}
	c.state.EnsureNode(node)

	// Ensure the service if provided
	if req.Service != nil {
		c.state.EnsureService(req.Node, req.Service.ID, req.Service.Service,
			req.Service.Tag, req.Service.Port)
	}

	// Ensure the check if provided
	if req.Check != nil {
		c.state.EnsureCheck(req.Check)
	}

	return nil
}

func (c *consulFSM) applyDeregister(buf []byte) interface{} {
	var req structs.DeregisterRequest
	if err := structs.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Either remove the service entry or the whole node
	if req.ServiceID != "" {
		c.state.DeleteNodeService(req.Node, req.ServiceID)
	} else if req.CheckID != "" {
		c.state.DeleteNodeCheck(req.Node, req.CheckID)
	} else {
		c.state.DeleteNode(req.Node)
	}
	return nil
}

func (c *consulFSM) Snapshot() (raft.FSMSnapshot, error) {
	defer func(start time.Time) {
		log.Printf("[INFO] consul: FSM snapshot created in %v", time.Now().Sub(start))
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
	state, err := NewStateStore()
	if err != nil {
		return err
	}
	c.state = state

	// Create a decoder
	var handle codec.MsgpackHandle
	dec := codec.NewDecoder(old, &handle)

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
			c.applyRegister(&req)

		default:
			return fmt.Errorf("Unrecognized msg type: %v", msgType)
		}
	}

	return nil
}

func (s *consulSnapshot) Persist(sink raft.SnapshotSink) error {
	// Get all the nodes
	nodes := s.state.Nodes()

	// Register the nodes
	handle := codec.MsgpackHandle{}
	encoder := codec.NewEncoder(sink, &handle)

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
			sink.Cancel()
			return err
		}

		// Register each service this node has
		services := s.state.NodeServices(nodes[i].Node)
		for _, srv := range services.Services {
			req.Service = srv
			sink.Write([]byte{byte(structs.RegisterRequestType)})
			if err := encoder.Encode(&req); err != nil {
				sink.Cancel()
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
				sink.Cancel()
				return err
			}
		}
	}
	return nil
}

func (s *consulSnapshot) Release() {
	s.state.Close()
}

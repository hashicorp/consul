package consul

import (
	"fmt"
	"github.com/hashicorp/consul/rpc"
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

func (c *consulFSM) Apply(buf []byte) interface{} {
	switch rpc.MessageType(buf[0]) {
	case rpc.RegisterRequestType:
		return c.applyRegister(buf[1:])
	case rpc.DeregisterRequestType:
		return c.applyDeregister(buf[1:])
	default:
		panic(fmt.Errorf("failed to apply request: %#v", buf))
	}
}

func (c *consulFSM) applyRegister(buf []byte) interface{} {
	var req rpc.RegisterRequest
	if err := rpc.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Ensure the node
	c.state.EnsureNode(req.Node, req.Address)

	// Ensure the service if provided
	if req.ServiceName != "" {
		c.state.EnsureService(req.Node, req.ServiceName, req.ServiceTag, req.ServicePort)
	}
	return nil
}

func (c *consulFSM) applyDeregister(buf []byte) interface{} {
	var req rpc.DeregisterRequest
	if err := rpc.Decode(buf, &req); err != nil {
		panic(fmt.Errorf("failed to decode request: %v", err))
	}

	// Either remove the service entry or the whole node
	if req.ServiceName != "" {
		c.state.DeleteNodeService(req.Node, req.ServiceName)
	} else {
		c.state.DeleteNode(req.Node)
	}
	return nil
}

func (c *consulFSM) Snapshot() (raft.FSMSnapshot, error) {
	defer func(start time.Time) {
		log.Printf("[INFO] FSM Snapshot created in %v", time.Now().Sub(start))
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
		switch rpc.MessageType(msgType[0]) {
		case rpc.RegisterRequestType:
			var req rpc.RegisterRequest
			if err := dec.Decode(&req); err != nil {
				return err
			}

			// Register the service or the node
			if req.ServiceName != "" {
				state.EnsureService(req.Node, req.ServiceName,
					req.ServiceTag, req.ServicePort)
			} else {
				state.EnsureNode(req.Node, req.Address)
			}

		default:
			return fmt.Errorf("Unrecognized msg type: %v", msgType)
		}
	}

	// Do an atomic flip, safe since Apply is not called concurrently
	c.state = state
	return nil
}

func (s *consulSnapshot) Persist(sink raft.SnapshotSink) error {
	// Get all the nodes
	nodes := s.state.Nodes()

	// Register the nodes
	handle := codec.MsgpackHandle{}
	encoder := codec.NewEncoder(sink, &handle)

	// Register each node
	var req rpc.RegisterRequest
	for i := 0; i < len(nodes); i += 2 {
		req = rpc.RegisterRequest{
			Node:    nodes[i],
			Address: nodes[i+1],
		}

		// Register the node itself
		sink.Write([]byte{byte(rpc.RegisterRequestType)})
		if err := encoder.Encode(&req); err != nil {
			sink.Cancel()
			return err
		}

		// Register each service this node has
		services := s.state.NodeServices(nodes[i])
		for serv, props := range services {
			req.ServiceName = serv
			req.ServiceTag = props.Tag
			req.ServicePort = props.Port

			sink.Write([]byte{byte(rpc.RegisterRequestType)})
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

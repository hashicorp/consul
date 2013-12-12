package rpc

import (
	"bytes"
	"fmt"
	"github.com/ugorji/go/codec"
)

var (
	ErrNoLeader = fmt.Errorf("No cluster leader")
	ErrNoDCPath = fmt.Errorf("No path to datacenter")
)

type MessageType uint8

const (
	RegisterRequestType MessageType = iota
	DeregisterRequestType
)

// RegisterRequest is used for the Catalog.Register endpoint
// to register a node as providing a service. If no service
// is provided, the node is registered.
type RegisterRequest struct {
	Datacenter  string
	Node        string
	Address     string
	ServiceName string
	ServiceTag  string
	ServicePort int
}

// DeregisterRequest is used for the Catalog.Deregister endpoint
// to deregister a node as providing a service. If no service is
// provided the entire node is deregistered.
type DeregisterRequest struct {
	Datacenter  string
	Node        string
	ServiceName string
}

// Used to return information about a node
type Node struct {
	Node    string
	Address string
}
type Nodes []Node

// Used to return information about a provided services.
// Maps service name to available tags
type Services map[string][]string

// ServiceNodesRequest is used to query the nodes of a service
type ServiceNodesRequest struct {
	Datacenter  string
	ServiceName string
	ServiceTag  string
	TagFilter   bool // Controls tag filtering
}

// ServiceNode represents a node that is part of a service
type ServiceNode struct {
	Node        string
	Address     string
	ServiceTag  string
	ServicePort int
}
type ServiceNodes []ServiceNode

// Decode is used to decode a MsgPack encoded object
func Decode(buf []byte, out interface{}) error {
	var handle codec.MsgpackHandle
	return codec.NewDecoder(bytes.NewReader(buf), &handle).Decode(out)
}

// Encode is used to encode a MsgPack object with type prefix
func Encode(t MessageType, msg interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(uint8(t))

	handle := codec.MsgpackHandle{}
	encoder := codec.NewEncoder(buf, &handle)
	err := encoder.Encode(msg)
	return buf.Bytes(), err
}

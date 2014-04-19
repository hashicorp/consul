package structs

import (
	"bytes"
	"fmt"
	"github.com/ugorji/go/codec"
	"time"
)

var (
	ErrNoLeader  = fmt.Errorf("No cluster leader")
	ErrNoDCPath  = fmt.Errorf("No path to datacenter")
	ErrNoServers = fmt.Errorf("No known Consul servers")
)

type MessageType uint8

const (
	RegisterRequestType MessageType = iota
	DeregisterRequestType
	KVSRequestType
)

const (
	HealthUnknown  = "unknown"
	HealthPassing  = "passing"
	HealthWarning  = "warning"
	HealthCritical = "critical"
)

// RPCInfo is used to describe common information about query
type RPCInfo interface {
	RequestDatacenter() string
	IsRead() bool
	AllowStaleRead() bool
}

// BlockingQuery is used to block on a query and wait for a change.
// Either both fields, or neither must be provided.
type BlockingQuery struct {
	// If set, wait until query exceeds given index
	MinQueryIndex uint64

	// Provided with MinQueryIndex to wait for change
	MaxQueryTime time.Duration
}

// QueryOptions is used to specify various flags for read queries
type QueryOptions struct {
	// If set, any follower can service the request. Results
	// may be arbitrarily stale.
	AllowStale bool

	// If set, the leader must verify leadership prior to
	// servicing the request. Prevents a stale read.
	RequireConsistent bool
}

// QueryOption only applies to reads, so always true
func (q QueryOptions) IsRead() bool {
	return true
}

func (q QueryOptions) AllowStaleRead() bool {
	return q.AllowStale
}

type WriteRequest struct{}

// WriteRequest only applies to writes, always false
func (w WriteRequest) IsRead() bool {
	return false
}

func (w WriteRequest) AllowStaleRead() bool {
	return false
}

// QueryMeta allows a query response to include potentially
// useful metadata about a query
type QueryMeta struct {
	// If AllowStale is used, this is time elapsed since
	// last contact between the follower and leader. This
	// can be used to gauge staleness.
	LastContact time.Duration

	// Used to indicate if there is a known leader node
	KnownLeader bool
}

// RegisterRequest is used for the Catalog.Register endpoint
// to register a node as providing a service. If no service
// is provided, the node is registered.
type RegisterRequest struct {
	Datacenter string
	Node       string
	Address    string
	Service    *NodeService
	Check      *HealthCheck
	WriteRequest
}

func (r *RegisterRequest) RequestDatacenter() string {
	return r.Datacenter
}

// DeregisterRequest is used for the Catalog.Deregister endpoint
// to deregister a node as providing a service. If no service is
// provided the entire node is deregistered.
type DeregisterRequest struct {
	Datacenter string
	Node       string
	ServiceID  string
	CheckID    string
	WriteRequest
}

func (r *DeregisterRequest) RequestDatacenter() string {
	return r.Datacenter
}

// DCSpecificRequest is used to query about a specific DC
type DCSpecificRequest struct {
	Datacenter string
	BlockingQuery
	QueryOptions
}

func (r *DCSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ServiceSpecificRequest is used to query about a specific node
type ServiceSpecificRequest struct {
	Datacenter  string
	ServiceName string
	ServiceTag  string
	TagFilter   bool // Controls tag filtering
	BlockingQuery
	QueryOptions
}

func (r *ServiceSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

// NodeSpecificRequest is used to request the information about a single node
type NodeSpecificRequest struct {
	Datacenter string
	Node       string
	BlockingQuery
	QueryOptions
}

func (r *NodeSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ChecksInStateRequest is used to query for nodes in a state
type ChecksInStateRequest struct {
	Datacenter string
	State      string
	BlockingQuery
	QueryOptions
}

func (r *ChecksInStateRequest) RequestDatacenter() string {
	return r.Datacenter
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

// ServiceNode represents a node that is part of a service
type ServiceNode struct {
	Node        string
	Address     string
	ServiceID   string
	ServiceName string
	ServiceTags []string
	ServicePort int
}
type ServiceNodes []ServiceNode

// NodeService is a service provided by a node
type NodeService struct {
	ID      string
	Service string
	Tags    []string
	Port    int
}
type NodeServices struct {
	Node     Node
	Services map[string]*NodeService
}

// HealthCheck represents a single check on a given node
type HealthCheck struct {
	Node        string
	CheckID     string // Unique per-node ID
	Name        string // Check name
	Status      string // The current check status
	Notes       string // Additional notes with the status
	ServiceID   string // optional associated service
	ServiceName string // optional service name
}
type HealthChecks []*HealthCheck

// CheckServiceNode is used to provide the node, it's service
// definition, as well as a HealthCheck that is associated
type CheckServiceNode struct {
	Node    Node
	Service NodeService
	Checks  HealthChecks
}
type CheckServiceNodes []CheckServiceNode

type IndexedNodes struct {
	Index uint64
	Nodes Nodes
	QueryMeta
}

type IndexedServices struct {
	Index    uint64
	Services Services
	QueryMeta
}

type IndexedServiceNodes struct {
	Index        uint64
	ServiceNodes ServiceNodes
	QueryMeta
}

type IndexedNodeServices struct {
	Index        uint64
	NodeServices *NodeServices
	QueryMeta
}

type IndexedHealthChecks struct {
	Index        uint64
	HealthChecks HealthChecks
	QueryMeta
}

type IndexedCheckServiceNodes struct {
	Index uint64
	Nodes CheckServiceNodes
	QueryMeta
}

// DirEntry is used to represent a directory entry. This is
// used for values in our Key-Value store.
type DirEntry struct {
	CreateIndex uint64
	ModifyIndex uint64
	Key         string
	Flags       uint64
	Value       []byte
}
type DirEntries []*DirEntry

type KVSOp string

const (
	KVSSet        KVSOp = "set"
	KVSDelete           = "delete"
	KVSDeleteTree       = "delete-tree"
	KVSCAS              = "cas" // Check-and-set
)

// KVSRequest is used to operate on the Key-Value store
type KVSRequest struct {
	Datacenter string
	Op         KVSOp    // Which operation are we performing
	DirEnt     DirEntry // Which directory entry
	WriteRequest
}

func (r *KVSRequest) RequestDatacenter() string {
	return r.Datacenter
}

// KeyRequest is used to request a key, or key prefix
type KeyRequest struct {
	Datacenter string
	Key        string
	BlockingQuery
	QueryOptions
}

func (r *KeyRequest) RequestDatacenter() string {
	return r.Datacenter
}

type IndexedDirEntries struct {
	Index   uint64
	Entries DirEntries
	QueryMeta
}

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

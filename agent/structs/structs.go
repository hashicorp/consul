package structs

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/serf/coordinate"
)

type MessageType uint8

// RaftIndex is used to track the index used while creating
// or modifying a given struct type.
type RaftIndex struct {
	CreateIndex uint64
	ModifyIndex uint64
}

// These are serialized between Consul servers and stored in Consul snapshots,
// so entries must only ever be added.
const (
	RegisterRequestType       MessageType = 0
	DeregisterRequestType                 = 1
	KVSRequestType                        = 2
	SessionRequestType                    = 3
	ACLRequestType                        = 4
	TombstoneRequestType                  = 5
	CoordinateBatchUpdateType             = 6
	PreparedQueryRequestType              = 7
	TxnRequestType                        = 8
	AutopilotRequestType                  = 9
	AreaRequestType                       = 10
	ACLBootstrapRequestType               = 11 // FSM snapshots only.
)

const (
	// IgnoreUnknownTypeFlag is set along with a MessageType
	// to indicate that the message type can be safely ignored
	// if it is not recognized. This is for future proofing, so
	// that new commands can be added in a way that won't cause
	// old servers to crash when the FSM attempts to process them.
	IgnoreUnknownTypeFlag MessageType = 128

	// NodeMaint is the special key set by a node in maintenance mode.
	NodeMaint = "_node_maintenance"

	// ServiceMaintPrefix is the prefix for a service in maintenance mode.
	ServiceMaintPrefix = "_service_maintenance:"

	// The meta key prefix reserved for Consul's internal use
	metaKeyReservedPrefix = "consul-"

	// metaMaxKeyPairs is maximum number of metadata key pairs allowed to be registered
	metaMaxKeyPairs = 64

	// metaKeyMaxLength is the maximum allowed length of a metadata key
	metaKeyMaxLength = 128

	// metaValueMaxLength is the maximum allowed length of a metadata value
	metaValueMaxLength = 512

	// MetaSegmentKey is the node metadata key used to store the node's network segment
	MetaSegmentKey = "consul-network-segment"

	// MaxLockDelay provides a maximum LockDelay value for
	// a session. Any value above this will not be respected.
	MaxLockDelay = 60 * time.Second
)

// metaKeyFormat checks if a metadata key string is valid
var metaKeyFormat = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString

func ValidStatus(s string) bool {
	return s == api.HealthPassing || s == api.HealthWarning || s == api.HealthCritical
}

// RPCInfo is used to describe common information about query
type RPCInfo interface {
	RequestDatacenter() string
	IsRead() bool
	AllowStaleRead() bool
	ACLToken() string
}

// QueryOptions is used to specify various flags for read queries
type QueryOptions struct {
	// Token is the ACL token ID. If not provided, the 'anonymous'
	// token is assumed for backwards compatibility.
	Token string

	// If set, wait until query exceeds given index. Must be provided
	// with MaxQueryTime.
	MinQueryIndex uint64

	// Provided with MinQueryIndex to wait for change.
	MaxQueryTime time.Duration

	// If set, any follower can service the request. Results
	// may be arbitrarily stale.
	AllowStale bool

	// If set, the leader must verify leadership prior to
	// servicing the request. Prevents a stale read.
	RequireConsistent bool

	// If set and AllowStale is true, will try first a stale
	// read, and then will perform a consistent read if stale
	// read is older than value
	MaxStaleDuration time.Duration
}

// IsRead is always true for QueryOption.
func (q QueryOptions) IsRead() bool {
	return true
}

// ConsistencyLevel display the consistency required by a request
func (q QueryOptions) ConsistencyLevel() string {
	if q.RequireConsistent {
		return "consistent"
	} else if q.AllowStale {
		return "stale"
	} else {
		return "leader"
	}
}

func (q QueryOptions) AllowStaleRead() bool {
	return q.AllowStale
}

func (q QueryOptions) ACLToken() string {
	return q.Token
}

type WriteRequest struct {
	// Token is the ACL token ID. If not provided, the 'anonymous'
	// token is assumed for backwards compatibility.
	Token string
}

// WriteRequest only applies to writes, always false
func (w WriteRequest) IsRead() bool {
	return false
}

func (w WriteRequest) AllowStaleRead() bool {
	return false
}

func (w WriteRequest) ACLToken() string {
	return w.Token
}

// QueryMeta allows a query response to include potentially
// useful metadata about a query
type QueryMeta struct {
	// This is the index associated with the read
	Index uint64

	// If AllowStale is used, this is time elapsed since
	// last contact between the follower and leader. This
	// can be used to gauge staleness.
	LastContact time.Duration

	// Used to indicate if there is a known leader node
	KnownLeader bool

	// Consistencylevel returns the consistency used to serve the query
	// Having `discovery_max_stale` on the agent can affect whether
	// the request was served by a leader.
	ConsistencyLevel string
}

// RegisterRequest is used for the Catalog.Register endpoint
// to register a node as providing a service. If no service
// is provided, the node is registered.
type RegisterRequest struct {
	Datacenter      string
	ID              types.NodeID
	Node            string
	Address         string
	TaggedAddresses map[string]string
	NodeMeta        map[string]string
	Service         *NodeService
	Check           *HealthCheck
	Checks          HealthChecks

	// SkipNodeUpdate can be used when a register request is intended for
	// updating a service and/or checks, but doesn't want to overwrite any
	// node information if the node is already registered. If the node
	// doesn't exist, it will still be created, but if the node exists, any
	// node portion of this update will not apply.
	SkipNodeUpdate bool

	WriteRequest
}

func (r *RegisterRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ChangesNode returns true if the given register request changes the given
// node, which can be nil. This only looks for changes to the node record itself,
// not any of the health checks.
func (r *RegisterRequest) ChangesNode(node *Node) bool {
	// This means it's creating the node.
	if node == nil {
		return true
	}

	// If we've been asked to skip the node update, then say there are no
	// changes.
	if r.SkipNodeUpdate {
		return false
	}

	// Check if any of the node-level fields are being changed.
	if r.ID != node.ID ||
		r.Node != node.Node ||
		r.Address != node.Address ||
		r.Datacenter != node.Datacenter ||
		!reflect.DeepEqual(r.TaggedAddresses, node.TaggedAddresses) ||
		!reflect.DeepEqual(r.NodeMeta, node.Meta) {
		return true
	}

	return false
}

// DeregisterRequest is used for the Catalog.Deregister endpoint
// to deregister a node as providing a service. If no service is
// provided the entire node is deregistered.
type DeregisterRequest struct {
	Datacenter string
	Node       string
	ServiceID  string
	CheckID    types.CheckID
	WriteRequest
}

func (r *DeregisterRequest) RequestDatacenter() string {
	return r.Datacenter
}

// QuerySource is used to pass along information about the source node
// in queries so that we can adjust the response based on its network
// coordinates.
type QuerySource struct {
	Datacenter string
	Segment    string
	Node       string
	Ip         string
}

// DCSpecificRequest is used to query about a specific DC
type DCSpecificRequest struct {
	Datacenter      string
	NodeMetaFilters map[string]string
	Source          QuerySource
	QueryOptions
}

func (r *DCSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ServiceSpecificRequest is used to query about a specific service
type ServiceSpecificRequest struct {
	Datacenter      string
	NodeMetaFilters map[string]string
	ServiceName     string
	ServiceTag      string
	TagFilter       bool // Controls tag filtering
	Source          QuerySource
	QueryOptions
}

func (r *ServiceSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

// NodeSpecificRequest is used to request the information about a single node
type NodeSpecificRequest struct {
	Datacenter string
	Node       string
	QueryOptions
}

func (r *NodeSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

// ChecksInStateRequest is used to query for nodes in a state
type ChecksInStateRequest struct {
	Datacenter      string
	NodeMetaFilters map[string]string
	State           string
	Source          QuerySource
	QueryOptions
}

func (r *ChecksInStateRequest) RequestDatacenter() string {
	return r.Datacenter
}

// Used to return information about a node
type Node struct {
	ID              types.NodeID
	Node            string
	Address         string
	Datacenter      string
	TaggedAddresses map[string]string
	Meta            map[string]string

	RaftIndex
}
type Nodes []*Node

// ValidateMeta validates a set of key/value pairs from the agent config
func ValidateMetadata(meta map[string]string, allowConsulPrefix bool) error {
	if len(meta) > metaMaxKeyPairs {
		return fmt.Errorf("Node metadata cannot contain more than %d key/value pairs", metaMaxKeyPairs)
	}

	for key, value := range meta {
		if err := validateMetaPair(key, value, allowConsulPrefix); err != nil {
			return fmt.Errorf("Couldn't load metadata pair ('%s', '%s'): %s", key, value, err)
		}
	}

	return nil
}

// validateMetaPair checks that the given key/value pair is in a valid format
func validateMetaPair(key, value string, allowConsulPrefix bool) error {
	if key == "" {
		return fmt.Errorf("Key cannot be blank")
	}
	if !metaKeyFormat(key) {
		return fmt.Errorf("Key contains invalid characters")
	}
	if len(key) > metaKeyMaxLength {
		return fmt.Errorf("Key is too long (limit: %d characters)", metaKeyMaxLength)
	}
	if strings.HasPrefix(key, metaKeyReservedPrefix) && !allowConsulPrefix {
		return fmt.Errorf("Key prefix '%s' is reserved for internal use", metaKeyReservedPrefix)
	}
	if len(value) > metaValueMaxLength {
		return fmt.Errorf("Value is too long (limit: %d characters)", metaValueMaxLength)
	}
	return nil
}

// SatisfiesMetaFilters returns true if the metadata map contains the given filters
func SatisfiesMetaFilters(meta map[string]string, filters map[string]string) bool {
	for key, value := range filters {
		if v, ok := meta[key]; !ok || v != value {
			return false
		}
	}
	return true
}

// Used to return information about a provided services.
// Maps service name to available tags
type Services map[string][]string

// ServiceNode represents a node that is part of a service. ID, Address,
// TaggedAddresses, and NodeMeta are node-related fields that are always empty
// in the state store and are filled in on the way out by parseServiceNodes().
// This is also why PartialClone() skips them, because we know they are blank
// already so it would be a waste of time to copy them.
type ServiceNode struct {
	ID                       types.NodeID
	Node                     string
	Address                  string
	Datacenter               string
	TaggedAddresses          map[string]string
	NodeMeta                 map[string]string
	ServiceID                string
	ServiceName              string
	ServiceTags              []string
	ServiceAddress           string
	ServiceMeta              map[string]string
	ServicePort              int
	ServiceEnableTagOverride bool

	RaftIndex
}

// PartialClone() returns a clone of the given service node, minus the node-
// related fields that get filled in later, Address and TaggedAddresses.
func (s *ServiceNode) PartialClone() *ServiceNode {
	tags := make([]string, len(s.ServiceTags))
	copy(tags, s.ServiceTags)
	nsmeta := make(map[string]string)
	for k, v := range s.ServiceMeta {
		nsmeta[k] = v
	}

	return &ServiceNode{
		// Skip ID, see above.
		Node: s.Node,
		// Skip Address, see above.
		// Skip TaggedAddresses, see above.
		ServiceID:                s.ServiceID,
		ServiceName:              s.ServiceName,
		ServiceTags:              tags,
		ServiceAddress:           s.ServiceAddress,
		ServicePort:              s.ServicePort,
		ServiceMeta:              nsmeta,
		ServiceEnableTagOverride: s.ServiceEnableTagOverride,
		RaftIndex: RaftIndex{
			CreateIndex: s.CreateIndex,
			ModifyIndex: s.ModifyIndex,
		},
	}
}

// ToNodeService converts the given service node to a node service.
func (s *ServiceNode) ToNodeService() *NodeService {
	return &NodeService{
		ID:                s.ServiceID,
		Service:           s.ServiceName,
		Tags:              s.ServiceTags,
		Address:           s.ServiceAddress,
		Port:              s.ServicePort,
		Meta:              s.ServiceMeta,
		EnableTagOverride: s.ServiceEnableTagOverride,
		RaftIndex: RaftIndex{
			CreateIndex: s.CreateIndex,
			ModifyIndex: s.ModifyIndex,
		},
	}
}

type ServiceNodes []*ServiceNode

// NodeService is a service provided by a node
type NodeService struct {
	ID                string
	Service           string
	Tags              []string
	Address           string
	Meta              map[string]string
	Port              int
	EnableTagOverride bool

	RaftIndex
}

// IsSame checks if one NodeService is the same as another, without looking
// at the Raft information (that's why we didn't call it IsEqual). This is
// useful for seeing if an update would be idempotent for all the functional
// parts of the structure.
func (s *NodeService) IsSame(other *NodeService) bool {
	if s.ID != other.ID ||
		s.Service != other.Service ||
		!reflect.DeepEqual(s.Tags, other.Tags) ||
		s.Address != other.Address ||
		s.Port != other.Port ||
		!reflect.DeepEqual(s.Meta, other.Meta) ||
		s.EnableTagOverride != other.EnableTagOverride {
		return false
	}

	return true
}

// ToServiceNode converts the given node service to a service node.
func (s *NodeService) ToServiceNode(node string) *ServiceNode {
	return &ServiceNode{
		// Skip ID, see ServiceNode definition.
		Node: node,
		// Skip Address, see ServiceNode definition.
		// Skip TaggedAddresses, see ServiceNode definition.
		ServiceID:                s.ID,
		ServiceName:              s.Service,
		ServiceTags:              s.Tags,
		ServiceAddress:           s.Address,
		ServicePort:              s.Port,
		ServiceMeta:              s.Meta,
		ServiceEnableTagOverride: s.EnableTagOverride,
		RaftIndex: RaftIndex{
			CreateIndex: s.CreateIndex,
			ModifyIndex: s.ModifyIndex,
		},
	}
}

type NodeServices struct {
	Node     *Node
	Services map[string]*NodeService
}

// HealthCheck represents a single check on a given node
type HealthCheck struct {
	Node        string
	CheckID     types.CheckID // Unique per-node ID
	Name        string        // Check name
	Status      string        // The current check status
	Notes       string        // Additional notes with the status
	Output      string        // Holds output of script runs
	ServiceID   string        // optional associated service
	ServiceName string        // optional service name
	ServiceTags []string      // optional service tags

	Definition HealthCheckDefinition

	RaftIndex
}

type HealthCheckDefinition struct {
	HTTP                           string               `json:",omitempty"`
	TLSSkipVerify                  bool                 `json:",omitempty"`
	Header                         map[string][]string  `json:",omitempty"`
	Method                         string               `json:",omitempty"`
	TCP                            string               `json:",omitempty"`
	Interval                       api.ReadableDuration `json:",omitempty"`
	Timeout                        api.ReadableDuration `json:",omitempty"`
	DeregisterCriticalServiceAfter api.ReadableDuration `json:",omitempty"`
}

// IsSame checks if one HealthCheck is the same as another, without looking
// at the Raft information (that's why we didn't call it IsEqual). This is
// useful for seeing if an update would be idempotent for all the functional
// parts of the structure.
func (c *HealthCheck) IsSame(other *HealthCheck) bool {
	if c.Node != other.Node ||
		c.CheckID != other.CheckID ||
		c.Name != other.Name ||
		c.Status != other.Status ||
		c.Notes != other.Notes ||
		c.Output != other.Output ||
		c.ServiceID != other.ServiceID ||
		c.ServiceName != other.ServiceName ||
		!reflect.DeepEqual(c.ServiceTags, other.ServiceTags) {
		return false
	}

	return true
}

// Clone returns a distinct clone of the HealthCheck.
func (c *HealthCheck) Clone() *HealthCheck {
	clone := new(HealthCheck)
	*clone = *c
	return clone
}

// HealthChecks is a collection of HealthCheck structs.
type HealthChecks []*HealthCheck

// CheckServiceNode is used to provide the node, its service
// definition, as well as a HealthCheck that is associated.
type CheckServiceNode struct {
	Node    *Node
	Service *NodeService
	Checks  HealthChecks
}
type CheckServiceNodes []CheckServiceNode

// Shuffle does an in-place random shuffle using the Fisher-Yates algorithm.
func (nodes CheckServiceNodes) Shuffle() {
	for i := len(nodes) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

// Filter removes nodes that are failing health checks (and any non-passing
// check if that option is selected). Note that this returns the filtered
// results AND modifies the receiver for performance.
func (nodes CheckServiceNodes) Filter(onlyPassing bool) CheckServiceNodes {
	return nodes.FilterIgnore(onlyPassing, nil)
}

// FilterIgnore removes nodes that are failing health checks just like Filter.
// It also ignores the status of any check with an ID present in ignoreCheckIDs
// as if that check didn't exist. Note that this returns the filtered results
// AND modifies the receiver for performance.
func (nodes CheckServiceNodes) FilterIgnore(onlyPassing bool,
	ignoreCheckIDs []types.CheckID) CheckServiceNodes {
	n := len(nodes)
OUTER:
	for i := 0; i < n; i++ {
		node := nodes[i]
	INNER:
		for _, check := range node.Checks {
			for _, ignore := range ignoreCheckIDs {
				if check.CheckID == ignore {
					// Skip this _check_ but keep looking at other checks for this node.
					continue INNER
				}
			}
			if check.Status == api.HealthCritical ||
				(onlyPassing && check.Status != api.HealthPassing) {
				nodes[i], nodes[n-1] = nodes[n-1], CheckServiceNode{}
				n--
				i--
				// Skip this _node_ now we've swapped it off the end of the list.
				continue OUTER
			}
		}
	}
	return nodes[:n]
}

// NodeInfo is used to dump all associated information about
// a node. This is currently used for the UI only, as it is
// rather expensive to generate.
type NodeInfo struct {
	ID              types.NodeID
	Node            string
	Address         string
	TaggedAddresses map[string]string
	Meta            map[string]string
	Services        []*NodeService
	Checks          HealthChecks
}

// NodeDump is used to dump all the nodes with all their
// associated data. This is currently used for the UI only,
// as it is rather expensive to generate.
type NodeDump []*NodeInfo

type IndexedNodes struct {
	Nodes Nodes
	QueryMeta
}

type IndexedServices struct {
	Services Services
	QueryMeta
}

type IndexedServiceNodes struct {
	ServiceNodes ServiceNodes
	QueryMeta
}

type IndexedNodeServices struct {
	// TODO: This should not be a pointer, see comments in
	// agent/catalog_endpoint.go.
	NodeServices *NodeServices
	QueryMeta
}

type IndexedHealthChecks struct {
	HealthChecks HealthChecks
	QueryMeta
}

type IndexedCheckServiceNodes struct {
	Nodes CheckServiceNodes
	QueryMeta
}

type IndexedNodeDump struct {
	Dump NodeDump
	QueryMeta
}

// DirEntry is used to represent a directory entry. This is
// used for values in our Key-Value store.
type DirEntry struct {
	LockIndex uint64
	Key       string
	Flags     uint64
	Value     []byte
	Session   string `json:",omitempty"`

	RaftIndex
}

// Returns a clone of the given directory entry.
func (d *DirEntry) Clone() *DirEntry {
	return &DirEntry{
		LockIndex: d.LockIndex,
		Key:       d.Key,
		Flags:     d.Flags,
		Value:     d.Value,
		Session:   d.Session,
		RaftIndex: RaftIndex{
			CreateIndex: d.CreateIndex,
			ModifyIndex: d.ModifyIndex,
		},
	}
}

type DirEntries []*DirEntry

// KVSRequest is used to operate on the Key-Value store
type KVSRequest struct {
	Datacenter string
	Op         api.KVOp // Which operation are we performing
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
	QueryOptions
}

func (r *KeyRequest) RequestDatacenter() string {
	return r.Datacenter
}

// KeyListRequest is used to list keys
type KeyListRequest struct {
	Datacenter string
	Prefix     string
	Seperator  string
	QueryOptions
}

func (r *KeyListRequest) RequestDatacenter() string {
	return r.Datacenter
}

type IndexedDirEntries struct {
	Entries DirEntries
	QueryMeta
}

type IndexedKeyList struct {
	Keys []string
	QueryMeta
}

type SessionBehavior string

const (
	SessionKeysRelease SessionBehavior = "release"
	SessionKeysDelete                  = "delete"
)

const (
	SessionTTLMax        = 24 * time.Hour
	SessionTTLMultiplier = 2
)

// Session is used to represent an open session in the KV store.
// This issued to associate node checks with acquired locks.
type Session struct {
	ID        string
	Name      string
	Node      string
	Checks    []types.CheckID
	LockDelay time.Duration
	Behavior  SessionBehavior // What to do when session is invalidated
	TTL       string

	RaftIndex
}
type Sessions []*Session

type SessionOp string

const (
	SessionCreate  SessionOp = "create"
	SessionDestroy           = "destroy"
)

// SessionRequest is used to operate on sessions
type SessionRequest struct {
	Datacenter string
	Op         SessionOp // Which operation are we performing
	Session    Session   // Which session
	WriteRequest
}

func (r *SessionRequest) RequestDatacenter() string {
	return r.Datacenter
}

// SessionSpecificRequest is used to request a session by ID
type SessionSpecificRequest struct {
	Datacenter string
	Session    string
	QueryOptions
}

func (r *SessionSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

type IndexedSessions struct {
	Sessions Sessions
	QueryMeta
}

// Coordinate stores a node name with its associated network coordinate.
type Coordinate struct {
	Node    string
	Segment string
	Coord   *coordinate.Coordinate
}

type Coordinates []*Coordinate

// IndexedCoordinate is used to represent a single node's coordinate from the state
// store.
type IndexedCoordinate struct {
	Coord *coordinate.Coordinate
	QueryMeta
}

// IndexedCoordinates is used to represent a list of nodes and their
// corresponding raw coordinates.
type IndexedCoordinates struct {
	Coordinates Coordinates
	QueryMeta
}

// DatacenterMap is used to represent a list of nodes with their raw coordinates,
// associated with a datacenter. Coordinates are only compatible between nodes in
// the same area.
type DatacenterMap struct {
	Datacenter  string
	AreaID      types.AreaID
	Coordinates Coordinates
}

// CoordinateUpdateRequest is used to update the network coordinate of a given
// node.
type CoordinateUpdateRequest struct {
	Datacenter string
	Node       string
	Segment    string
	Coord      *coordinate.Coordinate
	WriteRequest
}

// RequestDatacenter returns the datacenter for a given update request.
func (c *CoordinateUpdateRequest) RequestDatacenter() string {
	return c.Datacenter
}

// EventFireRequest is used to ask a server to fire
// a Serf event. It is a bit odd, since it doesn't depend on
// the catalog or leader. Any node can respond, so it's not quite
// like a standard write request. This is used only internally.
type EventFireRequest struct {
	Datacenter string
	Name       string
	Payload    []byte

	// Not using WriteRequest so that any server can process
	// the request. It is a bit unusual...
	QueryOptions
}

func (r *EventFireRequest) RequestDatacenter() string {
	return r.Datacenter
}

// EventFireResponse is used to respond to a fire request.
type EventFireResponse struct {
	QueryMeta
}

type TombstoneOp string

const (
	TombstoneReap TombstoneOp = "reap"
)

// TombstoneRequest is used to trigger a reaping of the tombstones
type TombstoneRequest struct {
	Datacenter string
	Op         TombstoneOp
	ReapIndex  uint64
	WriteRequest
}

func (r *TombstoneRequest) RequestDatacenter() string {
	return r.Datacenter
}

// msgpackHandle is a shared handle for encoding/decoding of structs
var msgpackHandle = &codec.MsgpackHandle{}

// Decode is used to decode a MsgPack encoded object
func Decode(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), msgpackHandle).Decode(out)
}

// Encode is used to encode a MsgPack object with type prefix
func Encode(t MessageType, msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(uint8(t))
	err := codec.NewEncoder(&buf, msgpackHandle).Encode(msg)
	return buf.Bytes(), err
}

// CompoundResponse is an interface for gathering multiple responses. It is
// used in cross-datacenter RPC calls where more than 1 datacenter is
// expected to reply.
type CompoundResponse interface {
	// Add adds a new response to the compound response
	Add(interface{})

	// New returns an empty response object which can be passed around by
	// reference, and then passed to Add() later on.
	New() interface{}
}

type KeyringOp string

const (
	KeyringList    KeyringOp = "list"
	KeyringInstall           = "install"
	KeyringUse               = "use"
	KeyringRemove            = "remove"
)

// KeyringRequest encapsulates a request to modify an encryption keyring.
// It can be used for install, remove, or use key type operations.
type KeyringRequest struct {
	Operation   KeyringOp
	Key         string
	Datacenter  string
	Forwarded   bool
	RelayFactor uint8
	QueryOptions
}

func (r *KeyringRequest) RequestDatacenter() string {
	return r.Datacenter
}

// KeyringResponse is a unified key response and can be used for install,
// remove, use, as well as listing key queries.
type KeyringResponse struct {
	WAN        bool
	Datacenter string
	Segment    string
	Messages   map[string]string `json:",omitempty"`
	Keys       map[string]int
	NumNodes   int
	Error      string `json:",omitempty"`
}

// KeyringResponses holds multiple responses to keyring queries. Each
// datacenter replies independently, and KeyringResponses is used as a
// container for the set of all responses.
type KeyringResponses struct {
	Responses []*KeyringResponse
	QueryMeta
}

func (r *KeyringResponses) Add(v interface{}) {
	val := v.(*KeyringResponses)
	r.Responses = append(r.Responses, val.Responses...)
}

func (r *KeyringResponses) New() interface{} {
	return new(KeyringResponses)
}

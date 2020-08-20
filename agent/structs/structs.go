package structs

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/serf/coordinate"
	"github.com/mitchellh/hashstructure"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
)

type MessageType uint8

// RaftIndex is used to track the index used while creating
// or modifying a given struct type.
type RaftIndex struct {
	CreateIndex uint64 `bexpr:"-"`
	ModifyIndex uint64 `bexpr:"-"`
}

// These are serialized between Consul servers and stored in Consul snapshots,
// so entries must only ever be added.
const (
	RegisterRequestType             MessageType = 0
	DeregisterRequestType                       = 1
	KVSRequestType                              = 2
	SessionRequestType                          = 3
	ACLRequestType                              = 4 // DEPRECATED (ACL-Legacy-Compat)
	TombstoneRequestType                        = 5
	CoordinateBatchUpdateType                   = 6
	PreparedQueryRequestType                    = 7
	TxnRequestType                              = 8
	AutopilotRequestType                        = 9
	AreaRequestType                             = 10
	ACLBootstrapRequestType                     = 11
	IntentionRequestType                        = 12
	ConnectCARequestType                        = 13
	ConnectCAProviderStateType                  = 14
	ConnectCAConfigType                         = 15 // FSM snapshots only.
	IndexRequestType                            = 16 // FSM snapshots only.
	ACLTokenSetRequestType                      = 17
	ACLTokenDeleteRequestType                   = 18
	ACLPolicySetRequestType                     = 19
	ACLPolicyDeleteRequestType                  = 20
	ConnectCALeafRequestType                    = 21
	ConfigEntryRequestType                      = 22
	ACLRoleSetRequestType                       = 23
	ACLRoleDeleteRequestType                    = 24
	ACLBindingRuleSetRequestType                = 25
	ACLBindingRuleDeleteRequestType             = 26
	ACLAuthMethodSetRequestType                 = 27
	ACLAuthMethodDeleteRequestType              = 28
	ChunkingStateType                           = 29
	FederationStateRequestType                  = 30
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

	// MetaWANFederationKey is the mesh gateway metadata key that indicates a
	// mesh gateway is usable for wan federation.
	MetaWANFederationKey = "consul-wan-federation"

	// MaxLockDelay provides a maximum LockDelay value for
	// a session. Any value above this will not be respected.
	MaxLockDelay = 60 * time.Second

	// lockDelayMinThreshold is used in JSON decoding to convert a
	// numeric lockdelay value from nanoseconds to seconds if it is
	// below thisthreshold. Users often send a value like 5, which
	// they assumeis seconds, but because Go uses nanosecond granularity,
	// ends up being very small. If we see a value below this threshold,
	// we multiply by time.Second
	lockDelayMinThreshold = 1000

	// WildcardSpecifier is the string which should be used for specifying a wildcard
	// The exact semantics of the wildcard is left up to the code where its used.
	WildcardSpecifier = "*"
)

var allowedConsulMetaKeysForMeshGateway = map[string]struct{}{MetaWANFederationKey: {}}

var (
	NodeMaintCheckID = NewCheckID(NodeMaint, nil)
)

const (
	TaggedAddressWAN     = "wan"
	TaggedAddressWANIPv4 = "wan_ipv4"
	TaggedAddressWANIPv6 = "wan_ipv6"
	TaggedAddressLAN     = "lan"
	TaggedAddressLANIPv4 = "lan_ipv4"
	TaggedAddressLANIPv6 = "lan_ipv6"
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
	TokenSecret() string
	SetTokenSecret(string)
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

	// If set, the local agent may respond with an arbitrarily stale locally
	// cached response. The semantics differ from AllowStale since the agent may
	// be entirely partitioned from the servers and still considered "healthy" by
	// operators. Stale responses from Servers are also arbitrarily stale, but can
	// provide additional bounds on the last contact time from the leader. It's
	// expected that servers that are partitioned are noticed and replaced in a
	// timely way by operators while the same may not be true for client agents.
	UseCache bool

	// If set and AllowStale is true, will try first a stale
	// read, and then will perform a consistent read if stale
	// read is older than value.
	MaxStaleDuration time.Duration

	// MaxAge limits how old a cached value will be returned if UseCache is true.
	// If there is a cached response that is older than the MaxAge, it is treated
	// as a cache miss and a new fetch invoked. If the fetch fails, the error is
	// returned. Clients that wish to allow for stale results on error can set
	// StaleIfError to a longer duration to change this behavior. It is ignored
	// if the endpoint supports background refresh caching. See
	// https://www.consul.io/api/index.html#agent-caching for more details.
	MaxAge time.Duration

	// MustRevalidate forces the agent to fetch a fresh version of a cached
	// resource or at least validate that the cached version is still fresh. It is
	// implied by either max-age=0 or must-revalidate Cache-Control headers. It
	// only makes sense when UseCache is true. We store it since MaxAge = 0 is the
	// default unset value.
	MustRevalidate bool

	// StaleIfError specifies how stale the client will accept a cached response
	// if the servers are unavailable to fetch a fresh one. Only makes sense when
	// UseCache is true and MaxAge is set to a lower, non-zero value. It is
	// ignored if the endpoint supports background refresh caching. See
	// https://www.consul.io/api/index.html#agent-caching for more details.
	StaleIfError time.Duration

	// Filter specifies the go-bexpr filter expression to be used for
	// filtering the data prior to returning a response
	Filter string

	// AllowNotModifiedResponse indicates that if the MinIndex matches the
	// QueryMeta.Index, the response can be left empty and QueryMeta.NotModified
	// will be set to true to indicate the result of the query has not changed.
	AllowNotModifiedResponse bool
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

func (q QueryOptions) TokenSecret() string {
	return q.Token
}

func (q *QueryOptions) SetTokenSecret(s string) {
	q.Token = s
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

func (w WriteRequest) TokenSecret() string {
	return w.Token
}

func (w *WriteRequest) SetTokenSecret(s string) {
	w.Token = s
}

// QueryMeta allows a query response to include potentially
// useful metadata about a query
type QueryMeta struct {
	// Index in the raft log of the latest item returned by the query.
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

	// NotModified is true when the Index of the query is the same value as the
	// requested MinIndex. It indicates that the entity has not been modified.
	// When NotModified is true, the response will not contain the result of
	// the query.
	NotModified bool
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

	// EnterpriseMeta is the embedded enterprise metadata
	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`

	WriteRequest
	RaftIndex `bexpr:"-"`
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
	Datacenter     string
	Node           string
	ServiceID      string
	CheckID        types.CheckID
	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	WriteRequest
}

func (r *DeregisterRequest) RequestDatacenter() string {
	return r.Datacenter
}

func (r *DeregisterRequest) UnmarshalJSON(data []byte) error {
	type Alias DeregisterRequest
	aux := &struct {
		Address string // obsolete field - but we want to explicitly allow it
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := lib.UnmarshalJSON(data, &aux); err != nil {
		return err
	}
	return nil
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

type DatacentersRequest struct {
	QueryOptions
}

func (r *DatacentersRequest) CacheInfo() cache.RequestInfo {
	return cache.RequestInfo{
		Token:          "",
		Datacenter:     "",
		MinIndex:       0,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
		Key:            "catalog-datacenters", // must not be empty for cache to work
	}
}

// DCSpecificRequest is used to query about a specific DC
type DCSpecificRequest struct {
	Datacenter      string
	NodeMetaFilters map[string]string
	Source          QuerySource
	EnterpriseMeta  `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (r *DCSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

func (r *DCSpecificRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	// To calculate the cache key we only hash the node meta filters and the bexpr filter.
	// The datacenter is handled by the cache framework. The other fields are
	// not, but should not be used in any cache types.
	v, err := hashstructure.Hash([]interface{}{
		r.NodeMetaFilters,
		r.Filter,
		r.EnterpriseMeta,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

func (r *DCSpecificRequest) CacheMinIndex() uint64 {
	return r.QueryOptions.MinQueryIndex
}

type ServiceDumpRequest struct {
	Datacenter     string
	ServiceKind    ServiceKind
	UseServiceKind bool
	Source         QuerySource
	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (r *ServiceDumpRequest) RequestDatacenter() string {
	return r.Datacenter
}

func (r *ServiceDumpRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	// When we are not using the service kind we want to normalize the ServiceKind
	keyKind := ServiceKindTypical
	if r.UseServiceKind {
		keyKind = r.ServiceKind
	}
	// To calculate the cache key we only hash the node meta filters and the bexpr filter.
	// The datacenter is handled by the cache framework. The other fields are
	// not, but should not be used in any cache types.
	v, err := hashstructure.Hash([]interface{}{
		keyKind,
		r.UseServiceKind,
		r.Filter,
		r.EnterpriseMeta,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

func (r *ServiceDumpRequest) CacheMinIndex() uint64 {
	return r.QueryOptions.MinQueryIndex
}

// ServiceSpecificRequest is used to query about a specific service
type ServiceSpecificRequest struct {
	Datacenter      string
	NodeMetaFilters map[string]string
	ServiceName     string
	// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
	// with 1.2.x is not required.
	ServiceTag     string
	ServiceTags    []string
	ServiceAddress string
	TagFilter      bool // Controls tag filtering
	Source         QuerySource

	// Connect if true will only search for Connect-compatible services.
	Connect bool

	// Ingress if true will only search for Ingress gateways for the given service.
	Ingress bool

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (r *ServiceSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

func (r *ServiceSpecificRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	// To calculate the cache key we hash over all the fields that affect the
	// output other than Datacenter and Token which are dealt with in the cache
	// framework already. Note the order here is important for the outcome - if we
	// ever care about cache-invalidation on updates e.g. because we persist
	// cached results, we need to be careful we maintain the same order of fields
	// here. We could alternatively use `hash:set` struct tag on an anonymous
	// struct to make it more robust if it becomes significant.
	sort.Strings(r.ServiceTags)
	v, err := hashstructure.Hash([]interface{}{
		r.NodeMetaFilters,
		r.ServiceName,
		// DEPRECATED (singular-service-tag) - remove this when upgrade RPC compat
		// with 1.2.x is not required. We still need this in because <1.3 agents
		// might still send RPCs with singular tag set. In fact the only place we
		// use this method is in agent cache so if the agent is new enough to have
		// this code this should never be set, but it's safer to include it until we
		// completely remove this field just in case it's erroneously used anywhere
		// (e.g. until this change DNS still used it).
		r.ServiceTag,
		r.ServiceTags,
		r.ServiceAddress,
		r.TagFilter,
		r.Connect,
		r.Filter,
		r.EnterpriseMeta,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

func (r *ServiceSpecificRequest) CacheMinIndex() uint64 {
	return r.QueryOptions.MinQueryIndex
}

// NodeSpecificRequest is used to request the information about a single node
type NodeSpecificRequest struct {
	Datacenter     string
	Node           string
	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (r *NodeSpecificRequest) RequestDatacenter() string {
	return r.Datacenter
}

func (r *NodeSpecificRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     r.Datacenter,
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	v, err := hashstructure.Hash([]interface{}{
		r.Node,
		r.Filter,
		r.EnterpriseMeta,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// ChecksInStateRequest is used to query for nodes in a state
type ChecksInStateRequest struct {
	Datacenter      string
	NodeMetaFilters map[string]string
	State           string
	Source          QuerySource

	EnterpriseMeta `mapstructure:",squash"`
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

	RaftIndex `bexpr:"-"`
}

func (n *Node) BestAddress(wan bool) string {
	if wan {
		if addr, ok := n.TaggedAddresses[TaggedAddressWAN]; ok {
			return addr
		}
	}
	return n.Address
}

type Nodes []*Node

// IsSame return whether nodes are similar without taking into account
// RaftIndex fields.
func (n *Node) IsSame(other *Node) bool {
	return n.ID == other.ID &&
		n.Node == other.Node &&
		n.Address == other.Address &&
		n.Datacenter == other.Datacenter &&
		reflect.DeepEqual(n.TaggedAddresses, other.TaggedAddresses) &&
		reflect.DeepEqual(n.Meta, other.Meta)
}

// ValidateNodeMetadata validates a set of key/value pairs from the agent
// config for use on a Node.
func ValidateNodeMetadata(meta map[string]string, allowConsulPrefix bool) error {
	return validateMetadata(meta, allowConsulPrefix, nil)
}

// ValidateServiceMetadata validates a set of key/value pairs from the agent config for use on a Service.
// ValidateMeta validates a set of key/value pairs from the agent config
func ValidateServiceMetadata(kind ServiceKind, meta map[string]string, allowConsulPrefix bool) error {
	switch kind {
	case ServiceKindMeshGateway:
		return validateMetadata(meta, allowConsulPrefix, allowedConsulMetaKeysForMeshGateway)
	default:
		return validateMetadata(meta, allowConsulPrefix, nil)
	}
}

func validateMetadata(meta map[string]string, allowConsulPrefix bool, allowedConsulKeys map[string]struct{}) error {
	if len(meta) > metaMaxKeyPairs {
		return fmt.Errorf("Node metadata cannot contain more than %d key/value pairs", metaMaxKeyPairs)
	}

	for key, value := range meta {
		if err := validateMetaPair(key, value, allowConsulPrefix, allowedConsulKeys); err != nil {
			return fmt.Errorf("Couldn't load metadata pair ('%s', '%s'): %s", key, value, err)
		}
	}

	return nil
}

// ValidateWeights checks the definition of DNS weight is valid
func ValidateWeights(weights *Weights) error {
	if weights == nil {
		return nil
	}
	if weights.Passing < 1 {
		return fmt.Errorf("Passing must be greater than 0")
	}
	if weights.Warning < 0 {
		return fmt.Errorf("Warning must be greater or equal than 0")
	}
	if weights.Passing > 65535 || weights.Warning > 65535 {
		return fmt.Errorf("DNS Weight must be between 0 and 65535")
	}
	return nil
}

// validateMetaPair checks that the given key/value pair is in a valid format
func validateMetaPair(key, value string, allowConsulPrefix bool, allowedConsulKeys map[string]struct{}) error {
	if key == "" {
		return fmt.Errorf("Key cannot be blank")
	}
	if !metaKeyFormat(key) {
		return fmt.Errorf("Key contains invalid characters")
	}
	if len(key) > metaKeyMaxLength {
		return fmt.Errorf("Key is too long (limit: %d characters)", metaKeyMaxLength)
	}
	if strings.HasPrefix(key, metaKeyReservedPrefix) {
		if _, ok := allowedConsulKeys[key]; !allowConsulPrefix && !ok {
			return fmt.Errorf("Key prefix '%s' is reserved for internal use", metaKeyReservedPrefix)
		}
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
	ServiceKind              ServiceKind
	ServiceID                string
	ServiceName              string
	ServiceTags              []string
	ServiceAddress           string
	ServiceTaggedAddresses   map[string]ServiceAddress `json:",omitempty"`
	ServiceWeights           Weights
	ServiceMeta              map[string]string
	ServicePort              int
	ServiceEnableTagOverride bool
	ServiceProxy             ConnectProxyConfig
	ServiceConnect           ServiceConnect

	EnterpriseMeta `hcl:",squash" mapstructure:",squash" bexpr:"-"`

	RaftIndex `bexpr:"-"`
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

	var svcTaggedAddrs map[string]ServiceAddress
	if len(s.ServiceTaggedAddresses) > 0 {
		svcTaggedAddrs = make(map[string]ServiceAddress)
		for k, v := range s.ServiceTaggedAddresses {
			svcTaggedAddrs[k] = v
		}
	}

	return &ServiceNode{
		// Skip ID, see above.
		Node: s.Node,
		// Skip Address, see above.
		// Skip TaggedAddresses, see above.
		ServiceKind:              s.ServiceKind,
		ServiceID:                s.ServiceID,
		ServiceName:              s.ServiceName,
		ServiceTags:              tags,
		ServiceAddress:           s.ServiceAddress,
		ServiceTaggedAddresses:   svcTaggedAddrs,
		ServicePort:              s.ServicePort,
		ServiceMeta:              nsmeta,
		ServiceWeights:           s.ServiceWeights,
		ServiceEnableTagOverride: s.ServiceEnableTagOverride,
		ServiceProxy:             s.ServiceProxy,
		ServiceConnect:           s.ServiceConnect,
		RaftIndex: RaftIndex{
			CreateIndex: s.CreateIndex,
			ModifyIndex: s.ModifyIndex,
		},
		EnterpriseMeta: s.EnterpriseMeta,
	}
}

// ToNodeService converts the given service node to a node service.
func (s *ServiceNode) ToNodeService() *NodeService {
	return &NodeService{
		Kind:              s.ServiceKind,
		ID:                s.ServiceID,
		Service:           s.ServiceName,
		Tags:              s.ServiceTags,
		Address:           s.ServiceAddress,
		TaggedAddresses:   s.ServiceTaggedAddresses,
		Port:              s.ServicePort,
		Meta:              s.ServiceMeta,
		Weights:           &s.ServiceWeights,
		EnableTagOverride: s.ServiceEnableTagOverride,
		Proxy:             s.ServiceProxy,
		Connect:           s.ServiceConnect,
		EnterpriseMeta:    s.EnterpriseMeta,
		RaftIndex: RaftIndex{
			CreateIndex: s.CreateIndex,
			ModifyIndex: s.ModifyIndex,
		},
	}
}

func (sn *ServiceNode) CompoundServiceID() ServiceID {
	id := sn.ServiceID
	if id == "" {
		id = sn.ServiceName
	}

	// copy the ent meta and normalize it
	entMeta := sn.EnterpriseMeta
	entMeta.Normalize()

	return ServiceID{
		ID:             id,
		EnterpriseMeta: entMeta,
	}
}

func (sn *ServiceNode) CompoundServiceName() ServiceName {
	name := sn.ServiceName
	if name == "" {
		name = sn.ServiceID
	}

	// copy the ent meta and normalize it
	entMeta := sn.EnterpriseMeta
	entMeta.Normalize()

	return ServiceName{
		Name:           name,
		EnterpriseMeta: entMeta,
	}
}

// Weights represent the weight used by DNS for a given status
type Weights struct {
	Passing int
	Warning int
}

type ServiceNodes []*ServiceNode

// ServiceKind is the kind of service being registered.
type ServiceKind string

const (
	// ServiceKindTypical is a typical, classic Consul service. This is
	// represented by the absence of a value. This was chosen for ease of
	// backwards compatibility: existing services in the catalog would
	// default to the typical service.
	ServiceKindTypical ServiceKind = ""

	// ServiceKindConnectProxy is a proxy for the Connect feature. This
	// service proxies another service within Consul and speaks the connect
	// protocol.
	ServiceKindConnectProxy ServiceKind = "connect-proxy"

	// ServiceKindMeshGateway is a Mesh Gateway for the Connect feature. This
	// service will proxy connections based off the SNI header set by other
	// connect proxies
	ServiceKindMeshGateway ServiceKind = "mesh-gateway"

	// ServiceKindTerminatingGateway is a Terminating Gateway for the Connect
	// feature. This service will proxy connections to services outside the mesh.
	ServiceKindTerminatingGateway ServiceKind = "terminating-gateway"

	// ServiceKindIngressGateway is an Ingress Gateway for the Connect feature.
	// This service allows external traffic to enter the mesh based on
	// centralized configuration.
	ServiceKindIngressGateway ServiceKind = "ingress-gateway"
)

// Type to hold a address and port of a service
type ServiceAddress struct {
	Address string
	Port    int
}

func (a ServiceAddress) ToAPIServiceAddress() api.ServiceAddress {
	return api.ServiceAddress{Address: a.Address, Port: a.Port}
}

// NodeService is a service provided by a node
type NodeService struct {
	// Kind is the kind of service this is. Different kinds of services may
	// have differing validation, DNS behavior, etc. An empty kind will default
	// to the Default kind. See ServiceKind for the full list of kinds.
	Kind ServiceKind `json:",omitempty"`

	ID                string
	Service           string
	Tags              []string
	Address           string
	TaggedAddresses   map[string]ServiceAddress `json:",omitempty"`
	Meta              map[string]string
	Port              int
	Weights           *Weights
	EnableTagOverride bool

	// Proxy is the configuration set for Kind = connect-proxy. It is mandatory in
	// that case and an error to be set for any other kind. This config is part of
	// a proxy service definition. ProxyConfig may be a more natural name here, but
	// it's confusing for the UX because one of the fields in ConnectProxyConfig is
	// also called just "Config"
	Proxy ConnectProxyConfig

	// Connect are the Connect settings for a service. This is purposely NOT
	// a pointer so that we never have to nil-check this.
	Connect ServiceConnect

	// LocallyRegisteredAsSidecar is private as it is only used by a local agent
	// state to track if the service was registered from a nested sidecar_service
	// block. We need to track that so we can know whether we need to deregister
	// it automatically too if it's removed from the service definition or if the
	// parent service is deregistered. Relying only on ID would cause us to
	// deregister regular services if they happen to be registered using the same
	// ID scheme as our sidecars do by default. We could use meta but that gets
	// unpleasant because we can't use the consul- prefix from an agent (reserved
	// for use internally but in practice that means within the state store or in
	// responses only), and it leaks the detail publicly which people might rely
	// on which is a bit unpleasant for something that is meant to be config-file
	// syntax sugar. Note this is not translated to ServiceNode and friends and
	// may not be set on a NodeService that isn't the one the agent registered and
	// keeps in it's local state. We never want this rendered in JSON as it's
	// internal only. Right now our agent endpoints return api structs which don't
	// include it but this is a safety net incase we change that or there is
	// somewhere this is used in API output.
	LocallyRegisteredAsSidecar bool `json:"-" bexpr:"-"`

	EnterpriseMeta `hcl:",squash" mapstructure:",squash" bexpr:"-"`

	RaftIndex `bexpr:"-"`
}

func (ns *NodeService) BestAddress(wan bool) (string, int) {
	addr := ns.Address
	port := ns.Port

	if wan {
		if wan, ok := ns.TaggedAddresses[TaggedAddressWAN]; ok {
			addr = wan.Address
			if wan.Port != 0 {
				port = wan.Port
			}
		}
	}
	return addr, port
}

func (ns *NodeService) CompoundServiceID() ServiceID {
	id := ns.ID
	if id == "" {
		id = ns.Service
	}

	// copy the ent meta and normalize it
	entMeta := ns.EnterpriseMeta
	entMeta.Normalize()

	return ServiceID{
		ID:             id,
		EnterpriseMeta: entMeta,
	}
}

func (ns *NodeService) CompoundServiceName() ServiceName {
	name := ns.Service
	if name == "" {
		name = ns.ID
	}

	// copy the ent meta and normalize it
	entMeta := ns.EnterpriseMeta
	entMeta.Normalize()

	return ServiceName{
		Name:           name,
		EnterpriseMeta: entMeta,
	}
}

// ServiceConnect are the shared Connect settings between all service
// definitions from the agent to the state store.
type ServiceConnect struct {
	// Native is true when this service can natively understand Connect.
	Native bool `json:",omitempty"`

	// SidecarService is a nested Service Definition to register at the same time.
	// It's purely a convenience mechanism to allow specifying a sidecar service
	// along with the application service definition. It's nested nature allows
	// all of the fields to be defaulted which can reduce the amount of
	// boilerplate needed to register a sidecar service separately, but the end
	// result is identical to just making a second service registration via any
	// other means.
	SidecarService *ServiceDefinition `json:",omitempty" bexpr:"-"`
}

func (t *ServiceConnect) UnmarshalJSON(data []byte) (err error) {
	type Alias ServiceConnect
	aux := &struct {
		SidecarServiceSnake *ServiceDefinition `json:"sidecar_service"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if t.SidecarService == nil && aux != nil {
		t.SidecarService = aux.SidecarServiceSnake
	}
	return nil
}

// IsSidecarProxy returns true if the NodeService is a sidecar proxy.
func (s *NodeService) IsSidecarProxy() bool {
	return s.Kind == ServiceKindConnectProxy && s.Proxy.DestinationServiceID != ""
}

func (s *NodeService) IsGateway() bool {
	return s.Kind == ServiceKindMeshGateway ||
		s.Kind == ServiceKindTerminatingGateway ||
		s.Kind == ServiceKindIngressGateway
}

// Validate validates the node service configuration.
//
// NOTE(mitchellh): This currently only validates fields for a ConnectProxy.
// Historically validation has been directly in the Catalog.Register RPC.
// ConnectProxy validation was moved here for easier table testing, but
// other validation still exists in Catalog.Register.
func (s *NodeService) Validate() error {
	var result error

	// ConnectProxy validation
	if s.Kind == ServiceKindConnectProxy {
		if strings.TrimSpace(s.Proxy.DestinationServiceName) == "" {
			result = multierror.Append(result, fmt.Errorf(
				"Proxy.DestinationServiceName must be non-empty for Connect proxy "+
					"services"))
		}

		if s.Port == 0 {
			result = multierror.Append(result, fmt.Errorf(
				"Port must be set for a Connect proxy"))
		}

		if s.Connect.Native {
			result = multierror.Append(result, fmt.Errorf(
				"A Proxy cannot also be Connect Native, only typical services"))
		}

		// ensure we don't have multiple upstreams for the same service
		var (
			upstreamKeys = make(map[UpstreamKey]struct{})
			bindAddrs    = make(map[string]struct{})
		)
		for _, u := range s.Proxy.Upstreams {
			if err := u.Validate(); err != nil {
				result = multierror.Append(result, err)
				continue
			}

			uk := u.ToKey()
			if _, ok := upstreamKeys[uk]; ok {
				result = multierror.Append(result, fmt.Errorf(
					"upstreams cannot contain duplicates of %s", uk))
				continue
			}
			upstreamKeys[uk] = struct{}{}

			addr := u.LocalBindAddress
			if addr == "" {
				addr = "127.0.0.1"
			}
			addr = net.JoinHostPort(addr, fmt.Sprintf("%d", u.LocalBindPort))

			if _, ok := bindAddrs[addr]; ok {
				result = multierror.Append(result, fmt.Errorf(
					"upstreams cannot contain duplicates by local bind address and port; %q is specified twice", addr))
				continue
			}
			bindAddrs[addr] = struct{}{}
		}
		var knownPaths = make(map[string]bool)
		var knownListeners = make(map[int]bool)
		for _, path := range s.Proxy.Expose.Paths {
			if path.Path == "" {
				result = multierror.Append(result, fmt.Errorf("expose.paths: empty path exposed"))
			}

			if seen := knownPaths[path.Path]; seen {
				result = multierror.Append(result, fmt.Errorf("expose.paths: duplicate paths exposed"))
			}
			knownPaths[path.Path] = true

			if seen := knownListeners[path.ListenerPort]; seen {
				result = multierror.Append(result, fmt.Errorf("expose.paths: duplicate listener ports exposed"))
			}
			knownListeners[path.ListenerPort] = true

			if path.ListenerPort <= 0 || path.ListenerPort > 65535 {
				result = multierror.Append(result, fmt.Errorf("expose.paths: invalid listener port: %d", path.ListenerPort))
			}

			path.Protocol = strings.ToLower(path.Protocol)
			if ok := allowedExposeProtocols[path.Protocol]; !ok && path.Protocol != "" {
				protocols := make([]string, 0)
				for p := range allowedExposeProtocols {
					protocols = append(protocols, p)
				}

				result = multierror.Append(result,
					fmt.Errorf("protocol '%s' not supported for path: %s, must be in: %v",
						path.Protocol, path.Path, protocols))
			}
		}
	}

	// Gateway validation
	if s.IsGateway() {
		// Non-ingress gateways must have a port
		if s.Port == 0 && s.Kind != ServiceKindIngressGateway {
			result = multierror.Append(result, fmt.Errorf("Port must be non-zero for a %s", s.Kind))
		}

		// Gateways cannot have sidecars
		if s.Connect.SidecarService != nil {
			result = multierror.Append(result, fmt.Errorf("A %s cannot have a sidecar service defined", s.Kind))
		}

		if s.Proxy.DestinationServiceName != "" {
			result = multierror.Append(result, fmt.Errorf("The Proxy.DestinationServiceName configuration is invalid for a %s", s.Kind))
		}

		if s.Proxy.DestinationServiceID != "" {
			result = multierror.Append(result, fmt.Errorf("The Proxy.DestinationServiceID configuration is invalid for a %s", s.Kind))
		}

		if s.Proxy.LocalServiceAddress != "" {
			result = multierror.Append(result, fmt.Errorf("The Proxy.LocalServiceAddress configuration is invalid for a %s", s.Kind))
		}

		if s.Proxy.LocalServicePort != 0 {
			result = multierror.Append(result, fmt.Errorf("The Proxy.LocalServicePort configuration is invalid for a %s", s.Kind))
		}

		if len(s.Proxy.Upstreams) != 0 {
			result = multierror.Append(result, fmt.Errorf("The Proxy.Upstreams configuration is invalid for a %s", s.Kind))
		}
	}

	// Nested sidecar validation
	if s.Connect.SidecarService != nil {
		if s.Connect.SidecarService.ID != "" {
			result = multierror.Append(result, fmt.Errorf(
				"A SidecarService cannot specify an ID as this is managed by the "+
					"agent"))
		}
		if s.Connect.SidecarService.Connect != nil {
			if s.Connect.SidecarService.Connect.SidecarService != nil {
				result = multierror.Append(result, fmt.Errorf(
					"A SidecarService cannot have a nested SidecarService"))
			}
		}
	}

	return result
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
		!reflect.DeepEqual(s.TaggedAddresses, other.TaggedAddresses) ||
		!reflect.DeepEqual(s.Weights, other.Weights) ||
		!reflect.DeepEqual(s.Meta, other.Meta) ||
		s.EnableTagOverride != other.EnableTagOverride ||
		s.Kind != other.Kind ||
		!reflect.DeepEqual(s.Proxy, other.Proxy) ||
		s.Connect != other.Connect ||
		!s.EnterpriseMeta.IsSame(&other.EnterpriseMeta) {
		return false
	}

	return true
}

// IsSameService checks if one Service of a ServiceNode is the same as another,
// without looking at the Raft information or Node information (that's why we
// didn't call it IsEqual).
// This is useful for seeing if an update would be idempotent for all the functional
// parts of the structure.
// In a similar fashion as ToNodeService(), fields related to Node are ignored
// see ServiceNode for more information.
func (s *ServiceNode) IsSameService(other *ServiceNode) bool {
	// Skip the following fields, see ServiceNode definition
	// Address                  string
	// Datacenter               string
	// TaggedAddresses          map[string]string
	// NodeMeta                 map[string]string
	if s.ID != other.ID ||
		s.Node != other.Node ||
		s.ServiceKind != other.ServiceKind ||
		s.ServiceID != other.ServiceID ||
		s.ServiceName != other.ServiceName ||
		!reflect.DeepEqual(s.ServiceTags, other.ServiceTags) ||
		s.ServiceAddress != other.ServiceAddress ||
		!reflect.DeepEqual(s.ServiceTaggedAddresses, other.ServiceTaggedAddresses) ||
		s.ServicePort != other.ServicePort ||
		!reflect.DeepEqual(s.ServiceMeta, other.ServiceMeta) ||
		!reflect.DeepEqual(s.ServiceWeights, other.ServiceWeights) ||
		s.ServiceEnableTagOverride != other.ServiceEnableTagOverride ||
		!reflect.DeepEqual(s.ServiceProxy, other.ServiceProxy) ||
		!reflect.DeepEqual(s.ServiceConnect, other.ServiceConnect) ||
		!s.EnterpriseMeta.IsSame(&other.EnterpriseMeta) {
		return false
	}

	return true
}

// ToServiceNode converts the given node service to a service node.
func (s *NodeService) ToServiceNode(node string) *ServiceNode {
	theWeights := Weights{
		Passing: 1,
		Warning: 1,
	}
	if s.Weights != nil {
		if err := ValidateWeights(s.Weights); err == nil {
			theWeights = *s.Weights
		}
	}
	return &ServiceNode{
		// Skip ID, see ServiceNode definition.
		Node: node,
		// Skip Address, see ServiceNode definition.
		// Skip TaggedAddresses, see ServiceNode definition.
		ServiceKind:              s.Kind,
		ServiceID:                s.ID,
		ServiceName:              s.Service,
		ServiceTags:              s.Tags,
		ServiceAddress:           s.Address,
		ServiceTaggedAddresses:   s.TaggedAddresses,
		ServicePort:              s.Port,
		ServiceMeta:              s.Meta,
		ServiceWeights:           theWeights,
		ServiceEnableTagOverride: s.EnableTagOverride,
		ServiceProxy:             s.Proxy,
		ServiceConnect:           s.Connect,
		EnterpriseMeta:           s.EnterpriseMeta,
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

type NodeServiceList struct {
	Node     *Node
	Services []*NodeService
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
	Type        string        // Check type: http/ttl/tcp/etc

	Definition HealthCheckDefinition `bexpr:"-"`

	EnterpriseMeta `hcl:",squash" mapstructure:",squash" bexpr:"-"`

	RaftIndex `bexpr:"-"`
}

func (hc *HealthCheck) CompoundServiceID() ServiceID {
	id := hc.ServiceID
	if id == "" {
		id = hc.ServiceName
	}

	entMeta := hc.EnterpriseMeta
	entMeta.Normalize()

	return ServiceID{
		ID:             id,
		EnterpriseMeta: entMeta,
	}
}

func (hc *HealthCheck) CompoundCheckID() CheckID {
	entMeta := hc.EnterpriseMeta
	entMeta.Normalize()

	return CheckID{
		ID:             hc.CheckID,
		EnterpriseMeta: entMeta,
	}
}

type HealthCheckDefinition struct {
	HTTP                           string              `json:",omitempty"`
	TLSSkipVerify                  bool                `json:",omitempty"`
	Header                         map[string][]string `json:",omitempty"`
	Method                         string              `json:",omitempty"`
	Body                           string              `json:",omitempty"`
	TCP                            string              `json:",omitempty"`
	Interval                       time.Duration       `json:",omitempty"`
	OutputMaxSize                  uint                `json:",omitempty"`
	Timeout                        time.Duration       `json:",omitempty"`
	DeregisterCriticalServiceAfter time.Duration       `json:",omitempty"`
	ScriptArgs                     []string            `json:",omitempty"`
	DockerContainerID              string              `json:",omitempty"`
	Shell                          string              `json:",omitempty"`
	GRPC                           string              `json:",omitempty"`
	GRPCUseTLS                     bool                `json:",omitempty"`
	AliasNode                      string              `json:",omitempty"`
	AliasService                   string              `json:",omitempty"`
	TTL                            time.Duration       `json:",omitempty"`
}

func (d *HealthCheckDefinition) MarshalJSON() ([]byte, error) {
	type Alias HealthCheckDefinition
	exported := &struct {
		Interval                       string `json:",omitempty"`
		OutputMaxSize                  uint   `json:",omitempty"`
		Timeout                        string `json:",omitempty"`
		DeregisterCriticalServiceAfter string `json:",omitempty"`
		*Alias
	}{
		Interval:                       d.Interval.String(),
		OutputMaxSize:                  d.OutputMaxSize,
		Timeout:                        d.Timeout.String(),
		DeregisterCriticalServiceAfter: d.DeregisterCriticalServiceAfter.String(),
		Alias:                          (*Alias)(d),
	}
	if d.Interval == 0 {
		exported.Interval = ""
	}
	if d.Timeout == 0 {
		exported.Timeout = ""
	}
	if d.DeregisterCriticalServiceAfter == 0 {
		exported.DeregisterCriticalServiceAfter = ""
	}

	return json.Marshal(exported)
}

func (t *HealthCheckDefinition) UnmarshalJSON(data []byte) (err error) {
	type Alias HealthCheckDefinition
	aux := &struct {
		Interval                       interface{}
		Timeout                        interface{}
		DeregisterCriticalServiceAfter interface{}
		TTL                            interface{}
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Interval != nil {
		switch v := aux.Interval.(type) {
		case string:
			if t.Interval, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.Interval = time.Duration(v)
		}
	}
	if aux.Timeout != nil {
		switch v := aux.Timeout.(type) {
		case string:
			if t.Timeout, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.Timeout = time.Duration(v)
		}
	}
	if aux.DeregisterCriticalServiceAfter != nil {
		switch v := aux.DeregisterCriticalServiceAfter.(type) {
		case string:
			if t.DeregisterCriticalServiceAfter, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.DeregisterCriticalServiceAfter = time.Duration(v)
		}
	}
	if aux.TTL != nil {
		switch v := aux.TTL.(type) {
		case string:
			if t.TTL, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			t.TTL = time.Duration(v)
		}
	}
	return nil
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
		!reflect.DeepEqual(c.ServiceTags, other.ServiceTags) ||
		!reflect.DeepEqual(c.Definition, other.Definition) ||
		!c.EnterpriseMeta.IsSame(&other.EnterpriseMeta) {
		return false
	}

	return true
}

// Clone returns a distinct clone of the HealthCheck. Note that the
// "ServiceTags" and "Definition.Header" field are not deep copied.
func (c *HealthCheck) Clone() *HealthCheck {
	clone := new(HealthCheck)
	*clone = *c
	return clone
}

func (c *HealthCheck) CheckType() *CheckType {
	return &CheckType{
		CheckID: c.CheckID,
		Name:    c.Name,
		Status:  c.Status,
		Notes:   c.Notes,

		ScriptArgs:                     c.Definition.ScriptArgs,
		AliasNode:                      c.Definition.AliasNode,
		AliasService:                   c.Definition.AliasService,
		HTTP:                           c.Definition.HTTP,
		GRPC:                           c.Definition.GRPC,
		GRPCUseTLS:                     c.Definition.GRPCUseTLS,
		Header:                         c.Definition.Header,
		Method:                         c.Definition.Method,
		Body:                           c.Definition.Body,
		TCP:                            c.Definition.TCP,
		Interval:                       c.Definition.Interval,
		DockerContainerID:              c.Definition.DockerContainerID,
		Shell:                          c.Definition.Shell,
		TLSSkipVerify:                  c.Definition.TLSSkipVerify,
		Timeout:                        c.Definition.Timeout,
		TTL:                            c.Definition.TTL,
		DeregisterCriticalServiceAfter: c.Definition.DeregisterCriticalServiceAfter,
	}
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

func (csn *CheckServiceNode) BestAddress(wan bool) (string, int) {
	// TODO (mesh-gateway) needs a test
	// best address
	// wan
	//   wan svc addr
	//   svc addr
	//   wan node addr
	//   node addr
	// lan
	//   svc addr
	//   node addr

	addr, port := csn.Service.BestAddress(wan)

	if addr == "" {
		addr = csn.Node.BestAddress(wan)
	}

	return addr, port
}

type CheckServiceNodes []CheckServiceNode

// Shuffle does an in-place random shuffle using the Fisher-Yates algorithm.
func (nodes CheckServiceNodes) Shuffle() {
	for i := len(nodes) - 1; i > 0; i-- {
		j := rand.Int31n(int32(i + 1))
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
}

func (nodes CheckServiceNodes) ToServiceDump() ServiceDump {
	var ret ServiceDump
	for i := range nodes {
		svc := ServiceInfo{
			Node:           nodes[i].Node,
			Service:        nodes[i].Service,
			Checks:         nodes[i].Checks,
			GatewayService: nil,
		}
		ret = append(ret, &svc)
	}
	return ret
}

// ShallowClone duplicates the slice and underlying array.
func (nodes CheckServiceNodes) ShallowClone() CheckServiceNodes {
	dup := make(CheckServiceNodes, len(nodes))
	copy(dup, nodes)
	return dup
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

type ServiceInfo struct {
	Node           *Node
	Service        *NodeService
	Checks         HealthChecks
	GatewayService *GatewayService
}

type ServiceDump []*ServiceInfo

type CheckID struct {
	ID types.CheckID
	EnterpriseMeta
}

func NewCheckID(id types.CheckID, entMeta *EnterpriseMeta) CheckID {
	var cid CheckID
	cid.ID = id
	if entMeta == nil {
		entMeta = DefaultEnterpriseMeta()
	}

	cid.EnterpriseMeta = *entMeta
	cid.EnterpriseMeta.Normalize()
	return cid
}

// StringHash is used mainly to populate part of the filename of a check
// definition persisted on the local agent
func (cid *CheckID) StringHash() string {
	hasher := md5.New()
	hasher.Write([]byte(cid.ID))
	cid.EnterpriseMeta.addToHash(hasher, true)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

type ServiceID struct {
	ID string
	EnterpriseMeta
}

func NewServiceID(id string, entMeta *EnterpriseMeta) ServiceID {
	var sid ServiceID
	sid.ID = id
	if entMeta == nil {
		entMeta = DefaultEnterpriseMeta()
	}

	sid.EnterpriseMeta = *entMeta
	sid.EnterpriseMeta.Normalize()
	return sid
}

func (sid *ServiceID) Matches(other *ServiceID) bool {
	if sid == nil && other == nil {
		return true
	}

	if sid == nil || other == nil || sid.ID != other.ID || !sid.EnterpriseMeta.Matches(&other.EnterpriseMeta) {
		return false
	}

	return true
}

// StringHash is used mainly to populate part of the filename of a service
// definition persisted on the local agent
func (sid *ServiceID) StringHash() string {
	hasher := md5.New()
	hasher.Write([]byte(sid.ID))
	sid.EnterpriseMeta.addToHash(hasher, true)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

func (sid *ServiceID) LessThan(other *ServiceID) bool {
	if sid.EnterpriseMeta.LessThan(&other.EnterpriseMeta) {
		return true
	}

	return sid.ID < other.ID
}

type IndexedNodes struct {
	Nodes Nodes
	QueryMeta
}

type IndexedServices struct {
	Services Services
	// In various situations we need to know the meta that the services are for - in particular
	// this is needed to be able to properly filter the list based on ACLs
	EnterpriseMeta
	QueryMeta
}

type ServiceName struct {
	Name string
	EnterpriseMeta
}

func NewServiceName(name string, entMeta *EnterpriseMeta) ServiceName {
	var ret ServiceName
	ret.Name = name
	if entMeta == nil {
		entMeta = DefaultEnterpriseMeta()
	}

	ret.EnterpriseMeta = *entMeta
	ret.EnterpriseMeta.Normalize()
	return ret
}

func (n *ServiceName) Matches(o *ServiceName) bool {
	if n == nil && o == nil {
		return true
	}

	if n == nil || o == nil || n.Name != o.Name || !n.EnterpriseMeta.Matches(&o.EnterpriseMeta) {
		return false
	}

	return true
}

func (si *ServiceName) ToServiceID() ServiceID {
	return ServiceID{ID: si.Name, EnterpriseMeta: si.EnterpriseMeta}
}

type ServiceList []ServiceName

type IndexedServiceList struct {
	Services ServiceList
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

type IndexedNodeServiceList struct {
	NodeServices NodeServiceList
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

type DatacenterIndexedCheckServiceNodes struct {
	DatacenterNodes map[string]CheckServiceNodes
	QueryMeta
}

type IndexedNodeDump struct {
	Dump NodeDump
	QueryMeta
}

type IndexedServiceDump struct {
	Dump ServiceDump
	QueryMeta
}

type IndexedGatewayServices struct {
	Services GatewayServices
	QueryMeta
}

// IndexedConfigEntries has its own encoding logic which differs from
// ConfigEntryRequest as it has to send a slice of ConfigEntry.
type IndexedConfigEntries struct {
	Kind    string
	Entries []ConfigEntry
	QueryMeta
}

func (c *IndexedConfigEntries) MarshalBinary() (data []byte, err error) {
	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, MsgpackHandle)

	// Encode length.
	err = enc.Encode(len(c.Entries))
	if err != nil {
		return nil, err
	}

	// Encode kind.
	err = enc.Encode(c.Kind)
	if err != nil {
		return nil, err
	}

	// Then actual value using alias trick to avoid infinite recursion
	type Alias IndexedConfigEntries
	err = enc.Encode(struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	})
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func (c *IndexedConfigEntries) UnmarshalBinary(data []byte) error {
	// First decode the number of entries.
	var numEntries int
	dec := codec.NewDecoderBytes(data, MsgpackHandle)
	if err := dec.Decode(&numEntries); err != nil {
		return err
	}

	// Next decode the kind.
	var kind string
	if err := dec.Decode(&kind); err != nil {
		return err
	}

	// Then decode the slice of ConfigEntries
	c.Entries = make([]ConfigEntry, numEntries)
	for i := 0; i < numEntries; i++ {
		entry, err := MakeConfigEntry(kind, "")
		if err != nil {
			return err
		}
		c.Entries[i] = entry
	}

	// Alias juggling to prevent infinite recursive calls back to this decode
	// method.
	type Alias IndexedConfigEntries
	as := struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := dec.Decode(&as); err != nil {
		return err
	}
	return nil
}

type IndexedGenericConfigEntries struct {
	Entries []ConfigEntry
	QueryMeta
}

func (c *IndexedGenericConfigEntries) MarshalBinary() (data []byte, err error) {
	// bs will grow if needed but allocate enough to avoid reallocation in common
	// case.
	bs := make([]byte, 128)
	enc := codec.NewEncoderBytes(&bs, MsgpackHandle)

	if err := enc.Encode(len(c.Entries)); err != nil {
		return nil, err
	}

	for _, entry := range c.Entries {
		if err := enc.Encode(entry.GetKind()); err != nil {
			return nil, err
		}
		if err := enc.Encode(entry); err != nil {
			return nil, err
		}
	}

	if err := enc.Encode(c.QueryMeta); err != nil {
		return nil, err
	}

	return bs, nil
}

func (c *IndexedGenericConfigEntries) UnmarshalBinary(data []byte) error {
	// First decode the number of entries.
	var numEntries int
	dec := codec.NewDecoderBytes(data, MsgpackHandle)
	if err := dec.Decode(&numEntries); err != nil {
		return err
	}

	// Then decode the slice of ConfigEntries
	c.Entries = make([]ConfigEntry, numEntries)
	for i := 0; i < numEntries; i++ {
		var kind string
		if err := dec.Decode(&kind); err != nil {
			return err
		}

		entry, err := MakeConfigEntry(kind, "")
		if err != nil {
			return err
		}

		if err := dec.Decode(entry); err != nil {
			return err
		}

		c.Entries[i] = entry
	}

	if err := dec.Decode(&c.QueryMeta); err != nil {
		return err
	}

	return nil

}

// DirEntry is used to represent a directory entry. This is
// used for values in our Key-Value store.
type DirEntry struct {
	LockIndex uint64
	Key       string
	Flags     uint64
	Value     []byte
	Session   string `json:",omitempty"`

	EnterpriseMeta `bexpr:"-"`
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
		EnterpriseMeta: d.EnterpriseMeta,
	}
}

func (d *DirEntry) Equal(o *DirEntry) bool {
	return d.LockIndex == o.LockIndex &&
		d.Key == o.Key &&
		d.Flags == o.Flags &&
		bytes.Equal(d.Value, o.Value) &&
		d.Session == o.Session
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
	EnterpriseMeta
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
	EnterpriseMeta
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

type Sessions []*Session

// Session is used to represent an open session in the KV store.
// This issued to associate node checks with acquired locks.
type Session struct {
	ID            string
	Name          string
	Node          string
	LockDelay     time.Duration
	Behavior      SessionBehavior // What to do when session is invalidated
	TTL           string
	NodeChecks    []string
	ServiceChecks []ServiceCheck

	// Deprecated v1.7.0.
	Checks []types.CheckID `json:",omitempty"`

	EnterpriseMeta
	RaftIndex
}

type ServiceCheck struct {
	ID        string
	Namespace string
}

func (s *Session) UnmarshalJSON(data []byte) (err error) {
	type Alias Session
	aux := &struct {
		LockDelay interface{}
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.LockDelay != nil {
		var dur time.Duration
		switch v := aux.LockDelay.(type) {
		case string:
			if dur, err = time.ParseDuration(v); err != nil {
				return err
			}
		case float64:
			dur = time.Duration(v)
		}
		// Convert low value integers into seconds
		if dur < lockDelayMinThreshold {
			dur = dur * time.Second
		}
		s.LockDelay = dur
	}
	return nil
}

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
	SessionID  string
	// DEPRECATED in 1.7.0
	Session string
	EnterpriseMeta
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

// MsgpackHandle is a shared handle for encoding/decoding msgpack payloads
var MsgpackHandle = &codec.MsgpackHandle{
	RawToString: true,
	BasicHandle: codec.BasicHandle{
		DecodeOptions: codec.DecodeOptions{
			MapType: reflect.TypeOf(map[string]interface{}{}),
		},
	},
}

// Decode is used to decode a MsgPack encoded object
func Decode(buf []byte, out interface{}) error {
	return codec.NewDecoder(bytes.NewReader(buf), MsgpackHandle).Decode(out)
}

// Encode is used to encode a MsgPack object with type prefix
func Encode(t MessageType, msg interface{}) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte(uint8(t))
	err := codec.NewEncoder(&buf, MsgpackHandle).Encode(msg)
	return buf.Bytes(), err
}

type ProtoMarshaller interface {
	Size() int
	MarshalTo([]byte) (int, error)
	Unmarshal([]byte) error
	ProtoMessage()
}

func EncodeProtoInterface(t MessageType, message interface{}) ([]byte, error) {
	if marshaller, ok := message.(ProtoMarshaller); ok {
		return EncodeProto(t, marshaller)
	}

	return nil, fmt.Errorf("message does not implement the ProtoMarshaller interface: %T", message)
}

func EncodeProto(t MessageType, message ProtoMarshaller) ([]byte, error) {
	data := make([]byte, message.Size()+1)
	data[0] = uint8(t)
	if _, err := message.MarshalTo(data[1:]); err != nil {
		return nil, err
	}
	return data, nil
}

func DecodeProto(buf []byte, out ProtoMarshaller) error {
	// Note that this assumes the leading byte indicating the type as already been stripped off.
	return out.Unmarshal(buf)
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
	LocalOnly   bool
	QueryOptions
}

func (r *KeyringRequest) RequestDatacenter() string {
	return r.Datacenter
}

// KeyringResponse is a unified key response and can be used for install,
// remove, use, as well as listing key queries.
type KeyringResponse struct {
	WAN         bool
	Datacenter  string
	Segment     string
	Messages    map[string]string `json:",omitempty"`
	Keys        map[string]int
	PrimaryKeys map[string]int
	NumNodes    int
	Error       string `json:",omitempty"`
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

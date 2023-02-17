package state

import (
	"fmt"
	"net"
	"reflect"
	"strings"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableNodes             = "nodes"
	tableServices          = "services"
	tableChecks            = "checks"
	tableGatewayServices   = "gateway-services"
	tableMeshTopology      = "mesh-topology"
	tableServiceVirtualIPs = "service-virtual-ips"
	tableFreeVirtualIPs    = "free-virtual-ips"
	tableKindServiceNames  = "kind-service-names"

	indexID          = "id"
	indexService     = "service"
	indexConnect     = "connect"
	indexKind        = "kind"
	indexKindOnly    = "kind-only"
	indexStatus      = "status"
	indexNodeService = "node_service"
	indexNode        = "node"
	indexUpstream    = "upstream"
	indexDownstream  = "downstream"
	indexGateway     = "gateway"
	indexUUID        = "uuid"
	indexMeta        = "meta"
	indexCounterOnly = "counter"
)

// nodesTableSchema returns a new table schema used for storing struct.Node.
func nodesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableNodes,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[Query, *structs.Node, any]{
					readIndex:   indexWithPeerName(indexFromQuery),
					writeIndex:  indexWithPeerName(indexFromNode),
					prefixIndex: prefixIndexFromQueryWithPeer,
				},
			},
			indexUUID: {
				Name:         indexUUID,
				AllowMissing: true,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[Query, *structs.Node, Query]{
					readIndex:   indexWithPeerName(indexFromUUIDQuery),
					writeIndex:  indexWithPeerName(indexIDFromNode),
					prefixIndex: prefixIndexFromUUIDWithPeerQuery,
				},
			},
			indexMeta: {
				Name:         indexMeta,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerMulti[KeyValueQuery, *structs.Node]{
					readIndex:       indexWithPeerName(indexFromKeyValueQuery),
					writeIndexMulti: multiIndexWithPeerName(indexMetaFromNode),
				},
			},
		},
	}
}

func indexFromNode(n *structs.Node) ([]byte, error) {
	if n.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(n.Node))
	return b.Bytes(), nil
}

func indexIDFromNode(n *structs.Node) ([]byte, error) {
	if n.ID == "" {
		return nil, errMissingValueForIndex
	}

	v, err := uuidStringToBytes(string(n.ID))
	if err != nil {
		return nil, err
	}

	return v, nil
}

func indexMetaFromNode(n *structs.Node) ([][]byte, error) {
	// NOTE: this is case-sensitive!

	vals := make([][]byte, 0, len(n.Meta))
	for key, val := range n.Meta {
		if key == "" {
			continue
		}

		var b indexBuilder
		b.String(key)
		b.String(val)
		vals = append(vals, b.Bytes())
	}
	if len(vals) == 0 {
		return nil, errMissingValueForIndex
	}

	return vals, nil
}

// servicesTableSchema returns a new table schema used to store information
// about services.
func servicesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableServices,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[NodeServiceQuery, *structs.ServiceNode, any]{
					readIndex:   indexWithPeerName(indexFromNodeServiceQuery),
					writeIndex:  indexWithPeerName(indexFromServiceNode),
					prefixIndex: prefixIndexFromQueryWithPeer,
				},
			},
			indexNode: {
				Name:         indexNode,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[Query, nodeIdentifier]{
					readIndex:  indexWithPeerName(indexFromQuery),
					writeIndex: indexWithPeerName(indexFromNodeIdentity),
				},
			},
			indexService: {
				Name:         indexService,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[Query, *structs.ServiceNode]{
					readIndex:  indexWithPeerName(indexFromQuery),
					writeIndex: indexWithPeerName(indexServiceNameFromServiceNode),
				},
			},
			indexConnect: {
				Name:         indexConnect,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[Query, *structs.ServiceNode]{
					readIndex:  indexWithPeerName(indexFromQuery),
					writeIndex: indexWithPeerName(indexConnectNameFromServiceNode),
				},
			},
			indexKind: {
				Name:         indexKind,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[Query, *structs.ServiceNode]{
					readIndex:  indexWithPeerName(indexFromQuery),
					writeIndex: indexWithPeerName(indexKindFromServiceNode),
				},
			},
		},
	}
}

func indexFromNodeServiceQuery(q NodeServiceQuery) ([]byte, error) {
	var b indexBuilder
	b.String(strings.ToLower(q.Node))
	b.String(strings.ToLower(q.Service))
	return b.Bytes(), nil
}

func indexFromServiceNode(n *structs.ServiceNode) ([]byte, error) {
	if n.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(n.Node))
	b.String(strings.ToLower(n.ServiceID))
	return b.Bytes(), nil
}

type nodeIdentifier interface {
	partitionIndexable
	peerIndexable

	NodeIdentity() structs.Identity
}

var _ nodeIdentifier = (*structs.HealthCheck)(nil)
var _ nodeIdentifier = (*structs.ServiceNode)(nil)

func indexFromNodeIdentity(n nodeIdentifier) ([]byte, error) {
	id := n.NodeIdentity()
	if id.ID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(id.ID))
	return b.Bytes(), nil
}

func indexServiceNameFromServiceNode(n *structs.ServiceNode) ([]byte, error) {
	if n.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(n.ServiceName))
	return b.Bytes(), nil
}

func indexConnectNameFromServiceNode(n *structs.ServiceNode) ([]byte, error) {
	name, ok := connectNameFromServiceNode(n)
	if !ok {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(name))
	return b.Bytes(), nil
}

func connectNameFromServiceNode(sn *structs.ServiceNode) (string, bool) {
	switch {
	case sn.ServiceKind == structs.ServiceKindConnectProxy:
		// For proxies, this service supports Connect for the destination
		return sn.ServiceProxy.DestinationServiceName, true

	case sn.ServiceConnect.Native:
		// For native, this service supports Connect directly
		return sn.ServiceName, true

	default:
		// Doesn't support Connect at all
		return "", false
	}
}

func indexKindFromServiceNode(n *structs.ServiceNode) ([]byte, error) {
	var b indexBuilder
	b.String(strings.ToLower(string(n.ServiceKind)))
	return b.Bytes(), nil
}

// indexWithPeerName adds peer name to the index.
func indexWithPeerName[T peerIndexable](
	fn func(T) ([]byte, error),
) func(T) ([]byte, error) {
	return func(e T) ([]byte, error) {
		v, err := fn(e)
		if err != nil {
			return nil, err
		}

		peername := e.PeerOrEmpty()
		if peername == "" {
			peername = structs.LocalPeerKeyword
		}
		b := newIndexBuilder(len(v) + len(peername) + 1)
		b.String(strings.ToLower(peername))
		b.Raw(v)
		return b.Bytes(), nil
	}
}

// multiIndexWithPeerName adds peer name to multiple indices, and returns multiple indices.
func multiIndexWithPeerName[T any](
	fn func(T) ([][]byte, error),
) func(T) ([][]byte, error) {
	return func(raw T) ([][]byte, error) {
		n, ok := any(raw).(peerIndexable)
		if !ok {
			return nil, fmt.Errorf("type must be peerIndexable: %T", raw)
		}

		results, err := fn(raw)
		if err != nil {
			return nil, err
		}

		peername := n.PeerOrEmpty()
		if peername == "" {
			peername = structs.LocalPeerKeyword
		}
		for i, v := range results {
			b := newIndexBuilder(len(v) + len(peername) + 1)
			b.String(strings.ToLower(peername))
			b.Raw(v)
			results[i] = b.Bytes()
		}
		return results, nil
	}
}

// checksTableSchema returns a new table schema used for storing and indexing
// health check information. Health checks have a number of different attributes
// we want to filter by, so this table is a bit more complex.
func checksTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableChecks,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[NodeCheckQuery, *structs.HealthCheck, any]{
					readIndex:   indexWithPeerName(indexFromNodeCheckQuery),
					writeIndex:  indexWithPeerName(indexFromHealthCheck),
					prefixIndex: prefixIndexFromQueryWithPeer,
				},
			},
			indexStatus: {
				Name:         indexStatus,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[Query, *structs.HealthCheck]{
					readIndex:  indexWithPeerName(indexFromQuery),
					writeIndex: indexWithPeerName(indexStatusFromHealthCheck),
				},
			},
			indexService: {
				Name:         indexService,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[Query, *structs.HealthCheck]{
					readIndex:  indexWithPeerName(indexFromQuery),
					writeIndex: indexWithPeerName(indexServiceNameFromHealthCheck),
				},
			},
			indexNode: {
				Name:         indexNode,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[Query, nodeIdentifier]{
					readIndex:  indexWithPeerName(indexFromQuery),
					writeIndex: indexWithPeerName(indexFromNodeIdentity),
				},
			},
			indexNodeService: {
				Name:         indexNodeService,
				AllowMissing: true,
				Unique:       false,
				Indexer: indexerSingle[NodeServiceQuery, *structs.HealthCheck]{
					readIndex:  indexWithPeerName(indexFromNodeServiceQuery),
					writeIndex: indexWithPeerName(indexNodeServiceFromHealthCheck),
				},
			},
		},
	}
}

func indexFromNodeCheckQuery(q NodeCheckQuery) ([]byte, error) {
	if q.Node == "" || q.CheckID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(q.Node))
	b.String(strings.ToLower(q.CheckID))
	return b.Bytes(), nil
}

func indexFromHealthCheck(hc *structs.HealthCheck) ([]byte, error) {
	if hc.Node == "" || hc.CheckID == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.Node))
	b.String(strings.ToLower(string(hc.CheckID)))
	return b.Bytes(), nil
}

func indexNodeServiceFromHealthCheck(hc *structs.HealthCheck) ([]byte, error) {
	if hc.Node == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.Node))
	b.String(strings.ToLower(hc.ServiceID))
	return b.Bytes(), nil
}

func indexStatusFromHealthCheck(hc *structs.HealthCheck) ([]byte, error) {
	if hc.Status == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.Status))
	return b.Bytes(), nil
}

func indexServiceNameFromHealthCheck(hc *structs.HealthCheck) ([]byte, error) {
	if hc.ServiceName == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(strings.ToLower(hc.ServiceName))
	return b.Bytes(), nil
}

//	gatewayServicesTableSchema returns a new table schema used to store information
//
// about services associated with terminating gateways.
func gatewayServicesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableGatewayServices,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&ServiceNameIndex{
							Field: "Gateway",
						},
						&ServiceNameIndex{
							Field: "Service",
						},
						&memdb.IntFieldIndex{
							Field: "Port",
						},
					},
				},
			},
			indexGateway: {
				Name:         indexGateway,
				AllowMissing: false,
				Unique:       false,
				Indexer: &ServiceNameIndex{
					Field: "Gateway",
				},
			},
			indexService: {
				Name:         indexService,
				AllowMissing: true,
				Unique:       false,
				Indexer: &ServiceNameIndex{
					Field: "Service",
				},
			},
		},
	}
}

// meshTopologyTableSchema returns a new table schema used to store information
// relating upstream and downstream services
func meshTopologyTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableMeshTopology,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&ServiceNameIndex{
							Field: "Upstream",
						},
						&ServiceNameIndex{
							Field: "Downstream",
						},
					},
				},
			},
			indexUpstream: {
				Name:         indexUpstream,
				AllowMissing: true,
				Unique:       false,
				Indexer: &ServiceNameIndex{
					Field: "Upstream",
				},
			},
			indexDownstream: {
				Name:         indexDownstream,
				AllowMissing: false,
				Unique:       false,
				Indexer: &ServiceNameIndex{
					Field: "Downstream",
				},
			},
		},
	}
}

type ServiceNameIndex struct {
	Field string
}

func (index *ServiceNameIndex) FromObject(obj interface{}) (bool, []byte, error) {
	v := reflect.ValueOf(obj)
	v = reflect.Indirect(v) // Dereference the pointer if any

	fv := v.FieldByName(index.Field)
	isPtr := fv.Kind() == reflect.Ptr
	fv = reflect.Indirect(fv)
	if !isPtr && !fv.IsValid() || !fv.CanInterface() {
		return false, nil,
			fmt.Errorf("field '%s' for %#v is invalid %v ", index.Field, obj, isPtr)
	}

	name, ok := fv.Interface().(structs.ServiceName)
	if !ok {
		return false, nil, fmt.Errorf("Field 'ServiceName' is not of type structs.ServiceName")
	}

	// Enforce lowercase and add null character as terminator
	id := strings.ToLower(name.String()) + "\x00"

	return true, []byte(id), nil
}

func (index *ServiceNameIndex) FromArgs(args ...interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("must provide only a single argument")
	}
	name, ok := args[0].(structs.ServiceName)
	if !ok {
		return nil, fmt.Errorf("argument must be of type structs.ServiceName: %#v", args[0])
	}

	// Enforce lowercase and add null character as terminator
	id := strings.ToLower(name.String()) + "\x00"

	return []byte(strings.ToLower(id)), nil
}

func (index *ServiceNameIndex) PrefixFromArgs(args ...interface{}) ([]byte, error) {
	val, err := index.FromArgs(args...)
	if err != nil {
		return nil, err
	}

	// Strip the null terminator, the rest is a prefix
	n := len(val)
	if n > 0 {
		return val[:n-1], nil
	}
	return val, nil
}

// upstreamDownstream pairs come from individual proxy registrations, which can be updated independently.
type upstreamDownstream struct {
	Upstream   structs.ServiceName
	Downstream structs.ServiceName

	// Refs stores the registrations that contain this pairing.
	// When there are no remaining Refs, the upstreamDownstream can be deleted.
	//
	// Note: This map must be treated as immutable when accessed in MemDB.
	//       The entire upstreamDownstream structure must be deep copied on updates.
	Refs map[string]struct{}

	structs.RaftIndex
}

// NodeCheckQuery is used to query the ID index of the checks table.
type NodeCheckQuery struct {
	Node     string
	CheckID  string
	PeerName string
	acl.EnterpriseMeta
}

type peerIndexable interface {
	PeerOrEmpty() string
}

func (q NodeCheckQuery) PeerOrEmpty() string {
	return q.PeerName
}

// NamespaceOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q NodeCheckQuery) NamespaceOrDefault() string {
	return q.EnterpriseMeta.NamespaceOrDefault()
}

// PartitionOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q NodeCheckQuery) PartitionOrDefault() string {
	return q.EnterpriseMeta.PartitionOrDefault()
}

// ServiceVirtualIP is used to store a virtual IP associated with a service.
// It is also used to store assigned virtual IPs when a snapshot is created.
type ServiceVirtualIP struct {
	Service structs.PeeredServiceName
	IP      net.IP

	structs.RaftIndex
}

// FreeVirtualIP is used to store a virtual IP freed up by a service deregistration.
// It is also used to store free virtual IPs when a snapshot is created.
type FreeVirtualIP struct {
	IP        net.IP
	IsCounter bool
}

func counterIndex(obj interface{}) (bool, error) {
	if vip, ok := obj.(FreeVirtualIP); ok {
		return vip.IsCounter, nil
	}
	return false, fmt.Errorf("object is not a virtual IP entry")
}

func serviceVirtualIPTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableServiceVirtualIPs,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[structs.PeeredServiceName, ServiceVirtualIP, Query]{
					readIndex:  indexFromPeeredServiceName,
					writeIndex: indexFromServiceVirtualIP,
					// Read all peers in a cluster / partition
					prefixIndex: prefixIndexFromQueryWithPeerWildcardable,
				},
			},
		},
	}
}

func indexFromServiceVirtualIP(vip ServiceVirtualIP) ([]byte, error) {
	if vip.Service.ServiceName.Name == "" {
		return nil, errMissingValueForIndex
	}
	return indexFromPeeredServiceName(vip.Service)
}

func freeVirtualIPTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableFreeVirtualIPs,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "IP",
						},
						&memdb.ConditionalIndex{
							Conditional: counterIndex,
						},
					},
				},
			},
			indexCounterOnly: {
				Name:         indexCounterOnly,
				AllowMissing: false,
				Unique:       false,
				Indexer: &memdb.ConditionalIndex{
					Conditional: counterIndex,
				},
			},
		},
	}
}

type KindServiceName struct {
	Kind    structs.ServiceKind
	Service structs.ServiceName

	structs.RaftIndex
}

func (n *KindServiceName) PartitionOrDefault() string {
	return n.Service.PartitionOrDefault()
}

func (n *KindServiceName) NamespaceOrDefault() string {
	return n.Service.NamespaceOrDefault()
}

func kindServiceNameTableSchema() *memdb.TableSchema {
	// TODO(peering): make this peer-aware
	return &memdb.TableSchema{
		Name: tableKindServiceNames,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingle[any, any]{
					readIndex:  indexFromKindServiceName,
					writeIndex: indexFromKindServiceName,
				},
			},
			indexKind: {
				Name:         indexKind,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle[enterpriseIndexable, enterpriseIndexable]{
					readIndex:  indexFromKindServiceNameKindOnly,
					writeIndex: indexFromKindServiceNameKindOnly,
				},
			},
		},
	}
}

// KindServiceNameQuery is used to lookup service names by kind or enterprise meta.
type KindServiceNameQuery struct {
	Kind structs.ServiceKind
	Name string
	acl.EnterpriseMeta
}

// NamespaceOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q KindServiceNameQuery) NamespaceOrDefault() string {
	return q.EnterpriseMeta.NamespaceOrDefault()
}

// PartitionOrDefault exists because structs.EnterpriseMeta uses a pointer
// receiver for this method. Remove once that is fixed.
func (q KindServiceNameQuery) PartitionOrDefault() string {
	return q.EnterpriseMeta.PartitionOrDefault()
}

func indexFromKindServiceNameKindOnly(raw enterpriseIndexable) ([]byte, error) {
	switch x := raw.(type) {
	case *KindServiceName:
		var b indexBuilder
		b.String(strings.ToLower(string(x.Kind)))
		return b.Bytes(), nil

	case Query:
		var b indexBuilder
		b.String(strings.ToLower(x.Value))
		return b.Bytes(), nil

	default:
		return nil, fmt.Errorf("type must be *KindServiceName or Query: %T", raw)
	}
}

func kindServiceNamesMaxIndex(tx ReadTxn, ws memdb.WatchSet, kind string) uint64 {
	return maxIndexWatchTxn(tx, ws, kindServiceNameIndexName(kind))
}

func kindServiceNameIndexName(kind string) string {
	return "kind_service_names." + kind
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"fmt"
	"net"

	"github.com/hashicorp/consul/agent/config"
)

var (
	ErrECSNotGlobal       = fmt.Errorf("ECS response is not global")
	ErrNoData             = fmt.Errorf("no data")
	ErrNotFound           = fmt.Errorf("not found")
	ErrNotSupported       = fmt.Errorf("not supported")
	ErrNoPathToDatacenter = fmt.Errorf("no path to datacenter")
)

// ECSNotGlobalError may be used to wrap an error or nil, to indicate that the
// EDNS client subnet source scope is not global.
type ECSNotGlobalError struct {
	error
}

func (e ECSNotGlobalError) Error() string {
	if e.error == nil {
		return ""
	}
	return e.error.Error()
}

func (e ECSNotGlobalError) Is(other error) bool {
	return other == ErrECSNotGlobal
}

func (e ECSNotGlobalError) Unwrap() error {
	return e.error
}

// Query is used to request a name-based Service Discovery lookup.
type Query struct {
	QueryType    QueryType
	QueryPayload QueryPayload
}

// QueryType is used to filter service endpoints.
// This is needed by the V1 catalog because of the
// overlapping lookups through the service endpoint.
type QueryType string

const (
	QueryTypeConnect       QueryType = "CONNECT" // deprecated: use for V1 only
	QueryTypeIngress       QueryType = "INGRESS" // deprecated: use for V1 only
	QueryTypeInvalid       QueryType = "INVALID"
	QueryTypeNode          QueryType = "NODE"
	QueryTypePreparedQuery QueryType = "PREPARED_QUERY" // deprecated: use for V1 only
	QueryTypeService       QueryType = "SERVICE"
	QueryTypeVirtual       QueryType = "VIRTUAL"
	QueryTypeWorkload      QueryType = "WORKLOAD" // V2-only
)

// Context is used to pass information about the request.
type Context struct {
	Token string
}

// QueryTenancy is used to filter catalog data based on tenancy.
type QueryTenancy struct {
	Namespace     string
	Partition     string
	SamenessGroup string
	Peer          string
	Datacenter    string
}

// QueryPayload represents all information needed by the data backend
// to decide which records to include.
type QueryPayload struct {
	Name     string
	PortName string       // v1 - this could optionally be "connect" or "ingress"; v2 - this is the service port name
	Tag      string       // deprecated: use for V1 only
	SourceIP net.IP       // deprecated: used for prepared queries
	Tenancy  QueryTenancy // tenancy includes any additional labels specified before the domain
	Limit    int          // The maximum number of records to return

	// v2 fields only
	EnableFailover bool
}

// ResultType indicates the Consul resource that a discovery record represents.
// This is useful for things like adding TTLs for different objects in the DNS.
type ResultType string

const (
	ResultTypeService  ResultType = "SERVICE"
	ResultTypeNode     ResultType = "NODE"
	ResultTypeVirtual  ResultType = "VIRTUAL"
	ResultTypeWorkload ResultType = "WORKLOAD"
)

// Result is a generic format of targets that could be returned in a query.
// It is the responsibility of the DNS encoder to know what to do with
// each Result, based on the query type.
type Result struct {
	Service  *Location         // The name and address of the service.
	Node     *Location         // The name and address of the node.
	Metadata map[string]string // Used to collect metadata into TXT Records
	Type     ResultType        // Used to reconstruct the fqdn name of the resource
	DNS      DNSConfig         // Used for DNS-specific configuration for this result

	// Ports include anything the node/service/workload implements. These are filtered if requested by the client.
	// They are used in to generate the FQDN and SRV port numbers in V2 Catalog responses.
	Ports []Port

	Tenancy ResultTenancy
}

// TaggedAddress is used to represent a tagged address.
type TaggedAddress struct {
	Name    string
	Address string
	Port    Port
}

// Location is used to represent a service, node, or workload.
type Location struct {
	Name            string
	Address         string
	TaggedAddresses map[string]*TaggedAddress // Used to collect tagged addresses into A/AAAA Records
}

type DNSConfig struct {
	TTL    *uint32 // deprecated: use for V1 prepared queries only
	Weight uint32  // SRV queries
}

type Port struct {
	Name   string
	Number uint32
}

// ResultTenancy is used to reconstruct the fqdn name of the resource.
type ResultTenancy struct {
	Namespace  string
	Partition  string
	PeerName   string
	Datacenter string
}

// LookupType is used by the CatalogDataFetcher to properly filter endpoints.
type LookupType string

const (
	LookupTypeService LookupType = "SERVICE"
	LookupTypeConnect LookupType = "CONNECT"
	LookupTypeIngress LookupType = "INGRESS"
)

// CatalogDataFetcher is an interface that abstracts data collection
// for Discovery queries. It is assumed that the instantiation also
// includes any agent configuration that influences catalog queries.
//
//go:generate mockery --name CatalogDataFetcher --inpackage
type CatalogDataFetcher interface {
	// LoadConfig is used to hot-reload the data fetcher with new agent config.
	LoadConfig(config *config.RuntimeConfig)

	// FetchNodes fetches A/AAAA/CNAME
	FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error)

	// FetchEndpoints fetches records for A/AAAA/CNAME or SRV requests for services
	FetchEndpoints(ctx Context, req *QueryPayload, lookupType LookupType) ([]*Result, error)

	// FetchVirtualIP fetches A/AAAA records for virtual IPs
	FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error)

	// FetchRecordsByIp is used for PTR requests
	// to look up a service/node from an IP.
	FetchRecordsByIp(ctx Context, ip net.IP) ([]*Result, error)

	// FetchWorkload fetches a single Result associated with
	// V2 Workload. V2-only.
	FetchWorkload(ctx Context, req *QueryPayload) (*Result, error)

	// FetchPreparedQuery evaluates the results of a prepared query.
	// deprecated in V2
	FetchPreparedQuery(ctx Context, req *QueryPayload) ([]*Result, error)

	// NormalizeRequest mutates the original request based on data fetcher configuration, like
	// defaulting tenancy to the agent's partition.
	NormalizeRequest(req *QueryPayload)

	// ValidateRequest throws an error is any of the input fields are invalid for this data fetcher.
	ValidateRequest(ctx Context, req *QueryPayload) error
}

// QueryProcessor is used to process a Discovery Query and return the results.
type QueryProcessor struct {
	dataFetcher CatalogDataFetcher
}

// NewQueryProcessor creates a new QueryProcessor.
func NewQueryProcessor(dataFetcher CatalogDataFetcher) *QueryProcessor {
	return &QueryProcessor{
		dataFetcher: dataFetcher,
	}
}

// QueryByName is used to look up a service, node, workload, or prepared query.
func (p *QueryProcessor) QueryByName(query *Query, ctx Context) ([]*Result, error) {
	if err := p.dataFetcher.ValidateRequest(ctx, &query.QueryPayload); err != nil {
		return nil, err
	}

	p.dataFetcher.NormalizeRequest(&query.QueryPayload)

	switch query.QueryType {
	case QueryTypeNode:
		return p.dataFetcher.FetchNodes(ctx, &query.QueryPayload)
	case QueryTypeService:
		return p.dataFetcher.FetchEndpoints(ctx, &query.QueryPayload, LookupTypeService)
	case QueryTypeConnect:
		return p.dataFetcher.FetchEndpoints(ctx, &query.QueryPayload, LookupTypeConnect)
	case QueryTypeIngress:
		return p.dataFetcher.FetchEndpoints(ctx, &query.QueryPayload, LookupTypeIngress)
	case QueryTypeVirtual:
		result, err := p.dataFetcher.FetchVirtualIP(ctx, &query.QueryPayload)
		if err != nil {
			return nil, err
		}
		return []*Result{result}, nil
	case QueryTypeWorkload:
		result, err := p.dataFetcher.FetchWorkload(ctx, &query.QueryPayload)
		if err != nil {
			return nil, err
		}
		return []*Result{result}, nil
	case QueryTypePreparedQuery:
		return p.dataFetcher.FetchPreparedQuery(ctx, &query.QueryPayload)
	default:
		return nil, fmt.Errorf("unknown query type: %s", query.QueryType)
	}
}

// QueryByIP is used to look up a service or node from an IP address.
func (p *QueryProcessor) QueryByIP(ip net.IP, reqCtx Context) ([]*Result, error) {
	return p.dataFetcher.FetchRecordsByIp(reqCtx, ip)
}

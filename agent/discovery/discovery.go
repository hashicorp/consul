// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"fmt"
	"net"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
)

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
	QueryTypeNode          QueryType = "NODE"
	QueryTypePreparedQuery QueryType = "PREPARED_QUERY" // deprecated: use for V1 only
	QueryTypeService       QueryType = "SERVICE"
	QueryTypeVirtual       QueryType = "VIRTUAL"
	QueryTypeWorkload      QueryType = "WORKLOAD" // V2-only
)

type Context struct {
	Token            string
	DefaultPartition string
	DefaultNamespace string
	DefaultLocality  *structs.Locality
}

type QueryTenancy struct {
	Partition     string
	Namespace     string
	SamenessGroup string
	Peer          string
	Datacenter    string
}

// QueryPayload represents all information needed by the data backend
// to decide which records to include.
type QueryPayload struct {
	Name       string
	PortName   string       // v1 - this could optionally be "connect" or "ingress"; v2 - this is the service port name
	Tag        string       // deprecated: use for V1 only
	RemoteAddr net.Addr     // deprecated: used for prepared queries
	Tenancy    QueryTenancy // tenancy includes any additional labels specified before the domain

	// v2 fields only
	DisableFailover bool
}

// Result is a generic format of targets that could be returned in a query.
// It is the responsibility of the DNS encoder to know what to do with
// each Result, based on the query type.
type Result struct {
	Address  string // A/AAAA/CNAME records - could be used in the Extra section. CNAME is required to handle hostname addresses in workloads & nodes.
	Weight   uint32 // SRV queries
	Port     uint32 // SRV queries
	TTL      uint32
	Metadata []string // Used to collect metadata into TXT Records

	// Used in SRV & PTR queries to point at an A/AAAA Record.
	// In V1, this could be a full-qualified Service or Node name.
	// In V2, this is generally a fully-qualified Workload name.
	Target string
}

type LookupType string

const (
	LookupTypeService LookupType = "SERVICE"
	LookupTypeConnect LookupType = "CONNECT"
	LookupTypeIngress LookupType = "INGRESS"
)

// CatalogDataFetcher is an interface that abstracts data collection
// for Discovery queries. It is assumed that the instantiation also
// includes any agent configuration that influences catalog queries.
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
}

type QueryProcessor struct {
	dataFetcher CatalogDataFetcher
}

func NewQueryProcessor(dataFetcher CatalogDataFetcher) *QueryProcessor {
	return &QueryProcessor{
		dataFetcher: dataFetcher,
	}
}

func (p *QueryProcessor) QueryByName(query *Query, ctx Context) ([]*Result, error) {
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

func (p *QueryProcessor) QueryByIP(ip net.IP, ctx Context) ([]*Result, error) {
	return p.dataFetcher.FetchRecordsByIp(ctx, ip)
}

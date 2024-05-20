// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync/atomic"

	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// V2DataFetcherDynamicConfig is used to store the dynamic configuration of the V2 data fetcher.
type V2DataFetcherDynamicConfig struct {
	OnlyPassing bool
}

// V2DataFetcher is used to fetch data from the V2 catalog.
type V2DataFetcher struct {
	client pbresource.ResourceServiceClient
	logger hclog.Logger

	// Requests inherit the partition of the agent unless otherwise specified.
	defaultPartition string

	dynamicConfig atomic.Value
}

// NewV2DataFetcher creates a new V2 data fetcher.
func NewV2DataFetcher(config *config.RuntimeConfig, client pbresource.ResourceServiceClient, logger hclog.Logger) *V2DataFetcher {
	f := &V2DataFetcher{
		client:           client,
		logger:           logger,
		defaultPartition: config.PartitionOrDefault(),
	}
	f.LoadConfig(config)
	return f
}

// LoadConfig loads the configuration for the V2 data fetcher.
func (f *V2DataFetcher) LoadConfig(config *config.RuntimeConfig) {
	dynamicConfig := &V2DataFetcherDynamicConfig{
		OnlyPassing: config.DNSOnlyPassing,
	}
	f.dynamicConfig.Store(dynamicConfig)
}

// GetConfig loads the configuration for the V2 data fetcher.
func (f *V2DataFetcher) GetConfig() *V2DataFetcherDynamicConfig {
	return f.dynamicConfig.Load().(*V2DataFetcherDynamicConfig)
}

// FetchNodes fetches A/AAAA/CNAME
func (f *V2DataFetcher) FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error) {
	// TODO (v2-dns): NET-6623 - Implement FetchNodes
	// Make sure that we validate that namespace is not provided here
	return nil, fmt.Errorf("not implemented")
}

// FetchEndpoints fetches records for A/AAAA/CNAME or SRV requests for services
func (f *V2DataFetcher) FetchEndpoints(reqContext Context, req *QueryPayload, lookupType LookupType) ([]*Result, error) {
	if lookupType != LookupTypeService {
		return nil, ErrNotSupported
	}

	configCtx := f.dynamicConfig.Load().(*V2DataFetcherDynamicConfig)

	serviceEndpoints := pbcatalog.ServiceEndpoints{}
	serviceEndpointsResource, err := f.fetchResource(reqContext, *req, pbcatalog.ServiceEndpointsType, &serviceEndpoints)
	if err != nil {
		return nil, err
	}

	f.logger.Trace("shuffling endpoints", "name", req.Name, "endpoints", len(serviceEndpoints.Endpoints))

	// Shuffle the endpoints slice
	shuffleFunc := func(i, j int) {
		serviceEndpoints.Endpoints[i], serviceEndpoints.Endpoints[j] = serviceEndpoints.Endpoints[j], serviceEndpoints.Endpoints[i]
	}
	rand.Shuffle(len(serviceEndpoints.Endpoints), shuffleFunc)

	// Convert the service endpoints to results up to the limit
	limit := req.Limit
	if len(serviceEndpoints.Endpoints) < limit || limit == 0 {
		limit = len(serviceEndpoints.Endpoints)
	}

	results := make([]*Result, 0, limit)
	for _, endpoint := range serviceEndpoints.Endpoints[:limit] {

		// First we check the endpoint first to make sure that the requested port is matched from the service.
		// We error here because we expect all endpoints to have the same ports as the service.
		ports := getResultPorts(req, endpoint.Ports) //assuming the logic changed in getResultPorts
		if len(ports) == 0 {
			f.logger.Debug("could not find matching port in endpoint", "name", req.Name, "port", req.PortName)
			return nil, ErrNotFound
		}

		address, err := f.addressFromWorkloadAddresses(endpoint.Addresses, req.Name)
		if err != nil {
			return nil, err
		}

		weight, ok := getEndpointWeight(endpoint, configCtx)
		if !ok {
			f.logger.Debug("endpoint filtered out because of health status", "name", req.Name, "endpoint", endpoint.GetTargetRef().GetName())
			continue
		}

		result := &Result{
			Node: &Location{
				Address: address,
				Name:    endpoint.GetTargetRef().GetName(),
			},
			Type: ResultTypeWorkload,
			Tenancy: ResultTenancy{
				Namespace: serviceEndpointsResource.GetId().GetTenancy().GetNamespace(),
				Partition: serviceEndpointsResource.GetId().GetTenancy().GetPartition(),
			},
			DNS: DNSConfig{
				Weight: weight,
			},
			Ports: ports,
		}
		results = append(results, result)
	}
	return results, nil
}

// FetchVirtualIP fetches A/AAAA records for virtual IPs
func (f *V2DataFetcher) FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error) {
	// TODO (v2-dns): NET-6624 - Implement FetchVirtualIP
	return nil, fmt.Errorf("not implemented")
}

// FetchRecordsByIp is used for PTR requests to look up a service/node from an IP.
func (f *V2DataFetcher) FetchRecordsByIp(ctx Context, ip net.IP) ([]*Result, error) {
	// TODO (v2-dns): NET-6795 - Implement FetchRecordsByIp
	// Validate non-nil IP
	return nil, fmt.Errorf("not implemented")
}

// FetchWorkload is used to fetch a single workload from the V2 catalog.
// V2-only.
func (f *V2DataFetcher) FetchWorkload(reqContext Context, req *QueryPayload) (*Result, error) {
	workload := pbcatalog.Workload{}
	resourceObj, err := f.fetchResource(reqContext, *req, pbcatalog.WorkloadType, &workload)
	if err != nil {
		return nil, err
	}

	// First we check the endpoint first to make sure that the requested port is matched from the service.
	// We error here because we expect all endpoints to have the same ports as the service.
	ports := getResultPorts(req, workload.Ports) //assuming the logic changed in getResultPorts
	if ports == nil || len(ports) == 0 {
		f.logger.Debug("could not find matching port in endpoint", "name", req.Name, "port", req.PortName)
		return nil, ErrNotFound
	}

	address, err := f.addressFromWorkloadAddresses(workload.Addresses, req.Name)
	if err != nil {
		return nil, err
	}

	tenancy := resourceObj.GetId().GetTenancy()
	result := &Result{
		Node: &Location{
			Address: address,
			Name:    resourceObj.GetId().GetName(),
		},
		Type: ResultTypeWorkload,
		Tenancy: ResultTenancy{
			Namespace: tenancy.GetNamespace(),
			Partition: tenancy.GetPartition(),
		},
		Ports: ports,
	}

	return result, nil
}

// FetchPreparedQuery is used to fetch a prepared query from the V2 catalog.
// Deprecated in V2.
func (f *V2DataFetcher) FetchPreparedQuery(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, ErrNotSupported
}

func (f *V2DataFetcher) NormalizeRequest(req *QueryPayload) {
	// If we do not have an explicit partition in the request, we use the agent's
	if req.Tenancy.Partition == "" {
		req.Tenancy.Partition = f.defaultPartition
	}
}

// ValidateRequest throws an error is any of the deprecated V1 input fields are used in a QueryByName for this data fetcher.
func (f *V2DataFetcher) ValidateRequest(_ Context, req *QueryPayload) error {
	if req.Tag != "" {
		return ErrNotSupported
	}
	if req.SourceIP != nil {
		return ErrNotSupported
	}
	return nil
}

// fetchResource is used to read a single resource from the V2 catalog and cast into a concrete type.
func (f *V2DataFetcher) fetchResource(reqContext Context, req QueryPayload, kind *pbresource.Type, payload proto.Message) (*pbresource.Resource, error) {
	// Query the resource service for the ServiceEndpoints by name and tenancy
	resourceReq := pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name:    req.Name,
			Type:    kind,
			Tenancy: queryTenancyToResourceTenancy(req.Tenancy),
		},
	}

	f.logger.Trace("fetching "+kind.String(), "name", req.Name)
	resourceCtx := metadata.AppendToOutgoingContext(context.Background(), "x-consul-token", reqContext.Token)

	// If the service is not found, return nil and an error equivalent to NXDOMAIN
	response, err := f.client.Read(resourceCtx, &resourceReq)
	switch {
	case grpcNotFoundErr(err):
		f.logger.Debug(kind.String()+" not found", "name", req.Name)
		return nil, ErrNotFound
	case err != nil:
		f.logger.Error("error fetching "+kind.String(), "name", req.Name)
		return nil, fmt.Errorf("error fetching %s: %w", kind.String(), err)
		// default: fallthrough
	}

	data := response.GetResource().GetData()
	if err := data.UnmarshalTo(payload); err != nil {
		f.logger.Error("error unmarshalling "+kind.String(), "name", req.Name)
		return nil, fmt.Errorf("error unmarshalling %s: %w", kind.String(), err)
	}
	return response.GetResource(), nil
}

// addressFromWorkloadAddresses returns one address from the workload addresses.
func (f *V2DataFetcher) addressFromWorkloadAddresses(addresses []*pbcatalog.WorkloadAddress, name string) (string, error) {
	// TODO: (v2-dns): we will need to intelligently return the right workload address based on either the translate
	// address setting or the locality of the requester. Workloads must have at least one.
	// We also need to make sure that we filter out unix sockets here.
	address := addresses[0].GetHost()
	if strings.HasPrefix(address, "unix://") {
		f.logger.Error("unix sockets are currently unsupported in workload results", "name", name)
		return "", ErrNotFound
	}
	return address, nil
}

// getEndpointWeight returns the weight of the endpoint and a boolean indicating if the endpoint should be included
// based on it's health status.
func getEndpointWeight(endpoint *pbcatalog.Endpoint, configCtx *V2DataFetcherDynamicConfig) (uint32, bool) {
	health := endpoint.GetHealthStatus().Enum()
	if health == nil {
		return 0, false
	}

	// Filter based on health status and agent config
	// This is also a good opportunity to see if SRV weights are set
	var weight uint32
	switch *health {
	case pbcatalog.Health_HEALTH_PASSING:
		weight = endpoint.GetDns().GetWeights().GetPassing()
	case pbcatalog.Health_HEALTH_CRITICAL:
		return 0, false // always filtered out
	case pbcatalog.Health_HEALTH_WARNING:
		if configCtx.OnlyPassing {
			return 0, false // filtered out
		}
		weight = endpoint.GetDns().GetWeights().GetWarning()
	default:
		// Everything else can be filtered out
		return 0, false
	}

	// Important! double-check the weight in the case DNS weights are not set
	if weight == 0 {
		weight = 1
	}
	return weight, true
}

// getResultPorts conditionally returns ports from a map based on a query. The results are sorted by name.
func getResultPorts(req *QueryPayload, workloadPorts map[string]*pbcatalog.WorkloadPort) []Port {
	if len(workloadPorts) == 0 {
		return nil
	}

	var ports []Port
	if req.PortName != "" {
		// Make sure the workload implements that port name.
		if _, ok := workloadPorts[req.PortName]; !ok {
			return nil
		}
		// In the case that the query asked for a specific port, we only return that port.
		ports = []Port{
			{
				Name:   req.PortName,
				Number: workloadPorts[req.PortName].Port,
			},
		}
	} else {
		// If the client didn't specify a particular port, return all the workload ports.
		for name, port := range workloadPorts {
			ports = append(ports, Port{
				Name:   name,
				Number: port.Port,
			})
		}
		// Stable Sort
		slices.SortStableFunc(ports, func(i, j Port) int {
			if i.Name < j.Name {
				return -1
			} else if i.Name > j.Name {
				return 1
			}
			return 0
		})
	}
	return ports
}

// queryTenancyToResourceTenancy converts a QueryTenancy to a pbresource.Tenancy.
func queryTenancyToResourceTenancy(qTenancy QueryTenancy) *pbresource.Tenancy {
	rTenancy := resource.DefaultNamespacedTenancy()

	// If the request has any tenancy specified, it overrides the defaults.
	if qTenancy.Namespace != "" {
		rTenancy.Namespace = qTenancy.Namespace
	}
	// In the case of partition, we have the agent's partition as the fallback.
	// That is handled in NormalizeRequest.
	if qTenancy.Partition != "" {
		rTenancy.Partition = qTenancy.Partition
	}

	return rTenancy
}

// grpcNotFoundErr returns true if the error is a gRPC status error with a code of NotFound.
func grpcNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	return ok && s.Code() == codes.NotFound
}

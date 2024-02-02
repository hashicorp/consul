// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// v2DataFetcherDynamicConfig is used to store the dynamic configuration of the V2 data fetcher.
type v2DataFetcherDynamicConfig struct {
	onlyPassing bool
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
	dynamicConfig := &v2DataFetcherDynamicConfig{
		onlyPassing: config.DNSOnlyPassing,
	}
	f.dynamicConfig.Store(dynamicConfig)
}

// FetchNodes fetches A/AAAA/CNAME
func (f *V2DataFetcher) FetchNodes(ctx Context, req *QueryPayload) ([]*Result, error) {
	return nil, nil
}

// FetchEndpoints fetches records for A/AAAA/CNAME or SRV requests for services
// TODO (v2-dns): Validate lookupType
func (f *V2DataFetcher) FetchEndpoints(ctx Context, req *QueryPayload, lookupType LookupType) ([]*Result, error) {
	return nil, nil
}

// FetchVirtualIP fetches A/AAAA records for virtual IPs
func (f *V2DataFetcher) FetchVirtualIP(ctx Context, req *QueryPayload) (*Result, error) {
	return nil, nil
}

// FetchRecordsByIp is used for PTR requests to look up a service/node from an IP.
// TODO (v2-dns): Validate non-nil IP
func (f *V2DataFetcher) FetchRecordsByIp(ctx Context, ip net.IP) ([]*Result, error) {
	return nil, nil
}

// FetchWorkload is used to fetch a single workload from the V2 catalog.
// V2-only.
func (f *V2DataFetcher) FetchWorkload(reqContext Context, req *QueryPayload) (*Result, error) {
	// Query the resource service for the workload by name and tenancy
	resourceReq := pbresource.ReadRequest{
		Id: &pbresource.ID{
			Name:    req.Name,
			Type:    pbcatalog.WorkloadType,
			Tenancy: queryTenancyToResourceTenancy(req.Tenancy),
		},
	}

	f.logger.Debug("fetching workload", "name", req.Name)
	resourceCtx := metadata.AppendToOutgoingContext(context.Background(), "x-consul-token", reqContext.Token)

	// If the workload is not found, return nil and an error equivalent to NXDOMAIN
	response, err := f.client.Read(resourceCtx, &resourceReq)
	switch {
	case grpcNotFoundErr(err):
		f.logger.Debug("workload not found", "name", req.Name)
		return nil, ErrNotFound
	case err != nil:
		f.logger.Error("error fetching workload", "name", req.Name)
		return nil, fmt.Errorf("error fetching workload: %w", err)
		// default: fallthrough
	}

	workload := &pbcatalog.Workload{}
	data := response.GetResource().GetData()
	if err := data.UnmarshalTo(workload); err != nil {
		f.logger.Error("error unmarshalling workload", "name", req.Name)
		return nil, fmt.Errorf("error unmarshalling workload: %w", err)
	}

	// TODO: (v2-dns): we will need to intelligently return the right workload address based on either the translate
	// address setting or the locality of the requester. Workloads must have at least one.
	// We also need to make sure that we filter out unix sockets here.
	address := workload.Addresses[0].GetHost()
	if strings.HasPrefix(address, "unix://") {
		f.logger.Error("unix sockets are currently unsupported in workload results", "name", req.Name)
		return nil, ErrNotFound
	}

	tenancy := response.GetResource().GetId().GetTenancy()
	result := &Result{
		Address: address,
		Type:    ResultTypeWorkload,
		Tenancy: ResultTenancy{
			Namespace: tenancy.GetNamespace(),
			Partition: tenancy.GetPartition(),
		},
		Target: response.GetResource().GetId().GetName(),
	}

	if req.PortName == "" {
		return result, nil
	}

	// If a port is specified, make sure the workload implements that port name.
	for name, port := range workload.Ports {
		if name == req.PortName {
			result.PortName = req.PortName
			result.PortNumber = port.Port
			return result, nil
		}
	}

	f.logger.Debug("could not find matching port for workload", "name", req.Name, "port", req.PortName)
	// Return an ErrNotFound, which is equivalent to NXDOMAIN
	return nil, ErrNotFound
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
	if req.RemoteAddr != nil {
		return ErrNotSupported
	}
	return nil
}

func queryTenancyToResourceTenancy(qTenancy QueryTenancy) *pbresource.Tenancy {
	rTenancy := resource.DefaultNamespacedTenancy()

	// If the request has any tenancy specified, it overrides the defaults.
	if qTenancy.Namespace != "" {
		rTenancy.Namespace = qTenancy.Namespace
	}
	// In the case of partition, we have the agent's partition as the fallback.
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

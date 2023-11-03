package endpoints

import (
	"context"
	"sort"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serviceData struct {
	resource *pbresource.Resource
	service  *pbcatalog.Service
}

type endpointsData struct {
	resource  *pbresource.Resource
	endpoints *pbcatalog.ServiceEndpoints
}

type workloadData struct {
	resource *pbresource.Resource
	workload *pbcatalog.Workload
}

// getServiceData will read the service with the given ID and unmarshal the
// Data field. The return value is a struct that contains the retrieved
// resource as well as the unmsashalled form. If the resource doesn't
// exist, nil will be returned. Any other error either with retrieving
// the resource or unmarshalling it will cause the error to be returned
// to the caller
func getServiceData(ctx context.Context, rt controller.Runtime, id *pbresource.ID) (*serviceData, error) {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: id})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	var service pbcatalog.Service
	err = rsp.Resource.Data.UnmarshalTo(&service)
	if err != nil {
		return nil, resource.NewErrDataParse(&service, err)
	}

	return &serviceData{resource: rsp.Resource, service: &service}, nil
}

// getEndpointsData will read the endpoints with the given ID and unmarshal the
// Data field. The return value is a struct that contains the retrieved
// resource as well as the unmsashalled form. If the resource doesn't
// exist, nil will be returned. Any other error either with retrieving
// the resource or unmarshalling it will cause the error to be returned
// to the caller
func getEndpointsData(ctx context.Context, rt controller.Runtime, id *pbresource.ID) (*endpointsData, error) {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: id})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	var endpoints pbcatalog.ServiceEndpoints
	err = rsp.Resource.Data.UnmarshalTo(&endpoints)
	if err != nil {
		return nil, resource.NewErrDataParse(&endpoints, err)
	}

	return &endpointsData{resource: rsp.Resource, endpoints: &endpoints}, nil
}

// getWorkloadData will retrieve all workloads for the given services selector
// and unmarhshal them, returning a slic of objects hold both the resource and
// unmarshaled forms. Unmarshalling errors, or other resource service errors
// will be returned to the caller.
func getWorkloadData(ctx context.Context, rt controller.Runtime, svc *serviceData) ([]*workloadData, error) {
	workloadResources, err := gatherWorkloadsForService(ctx, rt, svc)
	if err != nil {
		return nil, err
	}

	var results []*workloadData
	for _, res := range workloadResources {
		var workload pbcatalog.Workload
		err = res.Data.UnmarshalTo(&workload)
		if err != nil {
			return nil, resource.NewErrDataParse(&workload, err)
		}

		results = append(results, &workloadData{resource: res, workload: &workload})
	}

	return results, nil
}

// gatherWorkloadsForService will retrieve all the unique workloads for a given selector.
// NotFound errors for workloads selected by Name will be ignored. Any other
// resource service errors will be returned to the caller. Prior to returning
// the slice of resources, they will be sorted by name. The consistent ordering
// will allow callers to diff two versions of the data to determine if anything
// has changed but it also will make testing a little easier.
func gatherWorkloadsForService(ctx context.Context, rt controller.Runtime, svc *serviceData) ([]*pbresource.Resource, error) {
	var workloads []*pbresource.Resource

	sel := svc.service.GetWorkloads()

	// this map will track all the gathered workloads by name, this is mainly to deduplicate workloads if they
	// are specified multiple times throughout the list of selection criteria
	workloadNames := make(map[string]struct{})

	// First gather all the prefix matched workloads. We could do this second but by doing
	// it first its possible we can avoid some resource service calls to read individual
	// workloads selected by name if they are also matched by a prefix.
	for _, prefix := range sel.GetPrefixes() {
		rsp, err := rt.Client.List(ctx, &pbresource.ListRequest{
			Type:       types.WorkloadType,
			Tenancy:    svc.resource.Id.Tenancy,
			NamePrefix: prefix,
		})
		if err != nil {
			return nil, err
		}

		// append all workloads in the list response to our list of all selected workloads
		for _, workload := range rsp.Resources {
			// ignore duplicate workloads
			if _, found := workloadNames[workload.Id.Name]; !found {
				workloads = append(workloads, workload)
				workloadNames[workload.Id.Name] = struct{}{}
			}
		}
	}

	// Now gather the exact match selections
	for _, name := range sel.GetNames() {
		// ignore names we have already fetched
		if _, found := workloadNames[name]; found {
			continue
		}

		workloadID := &pbresource.ID{
			Type:    types.WorkloadType,
			Tenancy: svc.resource.Id.Tenancy,
			Name:    name,
		}

		rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: workloadID})
		switch {
		case status.Code(err) == codes.NotFound:
			// Ignore not found errors as services may select workloads that do not
			// yet exist. This is not considered an error state or mis-configuration
			// as the user could be getting ready to add the workloads.
			continue
		case err != nil:
			return nil, err
		}

		workloads = append(workloads, rsp.Resource)
		workloadNames[rsp.Resource.Id.Name] = struct{}{}
	}

	// Sorting ensures deterministic output. This will help for testing but
	// the real reason to do this is so we will be able to diff the set of
	// workloads endpoints to determine if we need to update them.
	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].Id.Name < workloads[j].Id.Name
	})

	return workloads, nil
}

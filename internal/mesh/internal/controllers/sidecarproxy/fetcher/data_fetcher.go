package fetcher

import (
	"context"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	ctrlStatus "github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	intermediateTypes "github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Fetcher struct {
	Client pbresource.ResourceServiceClient
	Cache  *sidecarproxycache.Cache
}

func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*intermediateTypes.Workload, error) {
	rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{Id: id})

	switch {
	case status.Code(err) == codes.NotFound:
		// We also need to make sure to delete the associated proxy from cache.
		// We are ignoring errors from cache here as this deletion is best effort.
		f.Cache.DeleteSourceProxy(resource.ReplaceType(types.ProxyStateTemplateType, id))
		return nil, nil
	case err != nil:
		return nil, err
	}

	w := &intermediateTypes.Workload{
		Resource: rsp.Resource,
	}

	var workload pbcatalog.Workload
	err = rsp.Resource.Data.UnmarshalTo(&workload)
	if err != nil {
		return nil, resource.NewErrDataParse(&workload, err)
	}

	w.Workload = &workload
	return w, nil
}

func (f *Fetcher) FetchProxyStateTemplate(ctx context.Context, id *pbresource.ID) (*intermediateTypes.ProxyStateTemplate, error) {
	rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{Id: id})

	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	p := &intermediateTypes.ProxyStateTemplate{
		Resource: rsp.Resource,
	}

	var tmpl pbmesh.ProxyStateTemplate
	err = rsp.Resource.Data.UnmarshalTo(&tmpl)
	if err != nil {
		return nil, resource.NewErrDataParse(&tmpl, err)
	}

	p.Tmpl = &tmpl
	return p, nil
}

func (f *Fetcher) FetchServiceEndpoints(ctx context.Context, id *pbresource.ID) (*intermediateTypes.ServiceEndpoints, error) {
	rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{Id: id})

	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	se := &intermediateTypes.ServiceEndpoints{
		Resource: rsp.Resource,
	}

	var endpoints pbcatalog.ServiceEndpoints
	err = rsp.Resource.Data.UnmarshalTo(&endpoints)
	if err != nil {
		return nil, resource.NewErrDataParse(&endpoints, err)
	}

	se.Endpoints = &endpoints
	return se, nil
}

func (f *Fetcher) FetchDestinations(ctx context.Context, id *pbresource.ID) (*intermediateTypes.Destinations, error) {
	rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{Id: id})

	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	u := &intermediateTypes.Destinations{
		Resource: rsp.Resource,
	}

	var destinations pbmesh.Upstreams
	err = rsp.Resource.Data.UnmarshalTo(&destinations)
	if err != nil {
		return nil, resource.NewErrDataParse(&destinations, err)
	}

	u.Destinations = &destinations
	return u, nil
}

func (f *Fetcher) FetchDestinationsData(
	ctx context.Context,
	destinationRefs []intermediateTypes.CombinedDestinationRef,
) ([]*intermediateTypes.Destination, map[string]*intermediateTypes.Status, error) {

	var destinations []*intermediateTypes.Destination
	statuses := make(map[string]*intermediateTypes.Status)
	for _, dest := range destinationRefs {
		// Fetch Destinations resource if there is one.
		us, err := f.FetchDestinations(ctx, dest.ExplicitDestinationsID)
		if err != nil {
			// If there's an error, return and force another reconcile instead of computing
			// partial proxy state.
			return nil, statuses, err
		}

		if us == nil {
			// If the Destinations resource is not found, then we should delete it from cache and continue.
			f.Cache.DeleteDestination(dest.ServiceRef, dest.Port)
			continue
		}

		d := &intermediateTypes.Destination{}
		// As Destinations resource contains a list of destinations,
		// we need to find the one that references our service and port.
		d.Explicit = findDestination(dest.ServiceRef, dest.Port, us.Destinations)

		// Fetch ServiceEndpoints.
		serviceID := resource.IDFromReference(dest.ServiceRef)
		se, err := f.FetchServiceEndpoints(ctx, resource.ReplaceType(catalog.ServiceEndpointsType, serviceID))
		if err != nil {
			return nil, statuses, err
		}

		serviceRef := resource.ReferenceToString(dest.ServiceRef)
		upstreamsRef := resource.IDToString(us.Resource.Id)
		if se == nil {
			// If the Service Endpoints resource is not found, then we update the status of the Upstreams resource
			// but don't remove it from cache in case it comes back.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionDestinationServiceNotFound(serviceRef))
			continue
		} else {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionDestinationServiceFound(serviceRef))
		}

		d.ServiceEndpoints = se

		// Check if this endpoints is mesh-enabled. If not, remove it from cache and return an error.
		if !IsMeshEnabled(se.Endpoints.Endpoints[0].Ports) {
			// Add invalid status but don't remove from cache. If this state changes,
			// we want to be able to detect this change.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionMeshProtocolNotFound(serviceRef))

			// This error should not cause the execution to stop, as we want to make sure that this non-mesh destination
			// gets removed from the proxy state.
			continue
		} else {
			// If everything was successful, add an empty condition so that we can remove any existing statuses.
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionMeshProtocolFound(serviceRef))
		}

		// No destination port should point to a port with "mesh" protocol,
		// so check if destination port has the mesh protocol and update the status.
		if se.Endpoints.Endpoints[0].Ports[dest.Port].Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionMeshProtocolDestinationPort(serviceRef, dest.Port))
			continue
		} else {
			updateStatusCondition(statuses, upstreamsRef, dest.ExplicitDestinationsID,
				us.Resource.Status, us.Resource.Generation, ctrlStatus.ConditionNonMeshProtocolDestinationPort(serviceRef, dest.Port))
		}

		// Gather all identities.
		if se != nil {
			var identities []*pbresource.Reference
			for _, ep := range se.Endpoints.Endpoints {
				identities = append(identities, &pbresource.Reference{
					Name:    ep.Identity,
					Tenancy: se.Resource.Id.Tenancy,
				})
			}
			d.Identities = identities
		}

		destinations = append(destinations, d)
	}

	return destinations, statuses, nil
}

// IsMeshEnabled returns true if the workload or service endpoints port
// contain a port with the "mesh" protocol.
func IsMeshEnabled(ports map[string]*pbcatalog.WorkloadPort) bool {
	for _, port := range ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			return true
		}
	}
	return false
}

func findDestination(ref *pbresource.Reference, port string, destinations *pbmesh.Upstreams) *pbmesh.Upstream {
	for _, destination := range destinations.Upstreams {
		if resource.EqualReference(ref, destination.DestinationRef) &&
			port == destination.DestinationPort {
			return destination
		}
	}
	return nil
}

func updateStatusCondition(
	statuses map[string]*intermediateTypes.Status,
	key string,
	id *pbresource.ID,
	oldStatus map[string]*pbresource.Status,
	generation string,
	condition *pbresource.Condition) {
	if _, ok := statuses[key]; ok {
		statuses[key].Conditions = append(statuses[key].Conditions, condition)
	} else {
		statuses[key] = &intermediateTypes.Status{
			ID:         id,
			Generation: generation,
			Conditions: []*pbresource.Condition{condition},
			OldStatus:  oldStatus,
		}
	}
}

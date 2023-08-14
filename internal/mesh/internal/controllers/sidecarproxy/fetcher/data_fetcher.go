package fetcher

import (
	"context"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	ctrlStatus "github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy/status"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	intermediateTypes "github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Fetcher struct {
	Client            pbresource.ResourceServiceClient
	DestinationsCache *sidecarproxycache.DestinationsCache
	ProxyCfgCache     *sidecarproxycache.ProxyConfigurationCache
}

func New(client pbresource.ResourceServiceClient,
	dCache *sidecarproxycache.DestinationsCache,
	pcfgCache *sidecarproxycache.ProxyConfigurationCache) *Fetcher {

	return &Fetcher{
		Client:            client,
		DestinationsCache: dCache,
		ProxyCfgCache:     pcfgCache,
	}
}

func (f *Fetcher) FetchWorkload(ctx context.Context, id *pbresource.ID) (*intermediateTypes.Workload, error) {
	rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{Id: id})

	switch {
	case status.Code(err) == codes.NotFound:
		// We also need to make sure to delete the associated proxy from cache.
		// We are ignoring errors from cache here as this deletion is best effort.
		proxyID := resource.ReplaceType(types.ProxyStateTemplateType, id)
		f.DestinationsCache.DeleteSourceProxy(proxyID)
		f.ProxyCfgCache.UntrackProxyID(proxyID)
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

func (f *Fetcher) FetchService(ctx context.Context, id *pbresource.ID) (*intermediateTypes.Service, error) {
	rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{Id: id})

	switch {
	case status.Code(err) == codes.NotFound:
		return nil, nil
	case err != nil:
		return nil, err
	}

	se := &intermediateTypes.Service{
		Resource: rsp.Resource,
	}

	var service pbcatalog.Service
	err = rsp.Resource.Data.UnmarshalTo(&service)
	if err != nil {
		return nil, resource.NewErrDataParse(&service, err)
	}

	se.Service = &service
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

func (f *Fetcher) FetchExplicitDestinationsData(
	ctx context.Context,
	explDestRefs []intermediateTypes.CombinedDestinationRef,
) ([]*intermediateTypes.Destination, map[string]*intermediateTypes.Status, error) {

	var destinations []*intermediateTypes.Destination
	statuses := make(map[string]*intermediateTypes.Status)
	for _, dest := range explDestRefs {
		// Fetch Destinations resource if there is one.
		us, err := f.FetchDestinations(ctx, dest.ExplicitDestinationsID)
		if err != nil {
			// If there's an error, return and force another reconcile instead of computing
			// partial proxy state.
			return nil, statuses, err
		}

		if us == nil {
			// If the Destinations resource is not found, then we should delete it from cache and continue.
			f.DestinationsCache.DeleteDestination(dest.ServiceRef, dest.Port)
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

// FetchImplicitDestinationsData fetches all implicit destinations and adds them to existing destinations.
// If the implicit destination is already in addToDestinations, it will be skipped.
// todo (ishustava): this function will eventually need to fetch implicit destinations from the ImplicitDestinations resource instead.
func (f *Fetcher) FetchImplicitDestinationsData(ctx context.Context, proxyID *pbresource.ID, addToDestinations []*intermediateTypes.Destination) ([]*intermediateTypes.Destination, error) {
	// First, convert existing destinations to a map so we can de-dup.
	destinations := make(map[resource.ReferenceKey]*intermediateTypes.Destination)
	for _, d := range addToDestinations {
		destinations[resource.NewReferenceKey(d.ServiceEndpoints.Resource.Id)] = d
	}

	// For now, we need to look up all service endpoints within a partition.
	rsp, err := f.Client.List(ctx, &pbresource.ListRequest{
		Type: catalog.ServiceEndpointsType,
		Tenancy: &pbresource.Tenancy{
			Namespace: storage.Wildcard,
			Partition: proxyID.Tenancy.Partition,
			PeerName:  proxyID.Tenancy.PeerName,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, r := range rsp.Resources {
		// If it's already in destinations, ignore it.
		if _, ok := destinations[resource.NewReferenceKey(r.Id)]; ok {
			continue
		}

		var endpoints pbcatalog.ServiceEndpoints
		err = r.Data.UnmarshalTo(&endpoints)
		if err != nil {
			return nil, err
		}

		// If this proxy is a part of this service, ignore it.
		if isPartOfService(resource.ReplaceType(catalog.WorkloadType, proxyID), &endpoints) {
			continue
		}

		// Collect all identities.
		var identities []*pbresource.Reference
		for _, ep := range endpoints.Endpoints {
			identities = append(identities, &pbresource.Reference{
				Name:    ep.Identity,
				Tenancy: r.Id.Tenancy,
			})
		}

		// Fetch the service.
		// todo (ishustava): this should eventually grab virtual IPs resource.
		s, err := f.FetchService(ctx, resource.ReplaceType(catalog.ServiceType, r.Id))
		if err != nil {
			return nil, err
		}
		if s == nil {
			// If service no longer exists, skip.
			continue
		}

		d := &intermediateTypes.Destination{
			ServiceEndpoints: &intermediateTypes.ServiceEndpoints{
				Resource:  r,
				Endpoints: &endpoints,
			},
			VirtualIPs: s.Service.VirtualIps,
			Identities: identities,
		}
		addToDestinations = append(addToDestinations, d)
	}
	return addToDestinations, err
}

// FetchAndMergeProxyConfigurations fetches proxy configurations for the proxy state template provided by id
// and merges them into one object.
func (f *Fetcher) FetchAndMergeProxyConfigurations(ctx context.Context, id *pbresource.ID) (*pbmesh.ProxyConfiguration, error) {
	proxyCfgRefs := f.ProxyCfgCache.ProxyConfigurationsByProxyID(id)

	result := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{},
	}
	for _, ref := range proxyCfgRefs {
		proxyCfgID := &pbresource.ID{
			Name:    ref.GetName(),
			Type:    ref.GetType(),
			Tenancy: ref.GetTenancy(),
		}
		rsp, err := f.Client.Read(ctx, &pbresource.ReadRequest{
			Id: proxyCfgID,
		})
		switch {
		case status.Code(err) == codes.NotFound:
			f.ProxyCfgCache.UntrackProxyConfiguration(proxyCfgID)
			return nil, nil
		case err != nil:
			return nil, err
		}

		var proxyCfg pbmesh.ProxyConfiguration
		err = rsp.Resource.Data.UnmarshalTo(&proxyCfg)
		if err != nil {
			return nil, err
		}

		// Note that we only care about dynamic config as bootstrap config
		// will not be updated dynamically by this controller.
		// todo (ishustava): do sorting etc.
		proto.Merge(result.DynamicConfig, proxyCfg.DynamicConfig)
	}

	return result, nil
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

func isPartOfService(workloadID *pbresource.ID, endpoints *pbcatalog.ServiceEndpoints) bool {
	// convert IDs to refs so that we can compare without UIDs.
	workloadRef := resource.Reference(workloadID, "")
	for _, ep := range endpoints.Endpoints {
		if ep.TargetRef != nil {
			targetRef := resource.Reference(ep.TargetRef, "")
			if resource.EqualReference(workloadRef, targetRef) {
				return true
			}
		}
	}
	return false
}

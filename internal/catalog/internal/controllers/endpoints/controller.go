// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"context"
	"sort"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/catalog/workloadselector"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	endpointsMetaManagedBy = "managed-by-controller"

	selectedWorkloadsIndexName = "selected-workloads"
)

type (
	DecodedWorkload         = resource.DecodedResource[*pbcatalog.Workload]
	DecodedService          = resource.DecodedResource[*pbcatalog.Service]
	DecodedServiceEndpoints = resource.DecodedResource[*pbcatalog.ServiceEndpoints]
)

// ServiceEndpointsController creates a controller to perform automatic endpoint management for
// services.
func ServiceEndpointsController() *controller.Controller {
	return controller.NewController(ControllerID, pbcatalog.ServiceEndpointsType).
		WithWatch(pbcatalog.ServiceType,
			// ServiceEndpoints are name-aligned with the Service type
			dependency.ReplaceType(pbcatalog.ServiceEndpointsType),
			// This cache index keeps track of the relationship between WorkloadSelectors (and the workload names and prefixes
			// they include) and Services. This allows us to efficiently find all services and service endpoints that are
			// are affected by the change to a workload.
			workloadselector.Index[*pbcatalog.Service](selectedWorkloadsIndexName)).
		WithWatch(pbcatalog.WorkloadType,
			// The cache index is kept on the Service type but we need to translate events for ServiceEndpoints.
			// Therefore we need to wrap the mapper from the workloadselector package with one which will
			// replace the request types of Service with ServiceEndpoints.
			dependency.WrapAndReplaceType(
				pbcatalog.ServiceEndpointsType,
				// This mapper will use the selected-workloads index to find all Services which select this
				// workload by exact name or by prefix.
				workloadselector.MapWorkloadsToSelectors(pbcatalog.ServiceType, selectedWorkloadsIndexName),
			),
		).
		WithReconciler(newServiceEndpointsReconciler())
}

type serviceEndpointsReconciler struct{}

func newServiceEndpointsReconciler() *serviceEndpointsReconciler {
	return &serviceEndpointsReconciler{}
}

// Reconcile will reconcile one ServiceEndpoints resource in response to some event.
func (r *serviceEndpointsReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// The runtime is passed by value so replacing it here for the remainder of this
	// reconciliation request processing will not affect future invocations.
	rt.Logger = rt.Logger.With("resource-id", req.ID)

	rt.Logger.Trace("reconciling service endpoints")

	endpointsID := req.ID
	serviceID := &pbresource.ID{
		Type:    pbcatalog.ServiceType,
		Tenancy: endpointsID.Tenancy,
		Name:    endpointsID.Name,
	}

	// First we read and unmarshal the service
	service, err := cache.GetDecoded[*pbcatalog.Service](rt.Cache, pbcatalog.ServiceType, "id", serviceID)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding Service", "error", err)
		return err
	}

	// Check if the service exists. If it doesn't we can avoid a bunch of other work.
	if service == nil {
		rt.Logger.Trace("service has been deleted")

		// Note that because we configured ServiceEndpoints to be owned by the service,
		// the service endpoints object should eventually be automatically deleted.
		// There is no reason to attempt deletion here.
		return nil
	}

	// Now read and unmarshal the endpoints. We don't need this data just yet but all
	// code paths from this point on will need this regardless of branching so we pull
	// it now.
	endpoints, err := cache.GetDecoded[*pbcatalog.ServiceEndpoints](rt.Cache, pbcatalog.ServiceEndpointsType, "id", endpointsID)
	if err != nil {
		rt.Logger.Error("error retrieving existing endpoints", "error", err)
		return err
	}

	var statusConditions []*pbresource.Condition

	if serviceUnderManagement(service.Data) {
		rt.Logger.Trace("service is enabled for automatic endpoint management")
		// This service should have its endpoints automatically managed
		statusConditions = append(statusConditions, ConditionManaged)

		// Now read and unmarshal all workloads selected by the service.
		workloads, err := workloadselector.GetWorkloadsWithSelector(rt.Cache, service)
		if err != nil {
			rt.Logger.Trace("error retrieving selected workloads", "error", err)
			return err
		}

		// Calculate the latest endpoints from the already gathered workloads
		latestEndpoints := workloadsToEndpoints(service.Data, workloads)

		// Add status
		if endpoints != nil {
			statusConditions = append(statusConditions,
				workloadIdentityStatusFromEndpoints(latestEndpoints))
		}

		// Before writing the endpoints actually check to see if they are changed
		if endpoints == nil || !proto.Equal(endpoints.Data, latestEndpoints) {
			rt.Logger.Trace("endpoints have changed")

			// First encode the endpoints data as an Any type.
			endpointData, err := anypb.New(latestEndpoints)
			if err != nil {
				rt.Logger.Error("error marshalling latest endpoints", "error", err)
				return err
			}

			// Now perform the write. The endpoints resource should be owned by the service
			// so that it will automatically be deleted upon service deletion. We are using
			// a special metadata entry to track that this controller is responsible for
			// the management of this resource.
			_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
				Resource: &pbresource.Resource{
					Id:    req.ID,
					Owner: service.Id,
					Metadata: map[string]string{
						endpointsMetaManagedBy: ControllerID,
					},
					Data: endpointData,
				},
			})
			if err != nil {
				rt.Logger.Error("error writing generated endpoints", "error", err)
				return err
			} else {
				rt.Logger.Trace("updated endpoints were successfully written")
			}
		}
	} else {
		rt.Logger.Trace("endpoints are not being automatically managed")
		// This service is not having its endpoints automatically managed
		statusConditions = append(statusConditions, ConditionUnmanaged)

		// Delete the managed ServiceEndpoints if necessary if the metadata would
		// indicate that they were previously managed by this controller
		if endpoints != nil && endpoints.Metadata[endpointsMetaManagedBy] == ControllerID {
			rt.Logger.Trace("removing previous managed endpoints")

			// This performs a CAS deletion to protect against the case where the user
			// has overwritten the endpoints since we fetched them.
			_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{
				Id:      endpoints.Id,
				Version: endpoints.Version,
			})

			// Potentially we could look for CAS failures by checking if the gRPC
			// status code is Aborted. However its an edge case and there could
			// possibly be other reasons why the gRPC status code would be aborted
			// besides CAS version mismatches. The simplest thing to do is to just
			// propagate the error and retry reconciliation later.
			if err != nil {
				rt.Logger.Error("error deleting previously managed endpoints", "error", err)
				return err
			}
		}
	}

	// Update the Service status if necessary. Mainly we want to inform the user
	// whether we are automatically managing the endpoints to set expectations
	// for that object existing or not.
	newStatus := &pbresource.Status{
		ObservedGeneration: service.Generation,
		Conditions:         statusConditions,
	}
	// If the status is unchanged then we should return and avoid the unnecessary write
	if resource.EqualStatus(service.Status[ControllerID], newStatus, false) {
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     service.Id,
		Key:    ControllerID,
		Status: newStatus,
	})

	if err != nil {
		rt.Logger.Error("error updating the service's status", "error", err, "service", serviceID)
	}
	return err
}

// determineWorkloadHealth will find the workload-health controller's status
// within the resource status and parse the workloads health out of it. If
// the workload-health controller has yet to reconcile the workload health
// or the status isn't in the expected form then this function will return
// HEALTH_CRITICAL.
func determineWorkloadHealth(workload *pbresource.Resource) pbcatalog.Health {
	status, found := workload.Status[workloadhealth.ControllerID]
	if !found {
		return pbcatalog.Health_HEALTH_CRITICAL
	}

	for _, condition := range status.Conditions {
		if condition.Type == workloadhealth.StatusConditionHealthy {
			raw, found := pbcatalog.Health_value[condition.Reason]
			if found {
				return pbcatalog.Health(raw)
			}
			return pbcatalog.Health_HEALTH_CRITICAL
		}
	}
	return pbcatalog.Health_HEALTH_CRITICAL
}

// serviceUnderManagement detects whether this service should have its
// endpoints automatically managed by the controller
func serviceUnderManagement(svc *pbcatalog.Service) bool {
	sel := svc.GetWorkloads()
	if sel == nil {
		// The selector wasn't present at all. Therefore this service is not under
		// automatic endpoint management.
		return false
	}

	if len(sel.Names) < 1 && len(sel.Prefixes) < 1 {
		// The selector was set in the request but the list of workload names
		// and prefixes were both empty. Therefore this service is not under
		// automatic endpoint management
		return false
	}

	// Some workload selection criteria exists, so this service is considered
	// under automatic endpoint management.
	return true
}

// workloadsToEndpoints will translate the Workload resources into a ServiceEndpoints resource
func workloadsToEndpoints(svc *pbcatalog.Service, workloads []*DecodedWorkload) *pbcatalog.ServiceEndpoints {
	var endpoints []*pbcatalog.Endpoint

	for _, workload := range workloads {
		endpoint := workloadToEndpoint(svc, workload)
		if endpoint != nil {
			endpoints = append(endpoints, endpoint)
		}
	}

	return &pbcatalog.ServiceEndpoints{
		Endpoints: endpoints,
	}
}

// workloadToEndpoint will convert a workload resource into a singular Endpoint to be
// put within a ServiceEndpoints resource.
//
// The conversion process involves parsing the workloads health and filtering its
// addresses and ports down to just what the service wants to consume.
//
// Determining the workloads health requires the workload-health controller to already
// have reconciled the workloads health and stored it within the resources Status field.
// Any unreconciled workload health will be represented in the ServiceEndpoints with
// the ANY health status.
func workloadToEndpoint(svc *pbcatalog.Service, workload *DecodedWorkload) *pbcatalog.Endpoint {
	health := determineWorkloadHealth(workload.Resource)

	endpointPorts := make(map[string]*pbcatalog.WorkloadPort)

	// Create the endpoints filtered ports map. Only workload ports specified in
	// one of the services ports are included. Ports with a protocol mismatch
	// between the service and workload will be excluded as well.
	for _, svcPort := range svc.Ports {
		workloadPort, found := workload.Data.Ports[svcPort.TargetPort]
		if !found {
			// this workload doesn't have this port so ignore it
			continue
		}

		// If workload protocol is not specified, we will default to service's protocol.
		// This is because on some platforms (kubernetes), workload protocol is not always
		// known, and so we need to inherit from the service instead.
		if workloadPort.Protocol == pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
			workloadPort.Protocol = svcPort.Protocol
		} else if workloadPort.Protocol != svcPort.Protocol {
			// Otherwise, there's workload port mismatch - ignore it.
			continue
		}

		endpointPorts[svcPort.TargetPort] = workloadPort
	}

	var workloadAddrs []*pbcatalog.WorkloadAddress

	// Now we filter down the addresses and their corresponding port usage to just
	// what the service needs to consume. If the address isn't being used to serve
	// any of the services target ports, it will be entirely excluded from the
	// address list. If some but not all of its ports are served, then the list
	// of ports will be reduced to just the intersection of the service ports
	// and the workload addresses ports
	for _, addr := range workload.Data.Addresses {
		var ports []string

		if len(addr.Ports) > 0 {
			// The workload address has defined ports, filter these as necessary

			for _, portName := range addr.Ports {
				// check if the workload port has been selected by the service
				_, found := endpointPorts[portName]
				if !found {
					// this port isn't selected by the service so drop this port
					continue
				}

				ports = append(ports, portName)
			}
		} else {
			// The workload address doesn't specify ports. This lack of port specification
			// means that all ports are exposed on the interface so here we create a list
			// of all the port names exposed by the service.
			for portName := range endpointPorts {
				ports = append(ports, portName)
			}
		}

		// sort the ports to keep them stable and prevent unnecessary rewrites when the endpoints
		// get diffed
		sort.Slice(ports, func(i, j int) bool {
			return ports[i] < ports[j]
		})

		// Only record this workload address if one or more of its ports were consumed
		// by the service.
		if len(ports) > 0 {
			workloadAddrs = append(workloadAddrs, &pbcatalog.WorkloadAddress{
				Host:     addr.Host,
				External: addr.External,
				Ports:    ports,
			})
		}
	}

	// If all the workload addresses were filtered out then we should completely ignore
	// the workload. While the name matched nothing else did so it isn't useable as
	// far as the service is concerned.
	if len(workloadAddrs) < 1 {
		return nil
	}

	return &pbcatalog.Endpoint{
		TargetRef:    workload.Id,
		HealthStatus: health,
		Addresses:    workloadAddrs,
		Ports:        endpointPorts,
		Identity:     workload.Data.Identity,
		Dns:          workload.Data.Dns,
	}
}

func workloadIdentityStatusFromEndpoints(endpoints *pbcatalog.ServiceEndpoints) *pbresource.Condition {
	identities := endpoints.GetIdentities()

	if len(identities) > 0 {
		return ConditionIdentitiesFound(identities)
	}

	return ConditionIdentitiesNotFound
}

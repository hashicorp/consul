// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package endpoints

import (
	"context"
	"sort"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	endpointsMetaManagedBy = "managed-by-controller"
)

// The WorkloadMapper interface is used to provide an implementation around being able
// to map a watch even for a Workload resource and translate it to reconciliation requests
type WorkloadMapper interface {
	// MapWorkload conforms to the controller.DependencyMapper signature. Given a Workload
	// resource it should report the resource IDs that have selected the workload.
	MapWorkload(context.Context, controller.Runtime, *pbresource.Resource) ([]controller.Request, error)

	// TrackIDForSelector should be used to associate the specified WorkloadSelector with
	// the given resource ID. Future calls to MapWorkload
	TrackIDForSelector(*pbresource.ID, *pbcatalog.WorkloadSelector)

	// UntrackID should be used to inform the tracker to forget about the specified ID
	UntrackID(*pbresource.ID)
}

// ServiceEndpointsController creates a controller to perform automatic endpoint management for
// services.
func ServiceEndpointsController(workloadMap WorkloadMapper) controller.Controller {
	if workloadMap == nil {
		panic("No WorkloadMapper was provided to the ServiceEndpointsController constructor")
	}

	return controller.ForType(types.ServiceEndpointsType).
		WithWatch(types.ServiceType, controller.ReplaceType(types.ServiceEndpointsType)).
		WithWatch(types.WorkloadType, workloadMap.MapWorkload).
		WithReconciler(newServiceEndpointsReconciler(workloadMap))
}

type serviceEndpointsReconciler struct {
	workloadMap WorkloadMapper
}

func newServiceEndpointsReconciler(workloadMap WorkloadMapper) *serviceEndpointsReconciler {
	return &serviceEndpointsReconciler{
		workloadMap: workloadMap,
	}
}

// Reconcile will reconcile one ServiceEndpoints resource in response to some event.
func (r *serviceEndpointsReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// The runtime is passed by value so replacing it here for the remainder of this
	// reconciliation request processing will not affect future invocations.
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)

	rt.Logger.Trace("reconciling service endpoints")

	endpointsID := req.ID
	serviceID := &pbresource.ID{
		Type:    types.ServiceType,
		Tenancy: endpointsID.Tenancy,
		Name:    endpointsID.Name,
	}

	// First we read and unmarshal the service

	serviceData, err := getServiceData(ctx, rt, serviceID)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding Service", "error", err)
		return err
	}

	// Check if the service exists. If it doesn't we can avoid a bunch of other work.
	if serviceData == nil {
		rt.Logger.Trace("service has been deleted")

		// The service was deleted so we need to update the WorkloadMapper to tell it to
		// stop tracking this service
		r.workloadMap.UntrackID(req.ID)

		// Note that because we configured ServiceEndpoints to be owned by the service,
		// the service endpoints object should eventually be automatically deleted.
		// There is no reason to attempt deletion here.
		return nil
	}

	// Now read and unmarshal the endpoints. We don't need this data just yet but all
	// code paths from this point on will need this regardless of branching so we pull
	// it now.
	endpointsData, err := getEndpointsData(ctx, rt, endpointsID)
	if err != nil {
		rt.Logger.Error("error retrieving existing endpoints", "error", err)
		return err
	}

	var status *pbresource.Condition

	if serviceUnderManagement(serviceData.service) {
		rt.Logger.Trace("service is enabled for automatic endpoint management")
		// This service should have its endpoints automatically managed
		status = ConditionManaged

		// Inform the WorkloadMapper to track this service and its selectors. So
		// future workload updates that would be matched by the services selectors
		// cause this service to be rereconciled.
		r.workloadMap.TrackIDForSelector(req.ID, serviceData.service.GetWorkloads())

		// Now read and umarshal all workloads selected by the service. It is imperative
		// that this happens after we notify the selection tracker to be tracking that
		// selection criteria. If the order were reversed we could potentially miss
		// workload creations that should be selected if they happen after gathering
		// the workloads but before tracking the selector. Tracking first ensures that
		// any event that happens after that would get mapped to an event for these
		// endpoints.
		workloadData, err := getWorkloadData(ctx, rt, serviceData)
		if err != nil {
			rt.Logger.Trace("error retrieving selected workloads", "error", err)
			return err
		}

		// Calculate the latest endpoints from the already gathered workloads
		latestEndpoints := workloadsToEndpoints(serviceData.service, workloadData)

		// Before writing the endpoints actually check to see if they are changed
		if endpointsData == nil || !proto.Equal(endpointsData.endpoints, latestEndpoints) {
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
					Owner: serviceData.resource.Id,
					Metadata: map[string]string{
						endpointsMetaManagedBy: StatusKey,
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
		status = ConditionUnmanaged

		// Inform the WorkloadMapper that it no longer needs to track this service
		// as it is no longer under endpoint management
		r.workloadMap.UntrackID(req.ID)

		// Delete the managed ServiceEndpoints if necessary if the metadata would
		// indicate that they were previously managed by this controller
		if endpointsData != nil && endpointsData.resource.Metadata[endpointsMetaManagedBy] == StatusKey {
			rt.Logger.Trace("removing previous managed endpoints")

			// This performs a CAS deletion to protect against the case where the user
			// has overwritten the endpoints since we fetched them.
			_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{
				Id:      endpointsData.resource.Id,
				Version: endpointsData.resource.Version,
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
		ObservedGeneration: serviceData.resource.Generation,
		Conditions: []*pbresource.Condition{
			status,
		},
	}
	// If the status is unchanged then we should return and avoid the unnecessary write
	if resource.EqualStatus(serviceData.resource.Status[StatusKey], newStatus, false) {
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     serviceData.resource.Id,
		Key:    StatusKey,
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
	status, found := workload.Status[workloadhealth.StatusKey]
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
func workloadsToEndpoints(svc *pbcatalog.Service, workloads []*workloadData) *pbcatalog.ServiceEndpoints {
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
func workloadToEndpoint(svc *pbcatalog.Service, data *workloadData) *pbcatalog.Endpoint {
	health := determineWorkloadHealth(data.resource)

	endpointPorts := make(map[string]*pbcatalog.WorkloadPort)

	// Create the endpoints filtered ports map. Only workload ports specified in
	// one of the services ports are included. Ports with a protocol mismatch
	// between the service and workload will be excluded as well.
	for _, svcPort := range svc.Ports {
		workloadPort, found := data.workload.Ports[svcPort.TargetPort]
		if !found {
			// this workload doesn't have this port so ignore it
			continue
		}

		if workloadPort.Protocol != svcPort.Protocol {
			// workload port mismatch - ignore it
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
	for _, addr := range data.workload.Addresses {
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
		TargetRef:    data.resource.Id,
		HealthStatus: health,
		Addresses:    workloadAddrs,
		Ports:        endpointPorts,
	}
}

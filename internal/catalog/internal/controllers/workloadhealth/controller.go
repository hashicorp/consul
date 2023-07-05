package workloadhealth

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	errNodeUnreconciled            = errors.New("Node health has not been reconciled yet")
	errNodeHealthInvalid           = errors.New("Node health has invalid reason")
	errNodeHealthConditionNotFound = fmt.Errorf("Node health status is missing the %s condition", nodehealth.StatusConditionHealthy)
)

// The NodeMapper interface is used to provide an implementation around being able to
// map a watch event for a Node resource and translate it to reconciliation requests
// for all Workloads assigned to that node.
type NodeMapper interface {
	// MapNodeToWorkloads will take a Node resource and return controller requests
	// for all Workloads associated with the Node.
	MapNodeToWorkloads(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error)

	// TrackWorkload instructs the NodeMapper to associate the given workload
	// ID with the given node ID.
	TrackWorkload(workloadID *pbresource.ID, nodeID *pbresource.ID)

	// UntrackWorkload instructs the Nodemapper to forget about any
	// association it was tracking for this workload.
	UntrackWorkload(workloadID *pbresource.ID)

	// NodeIDFromWorkload is used to generate the resource ID for the Node referenced
	// within the NodeName field of the Workload.
	NodeIDFromWorkload(workload *pbresource.Resource, workloadData *pbcatalog.Workload) *pbresource.ID
}

func WorkloadHealthController(nodeMap NodeMapper) controller.Controller {
	if nodeMap == nil {
		panic("No NodeMapper was provided to the WorkloadHealthController constructor")
	}

	return controller.ForType(types.WorkloadType).
		WithWatch(types.HealthStatusType, controller.MapOwnerFiltered(types.WorkloadType)).
		WithWatch(types.NodeType, nodeMap.MapNodeToWorkloads).
		WithReconciler(&workloadHealthReconciler{nodeMap: nodeMap})
}

type workloadHealthReconciler struct {
	nodeMap NodeMapper
}

func (r *workloadHealthReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// The runtime is passed by value so replacing it here for the remainder of this
	// reconciliation request processing will not affect future invocations.
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", StatusKey)

	rt.Logger.Trace("reconciling workload health")

	// read the workload
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		rt.Logger.Trace("workload has been deleted")
		r.nodeMap.UntrackWorkload(req.ID)
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}

	res := rsp.Resource
	var workload pbcatalog.Workload
	if err := res.Data.UnmarshalTo(&workload); err != nil {
		// This should be impossible and will not be exercised in tests. Various
		// type validations on admission ensure that all Workloads would
		// be marshallable in this way.
		rt.Logger.Error("error unmarshalling workload data", "error", err)
		return err
	}

	nodeHealth := pbcatalog.Health_HEALTH_PASSING
	if workload.NodeName != "" {
		nodeID := r.nodeMap.NodeIDFromWorkload(res, &workload)
		r.nodeMap.TrackWorkload(res.Id, nodeID)

		// It is important that getting the nodes health happens after tracking the
		// Workload with the node mapper. If the order were reversed we could
		// potentially miss events for data that changes after we read the node but
		// before we configured the node mapper to map subsequent events to this
		// workload.
		nodeHealth, err = getNodeHealth(ctx, rt, nodeID)
		if err != nil {
			rt.Logger.Error("error looking up node health", "error", err, "node-id", nodeID)
			return err
		}
	} else {
		// the node association may be been removed so stop tracking it.
		r.nodeMap.UntrackWorkload(res.Id)
	}

	workloadHealth, err := getWorkloadHealth(ctx, rt, req.ID)
	if err != nil {
		// This should be impossible under normal operations and will not be exercised
		// within the unit tests. This can only fail if the resource service fails
		// or allows admission of invalid health statuses.
		rt.Logger.Error("error aggregating workload health statuses", "error", err)
		return err
	}

	health := nodeHealth
	if workloadHealth > health {
		health = workloadHealth
	}

	condition := WorkloadConditions[workloadHealth]
	if workload.NodeName != "" {
		condition = NodeAndWorkloadConditions[workloadHealth][nodeHealth]
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions: []*pbresource.Condition{
			condition,
		},
	}

	if resource.EqualStatus(res.Status[StatusKey], newStatus, false) {
		rt.Logger.Trace("resources workload health status is unchanged",
			"health", health.String(),
			"node-health", nodeHealth.String(),
			"workload-health", workloadHealth.String())
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    StatusKey,
		Status: newStatus,
	})

	if err != nil {
		rt.Logger.Error("error encountered when attempting to update the resources workload status", "error", err)
		return err
	}

	rt.Logger.Trace("resource's workload health status was updated",
		"health", health.String(),
		"node-health", nodeHealth.String(),
		"workload-health", workloadHealth.String())
	return nil
}

func getNodeHealth(ctx context.Context, rt controller.Runtime, nodeRef *pbresource.ID) (pbcatalog.Health, error) {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: nodeRef})
	switch {
	case status.Code(err) == codes.NotFound:
		return pbcatalog.Health_HEALTH_CRITICAL, nil
	case err != nil:
		return pbcatalog.Health_HEALTH_CRITICAL, err
	default:
		healthStatus, ok := rsp.Resource.Status[nodehealth.StatusKey]
		if !ok {
			// The Nodes health has never been reconciled and therefore the
			// workloads health cannot be determined. Returning nil is acceptable
			// because the controller should sometime soon run reconciliation for
			// the node which will then trigger rereconciliation of this workload
			return pbcatalog.Health_HEALTH_CRITICAL, errNodeUnreconciled
		}

		for _, condition := range healthStatus.Conditions {
			if condition.Type == nodehealth.StatusConditionHealthy {
				if condition.State == pbresource.Condition_STATE_TRUE {
					return pbcatalog.Health_HEALTH_PASSING, nil
				}

				healthReason, valid := pbcatalog.Health_value[condition.Reason]
				if !valid {
					// The Nodes health is unknown - presumably the node health controller
					// will come along and fix that up momentarily causing this workload
					// reconciliation to occur again.
					return pbcatalog.Health_HEALTH_CRITICAL, errNodeHealthInvalid
				}
				return pbcatalog.Health(healthReason), nil
			}
		}
		return pbcatalog.Health_HEALTH_CRITICAL, errNodeHealthConditionNotFound
	}
}

func getWorkloadHealth(ctx context.Context, rt controller.Runtime, workloadRef *pbresource.ID) (pbcatalog.Health, error) {
	rsp, err := rt.Client.ListByOwner(ctx, &pbresource.ListByOwnerRequest{
		Owner: workloadRef,
	})

	if err != nil {
		return pbcatalog.Health_HEALTH_CRITICAL, err
	}

	workloadHealth := pbcatalog.Health_HEALTH_PASSING

	for _, res := range rsp.Resources {
		if resource.EqualType(res.Id.Type, types.HealthStatusType) {
			var hs pbcatalog.HealthStatus
			if err := res.Data.UnmarshalTo(&hs); err != nil {
				// This should be impossible and will not be executing in tests. The resource type
				// is the HealthStatus type and therefore must be unmarshallable into the HealthStatus
				// object or else it wouldn't have passed admission validation checks.
				return workloadHealth, fmt.Errorf("error unmarshalling health status data: %w", err)
			}

			if hs.Status > workloadHealth {
				workloadHealth = hs.Status
			}
		}
	}

	return workloadHealth, nil
}

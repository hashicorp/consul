package workloadhealth

import (
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	StatusKey              = "consul.io/workload-health"
	StatusConditionHealthy = "healthy"

	NodeAndWorkloadHealthyMessage   = "All workload and associated node health checks are passing"
	WorkloadHealthyMessage          = "All workload health checks are passing"
	NodeAndWorkloadUnhealthyMessage = "One or more workload and node health checks are not passing"
	WorkloadUnhealthyMessage        = "One or more workload health checks are not passing"
)

var (
	ConditionWorkloadPassing = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  pbcatalog.Health_HEALTH_PASSING.String(),
		Message: WorkloadHealthyMessage,
	}

	ConditionWorkloadWarning = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_WARNING.String(),
		Message: WorkloadUnhealthyMessage,
	}

	ConditionWorkloadCritical = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_CRITICAL.String(),
		Message: WorkloadUnhealthyMessage,
	}

	ConditionWorkloadMaintenance = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_MAINTENANCE.String(),
		Message: WorkloadUnhealthyMessage,
	}

	ConditionNodeAndWorkloadPassing = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_TRUE,
		Reason:  pbcatalog.Health_HEALTH_PASSING.String(),
		Message: NodeAndWorkloadHealthyMessage,
	}

	ConditionNodeAndWorkloadWarning = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_WARNING.String(),
		Message: NodeAndWorkloadUnhealthyMessage,
	}

	ConditionNodeAndWorkloadCritical = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_CRITICAL.String(),
		Message: NodeAndWorkloadUnhealthyMessage,
	}

	ConditionNodeAndWorkloadMaintenance = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_MAINTENANCE.String(),
		Message: NodeAndWorkloadUnhealthyMessage,
	}

	ConditionNodeWarning = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_WARNING.String(),
		Message: nodehealth.NodeUnhealthyMessage,
	}

	ConditionNodeCritical = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_CRITICAL.String(),
		Message: nodehealth.NodeUnhealthyMessage,
	}

	ConditionNodeMaintenance = &pbresource.Condition{
		Type:    StatusConditionHealthy,
		State:   pbresource.Condition_STATE_FALSE,
		Reason:  pbcatalog.Health_HEALTH_MAINTENANCE.String(),
		Message: nodehealth.NodeUnhealthyMessage,
	}

	// WorkloadConditions is a map of the workloadhealth to the status condition
	// used to represent that health.
	WorkloadConditions = map[pbcatalog.Health]*pbresource.Condition{
		pbcatalog.Health_HEALTH_PASSING:     ConditionWorkloadPassing,
		pbcatalog.Health_HEALTH_WARNING:     ConditionWorkloadWarning,
		pbcatalog.Health_HEALTH_CRITICAL:    ConditionWorkloadCritical,
		pbcatalog.Health_HEALTH_MAINTENANCE: ConditionWorkloadMaintenance,
	}

	// NodeAndWorkloadConditions is a map whose ultimate values are the status conditions
	// used to represent the combined health of a workload and its associated node.
	// The outer map's keys are the workloads health and the inner maps keys are the nodes
	// health
	NodeAndWorkloadConditions = map[pbcatalog.Health]map[pbcatalog.Health]*pbresource.Condition{
		pbcatalog.Health_HEALTH_PASSING: {
			pbcatalog.Health_HEALTH_PASSING:     ConditionNodeAndWorkloadPassing,
			pbcatalog.Health_HEALTH_WARNING:     ConditionNodeWarning,
			pbcatalog.Health_HEALTH_CRITICAL:    ConditionNodeCritical,
			pbcatalog.Health_HEALTH_MAINTENANCE: ConditionNodeMaintenance,
		},
		pbcatalog.Health_HEALTH_WARNING: {
			pbcatalog.Health_HEALTH_PASSING:     ConditionWorkloadWarning,
			pbcatalog.Health_HEALTH_WARNING:     ConditionNodeAndWorkloadWarning,
			pbcatalog.Health_HEALTH_CRITICAL:    ConditionNodeAndWorkloadCritical,
			pbcatalog.Health_HEALTH_MAINTENANCE: ConditionNodeAndWorkloadMaintenance,
		},
		pbcatalog.Health_HEALTH_CRITICAL: {
			pbcatalog.Health_HEALTH_PASSING:     ConditionWorkloadCritical,
			pbcatalog.Health_HEALTH_WARNING:     ConditionNodeAndWorkloadCritical,
			pbcatalog.Health_HEALTH_CRITICAL:    ConditionNodeAndWorkloadCritical,
			pbcatalog.Health_HEALTH_MAINTENANCE: ConditionNodeAndWorkloadMaintenance,
		},
		pbcatalog.Health_HEALTH_MAINTENANCE: {
			pbcatalog.Health_HEALTH_PASSING:     ConditionWorkloadMaintenance,
			pbcatalog.Health_HEALTH_WARNING:     ConditionNodeAndWorkloadMaintenance,
			pbcatalog.Health_HEALTH_CRITICAL:    ConditionNodeAndWorkloadMaintenance,
			pbcatalog.Health_HEALTH_MAINTENANCE: ConditionNodeAndWorkloadMaintenance,
		},
	}
)

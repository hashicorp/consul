package workloadhealth

const (
	StatusKey              = "consul.io/workload-health"
	StatusConditionHealthy = "healthy"

	NodeAndWorkloadHealthyMessage   = "All workload and associated node health checks are passing"
	WorkloadHealthyMessage          = "All workload health checks are passing"
	NodeAndWorkloadUnhealthyMessage = "One or more workload and node health checks are not passing"
	WorkloadUnhealthyMessage        = "One or more workload health checks are not passing"
)

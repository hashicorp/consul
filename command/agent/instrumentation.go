package agent

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/consul/structs"
)

var (
	DefaultAgentIntrumentationInterval = 5 * time.Second
	AgentIntrumentationInterval        = DefaultAgentIntrumentationInterval
)

func (a *Agent) instrumentServices() {
	t := time.NewTicker(AgentIntrumentationInterval)

	// As node checks have an empty ServiceID, the status of the node as a whole
	// will be listed under the service ""
	nodeService := ""

	severities := map[string]int{
		structs.HealthUnknown:  0,
		structs.HealthPassing:  1,
		structs.HealthWarning:  2,
		structs.HealthCritical: 3,
	}

	for {
		select {
		case <-a.shutdownCh:
			t.Stop()
			return
		case <-t.C:
			statusOfEachService := map[string]string{
				nodeService: structs.HealthPassing,
			}

			// We aggregate service name => status => number of instances in this state
			// as multiple instances of a service could be running on the same node,
			// in varying states
			aggregatedServiceStatus := map[string]map[string]float32{}

			// Work out the "worst case" status for each service
			for _, check := range a.state.Checks() {
				currentlyKnownServiceStatus := statusOfEachService[check.ServiceID]

				if severities[check.Status] > severities[currentlyKnownServiceStatus] {
					statusOfEachService[check.ServiceID] = check.Status
				}
			}

			for _, service := range a.state.Services() {
				status := statusOfEachService[service.ID]

				if severities[statusOfEachService[nodeService]] > severities[status] {
					status = statusOfEachService[nodeService]
				}

				if _, ok := aggregatedServiceStatus[service.Service]; !ok {
					aggregatedServiceStatus[service.Service] = map[string]float32{
						structs.HealthPassing:  0,
						structs.HealthWarning:  0,
						structs.HealthCritical: 0,
					}
				}

				aggregatedServiceStatus[service.Service][status] += 1
			}

			for service, instances := range aggregatedServiceStatus {
				for status, count := range instances {
					metrics.SetGauge([]string{"services", service, "instances_by_state", status}, count)
				}
			}
		}
	}
}

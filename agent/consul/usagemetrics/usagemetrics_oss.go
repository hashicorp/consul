// +build !consulent

package usagemetrics

import (
	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/state"
)

func (u *UsageMetricsReporter) emitServiceUsage(serviceUsage state.ServiceUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "services"},
		float32(serviceUsage.Services),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "service_instances"},
		float32(serviceUsage.ServiceInstances),
		u.metricLabels,
	)
}

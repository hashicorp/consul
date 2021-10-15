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

	for k, i := range serviceUsage.ConnectServiceInstances {
		metrics.SetGaugeWithLabels(
			[]string{"consul", "state", "connect_instances"},
			float32(i),
			append(u.metricLabels, metrics.Label{Name: "kind", Value: k}),
		)
	}
}

func (u *UsageMetricsReporter) emitKVUsage(kvUsage state.KVUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "kv_entries"},
		float32(kvUsage.KVCount),
		u.metricLabels,
	)
}

func (u *UsageMetricsReporter) emitConfigEntryUsage(configUsage state.ConfigEntryUsage) {
	for k, i := range configUsage.ConfigByKind {
		metrics.SetGaugeWithLabels(
			[]string{"consul", "state", "config_entries"},
			float32(i),
			append(u.metricLabels, metrics.Label{Name: "kind", Value: k}),
		)
	}
}

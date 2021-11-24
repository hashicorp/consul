//go:build !consulent
// +build !consulent

package usagemetrics

import (
	"github.com/armon/go-metrics"

	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/consul/state"
)

func (u *UsageMetricsReporter) emitNodeUsage(nodeUsage state.NodeUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "nodes"},
		float32(nodeUsage.Nodes),
		u.metricLabels,
	)
}

func (u *UsageMetricsReporter) emitMemberUsage(members []serf.Member) {
	var (
		servers int
		clients int
	)
	for _, m := range members {
		switch m.Tags["role"] {
		case "node":
			clients++
		case "consul":
			servers++
		}
	}

	metrics.SetGaugeWithLabels(
		[]string{"consul", "members", "clients"},
		float32(clients),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"consul", "members", "servers"},
		float32(servers),
		u.metricLabels,
	)
}

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

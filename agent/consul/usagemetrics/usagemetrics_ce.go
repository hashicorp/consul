// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package usagemetrics

import (
	"github.com/armon/go-metrics"

	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

func (u *UsageMetricsReporter) emitNodeUsage(nodeUsage state.NodeUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "nodes"},
		float32(nodeUsage.Nodes),
		u.metricLabels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"state", "nodes"},
		float32(nodeUsage.Nodes),
		u.metricLabels,
	)
}

func (u *UsageMetricsReporter) emitPeeringUsage(peeringUsage state.PeeringUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "peerings"},
		float32(peeringUsage.Peerings),
		u.metricLabels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"state", "peerings"},
		float32(peeringUsage.Peerings),
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
		[]string{"members", "clients"},
		float32(clients),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"consul", "members", "servers"},
		float32(servers),
		u.metricLabels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"members", "servers"},
		float32(servers),
		u.metricLabels,
	)
}

func (u *UsageMetricsReporter) emitServiceUsage(serviceUsage structs.ServiceUsage) {
	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "services"},
		float32(serviceUsage.Services),
		u.metricLabels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"state", "services"},
		float32(serviceUsage.Services),
		u.metricLabels,
	)

	metrics.SetGaugeWithLabels(
		[]string{"consul", "state", "service_instances"},
		float32(serviceUsage.ServiceInstances),
		u.metricLabels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"state", "service_instances"},
		float32(serviceUsage.ServiceInstances),
		u.metricLabels,
	)
	metrics.SetGaugeWithLabels(
		[]string{"state", "billable_service_instances"},
		float32(serviceUsage.BillableServiceInstances),
		u.metricLabels,
	)

	for k, i := range serviceUsage.ConnectServiceInstances {
		metrics.SetGaugeWithLabels(
			[]string{"consul", "state", "connect_instances"},
			float32(i),
			append(u.metricLabels, metrics.Label{Name: "kind", Value: k}),
		)
		metrics.SetGaugeWithLabels(
			[]string{"state", "connect_instances"},
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
	metrics.SetGaugeWithLabels(
		[]string{"state", "kv_entries"},
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
		metrics.SetGaugeWithLabels(
			[]string{"state", "config_entries"},
			float32(i),
			append(u.metricLabels, metrics.Label{Name: "kind", Value: k}),
		)
	}
}

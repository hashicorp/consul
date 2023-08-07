// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hoststats

import (
	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
)

// Metrics defines an interface for the methods used to emit data to the go-metrics library.
// `metrics.Default()` should always satisfy this interface.
type Metrics interface {
	SetGaugeWithLabels(key []string, val float32, labels []metrics.Label)
}

var Gauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"host", "memory", "total"},
		Help: "Total physical memory in bytes",
	},
	{
		Name: []string{"host", "memory", "available"},
		Help: "Available physical memory in bytes",
	},
	{
		Name: []string{"host", "memory", "free"},
		Help: "Free physical memory in bytes",
	},
	{
		Name: []string{"host", "memory", "used"},
		Help: "Used physical memory in bytes",
	},
	{
		Name: []string{"host", "memory", "used_percent"},
		Help: "Percentage of physical memory in use",
	},
	{
		Name: []string{"host", "cpu", "total"},
		Help: "Total cpu utilization",
	},
	{
		Name: []string{"host", "cpu", "user"},
		Help: "User cpu utilization",
	},
	{
		Name: []string{"host", "cpu", "idle"},
		Help: "Idle cpu utilization",
	},
	{
		Name: []string{"host", "cpu", "iowait"},
		Help: "Iowait cpu utilization",
	},
	{
		Name: []string{"host", "cpu", "system"},
		Help: "System cpu utilization",
	},
	{
		Name: []string{"host", "disk", "size"},
		Help: "Size of disk in bytes",
	},
	{
		Name: []string{"host", "disk", "used"},
		Help: "Disk usage in bytes",
	},
	{
		Name: []string{"host", "disk", "available"},
		Help: "Available bytes on disk",
	},
	{
		Name: []string{"host", "disk", "used_percent"},
		Help: "Percentage of disk space usage",
	},
	{
		Name: []string{"host", "disk", "inodes_percent"},
		Help: "Percentage of disk inodes usage",
	},
	{
		Name: []string{"host", "uptime"},
		Help: "System uptime",
	},
}

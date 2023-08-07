// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hoststats

import (
	"time"

	"github.com/armon/go-metrics"
)

var hostStatsCollectionInterval = 10 * time.Second

// HostStats represents resource usage hoststats of the host running a Consul agent
type HostStats struct {
	Memory       *MemoryStats
	CPU          []*CPUStats
	DataDirStats *DiskStats
	Uptime       uint64
	Timestamp    int64
}

func (hs *HostStats) Clone() *HostStats {
	clone := &HostStats{}
	*clone = *hs
	return clone
}

func (hs *HostStats) Emit(sink Metrics, baseLabels []metrics.Label) {

	if hs.Memory != nil {
		sink.SetGaugeWithLabels([]string{"host", "memory", "total"}, float32(hs.Memory.Total), baseLabels)
		sink.SetGaugeWithLabels([]string{"host", "memory", "available"}, float32(hs.Memory.Available), baseLabels)
		sink.SetGaugeWithLabels([]string{"host", "memory", "used"}, float32(hs.Memory.Used), baseLabels)
		sink.SetGaugeWithLabels([]string{"host", "memory", "used_percent"}, float32(hs.Memory.UsedPercent), baseLabels)
		sink.SetGaugeWithLabels([]string{"host", "memory", "free"}, float32(hs.Memory.Free), baseLabels)
	}

	for _, cpu := range hs.CPU {
		labels := append(baseLabels, metrics.Label{
			Name:  "cpu",
			Value: cpu.CPU,
		})

		sink.SetGaugeWithLabels([]string{"host", "cpu", "total"}, float32(cpu.Total), labels)
		sink.SetGaugeWithLabels([]string{"host", "cpu", "user"}, float32(cpu.User), labels)
		sink.SetGaugeWithLabels([]string{"host", "cpu", "idle"}, float32(cpu.Idle), labels)
		sink.SetGaugeWithLabels([]string{"host", "cpu", "iowait"}, float32(cpu.Iowait), labels)
		sink.SetGaugeWithLabels([]string{"host", "cpu", "system"}, float32(cpu.System), labels)
	}

	if hs.DataDirStats != nil {
		diskLabels := append(baseLabels, metrics.Label{
			Name:  "path",
			Value: hs.DataDirStats.Path,
		})

		sink.SetGaugeWithLabels([]string{"host", "disk", "size"}, float32(hs.DataDirStats.Size), diskLabels)
		sink.SetGaugeWithLabels([]string{"host", "disk", "used"}, float32(hs.DataDirStats.Used), diskLabels)
		sink.SetGaugeWithLabels([]string{"host", "disk", "available"}, float32(hs.DataDirStats.Available), diskLabels)
		sink.SetGaugeWithLabels([]string{"host", "disk", "used_percent"}, float32(hs.DataDirStats.UsedPercent), diskLabels)
		sink.SetGaugeWithLabels([]string{"host", "disk", "inodes_percent"}, float32(hs.DataDirStats.InodesUsedPercent), diskLabels)
	}

	sink.SetGaugeWithLabels([]string{"host", "uptime"}, float32(hs.Uptime), baseLabels)
}

// CPUStats represents hoststats related to cpu usage
type CPUStats struct {
	CPU    string
	User   float64
	System float64
	Idle   float64
	Iowait float64
	Total  float64
}

// MemoryStats represents hoststats related to virtual memory usage
type MemoryStats struct {
	Total       uint64
	Available   uint64
	Used        uint64
	UsedPercent float64
	Free        uint64
}

// DiskStats represents hoststats related to disk usage
type DiskStats struct {
	Path              string
	Size              uint64
	Used              uint64
	Available         uint64
	UsedPercent       float64
	InodesUsedPercent float64
}

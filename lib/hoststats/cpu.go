package hoststats

import (
	"math"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

// cpuStatsCalculator calculates cpu usage percentages
type cpuStatsCalculator struct {
	prev      cpu.TimesStat
	prevBusy  float64
	prevTotal float64
}

// calculate calculates the current cpu usage percentages
func (h *cpuStatsCalculator) calculate(times cpu.TimesStat) *CPUStats {

	currentBusy := times.User + times.System + times.Nice + times.Iowait + times.Irq +
		times.Softirq + times.Steal + times.Guest + times.GuestNice
	currentTotal := currentBusy + times.Idle

	deltaTotal := currentTotal - h.prevTotal
	stats := &CPUStats{
		CPU: times.CPU,

		Idle:   ((times.Idle - h.prev.Idle) / deltaTotal) * 100,
		User:   ((times.User - h.prev.User) / deltaTotal) * 100,
		System: ((times.System - h.prev.System) / deltaTotal) * 100,
		Iowait: ((times.Iowait - h.prev.Iowait) / deltaTotal) * 100,
		Total:  ((currentBusy - h.prevBusy) / deltaTotal) * 100,
	}

	// Protect against any invalid values
	if math.IsNaN(stats.Idle) || math.IsInf(stats.Idle, 0) {
		stats.Idle = 100.0
	}
	if math.IsNaN(stats.User) || math.IsInf(stats.User, 0) {
		stats.User = 0.0
	}
	if math.IsNaN(stats.System) || math.IsInf(stats.System, 0) {
		stats.System = 0.0
	}
	if math.IsNaN(stats.Iowait) || math.IsInf(stats.Iowait, 0) {
		stats.Iowait = 0.0
	}
	if math.IsNaN(stats.Total) || math.IsInf(stats.Total, 0) {
		stats.Total = 0.0
	}

	h.prev = times
	h.prevTotal = currentTotal
	h.prevBusy = currentBusy
	return stats
}

// cpuStats calculates cpu usage percentage
type cpuStats struct {
	prevCpuTime float64
	prevTime    time.Time

	totalCpus int
}

func (h *Collector) collectCPUStats() (cpus []*CPUStats, err error) {

	cpuStats, err := cpu.Times(true)
	if err != nil {
		return nil, err
	}
	cs := make([]*CPUStats, len(cpuStats))
	for idx, cpuStat := range cpuStats {
		percentCalculator, ok := h.cpuCalculator[cpuStat.CPU]
		if !ok {
			percentCalculator = &cpuStatsCalculator{}
			h.cpuCalculator[cpuStat.CPU] = percentCalculator
		}
		cs[idx] = percentCalculator.calculate(cpuStat)
	}

	return cs, nil
}

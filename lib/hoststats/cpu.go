package hoststats

import (
	"math"

	"github.com/shirou/gopsutil/v3/cpu"
)

// cpuStatsCalculator calculates cpu usage percentages
type cpuStatsCalculator struct {
	prev      cpu.TimesStat
	prevBusy  float64
	prevTotal float64
}

// calculate the current cpu usage percentages.
// Since the cpu.TimesStat captures the total time a cpu spent in various states
// this function tracks the last seen stat and derives each cpu state's utilization
// as a percentage of the total change in cpu time between calls.
// The first time calculate is called CPUStats will report %100 idle
// usage since there is not a previous value to calculate against
func (h *cpuStatsCalculator) calculate(times cpu.TimesStat) *CPUStats {

	// sum all none idle counters to get the total busy cpu time
	currentBusy := times.User + times.System + times.Nice + times.Iowait + times.Irq +
		times.Softirq + times.Steal + times.Guest + times.GuestNice
	// sum of the total cpu time
	currentTotal := currentBusy + times.Idle

	// calculate how much cpu time has passed since last calculation
	deltaTotal := currentTotal - h.prevTotal

	stats := &CPUStats{
		CPU: times.CPU,

		// calculate each percentage as the ratio of the change
		// in each state's time to the total change in cpu time
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

func (c *Collector) collectCPUStats() (cpus []*CPUStats, err error) {

	cpuStats, err := cpu.Times(true)
	if err != nil {
		return nil, err
	}
	cs := make([]*CPUStats, len(cpuStats))
	for idx, cpuStat := range cpuStats {
		percentCalculator, ok := c.cpuCalculator[cpuStat.CPU]
		if !ok {
			percentCalculator = &cpuStatsCalculator{}
			c.cpuCalculator[cpuStat.CPU] = percentCalculator
		}
		cs[idx] = percentCalculator.calculate(cpuStat)
	}

	return cs, nil
}

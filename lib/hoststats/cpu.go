package hoststats

import (
	"math"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

// cpuStatsCalculator calculates cpu usage percentages
type cpuStatsCalculator struct {
	prevIdle   float64
	prevUser   float64
	prevSystem float64
	prevBusy   float64
	prevTotal  float64
}

// calculate calculates the current cpu usage percentages
func (h *cpuStatsCalculator) calculate(times cpu.TimesStat) (idle float64, user float64, system float64, total float64) {
	currentIdle := times.Idle
	currentUser := times.User
	currentSystem := times.System
	currentTotal := times.Total()
	currentBusy := times.User + times.System + times.Nice + times.Iowait + times.Irq +
		times.Softirq + times.Steal + times.Guest + times.GuestNice

	deltaTotal := currentTotal - h.prevTotal
	idle = ((currentIdle - h.prevIdle) / deltaTotal) * 100
	user = ((currentUser - h.prevUser) / deltaTotal) * 100
	system = ((currentSystem - h.prevSystem) / deltaTotal) * 100
	total = ((currentBusy - h.prevBusy) / deltaTotal) * 100

	// Protect against any invalid values
	if math.IsNaN(idle) || math.IsInf(idle, 0) {
		idle = 100.0
	}
	if math.IsNaN(user) || math.IsInf(user, 0) {
		user = 0.0
	}
	if math.IsNaN(system) || math.IsInf(system, 0) {
		system = 0.0
	}
	if math.IsNaN(total) || math.IsInf(total, 0) {
		total = 0.0
	}

	h.prevIdle = currentIdle
	h.prevUser = currentUser
	h.prevSystem = currentSystem
	h.prevTotal = currentTotal
	h.prevBusy = currentBusy
	return
}

// cpuStats calculates cpu usage percentage
type cpuStats struct {
	prevCpuTime float64
	prevTime    time.Time

	totalCpus int
}

// percent calculates the cpu usage percentage based on the current cpu usage
// and the previous cpu usage where usage is given as time in nanoseconds spend
// in the cpu
func (c *cpuStats) percent(cpuTime float64) float64 {
	now := time.Now()

	if c.prevCpuTime == 0.0 {
		// invoked first time
		c.prevCpuTime = cpuTime
		c.prevTime = now
		return 0.0
	}

	timeDelta := now.Sub(c.prevTime).Nanoseconds()
	ret := c.calculatePercent(c.prevCpuTime, cpuTime, timeDelta)
	c.prevCpuTime = cpuTime
	c.prevTime = now
	return ret
}

func (c *cpuStats) calculatePercent(t1, t2 float64, timeDelta int64) float64 {
	vDelta := t2 - t1
	if timeDelta <= 0 || vDelta <= 0.0 {
		return 0.0
	}

	overall_percent := (vDelta / float64(timeDelta)) * 100.0
	return overall_percent
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
		idle, user, system, total := percentCalculator.calculate(cpuStat)
		cs[idx] = &CPUStats{
			CPU:    cpuStat.CPU,
			User:   user,
			System: system,
			Idle:   idle,
			Total:  total,
		}
	}

	return cs, nil
}

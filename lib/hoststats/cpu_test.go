package hoststats

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostStats_CPU(t *testing.T) {
	logger := testutil.Logger(t)
	cwd, err := os.Getwd()
	assert.Nil(t, err)
	hs := initCollector(logger, cwd)

	// Collect twice so we can calculate percents we need to generate some work
	// so that the cpu values change
	hs.collect()
	for begin := time.Now(); time.Now().Sub(begin) < 100*time.Millisecond; {
	}
	hs.collect()
	stats := hs.Stats()
	assert.NotZero(t, len(stats.CPU))

	for _, cpu := range stats.CPU {
		assert.False(t, math.IsNaN(cpu.Idle))
		assert.False(t, math.IsNaN(cpu.Total))
		assert.False(t, math.IsNaN(cpu.System))
		assert.False(t, math.IsNaN(cpu.User))

		assert.False(t, math.IsInf(cpu.Idle, 0))
		assert.False(t, math.IsInf(cpu.Total, 0))
		assert.False(t, math.IsInf(cpu.System, 0))
		assert.False(t, math.IsInf(cpu.User, 0))
	}
}

func TestCpuStatsCalculator_Nan(t *testing.T) {
	times := cpu.TimesStat{
		User:   0.0,
		Idle:   100.0,
		System: 0.0,
	}

	calculator := &cpuStatsCalculator{}
	calculator.calculate(times)
	stats := calculator.calculate(times)
	require.Equal(t, 100.0, stats.Idle)
	require.Zero(t, stats.User)
	require.Zero(t, stats.System)
	require.Zero(t, stats.Iowait)
	require.Zero(t, stats.Total)
}

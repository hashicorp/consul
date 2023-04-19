package hoststats

import (
	"math"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCpuStats_percent(t *testing.T) {
	cs := &cpuStats{
		totalCpus: runtime.NumCPU(),
	}
	cs.percent(79.7)
	time.Sleep(1 * time.Second)
	percent := cs.percent(80.69)
	expectedPercent := 98.00
	if percent < expectedPercent && percent > (expectedPercent+1.00) {
		t.Fatalf("expected: %v, actual: %v", expectedPercent, percent)
	}
}

func TestHostStats_CPU(t *testing.T) {

	assert := assert.New(t)

	logger := testutil.Logger(t)
	cwd, err := os.Getwd()
	assert.Nil(err)
	hs := initCollector(logger, cwd)

	// Collect twice so we can calculate percents we need to generate some work
	// so that the cpu values change
	hs.collect()
	total := 0
	for i := 1; i < 1000000000; i++ {
		total *= i
		total = total % i
	}
	hs.collect()
	stats := hs.Stats()
	assert.NotZero(len(stats.CPU))

	for _, cpu := range stats.CPU {
		assert.False(math.IsNaN(cpu.Idle))
		assert.False(math.IsNaN(cpu.Total))
		assert.False(math.IsNaN(cpu.System))
		assert.False(math.IsNaN(cpu.User))

		assert.False(math.IsInf(cpu.Idle, 0))
		assert.False(math.IsInf(cpu.Total, 0))
		assert.False(math.IsInf(cpu.System, 0))
		assert.False(math.IsInf(cpu.User, 0))
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
	idle, user, system, total := calculator.calculate(times)
	require.Equal(t, 100.0, idle)
	require.Zero(t, user)
	require.Zero(t, system)
	require.Zero(t, total)
}

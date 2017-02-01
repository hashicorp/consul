package cpu

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestCpu_times(t *testing.T) {
	v, err := Times(false)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(v) == 0 {
		t.Error("could not get CPUs ", err)
	}
	empty := TimesStat{}
	for _, vv := range v {
		if vv == empty {
			t.Errorf("could not get CPU User: %v", vv)
		}
	}
}

func TestCpu_counts(t *testing.T) {
	v, err := Counts(true)
	if err != nil {
		t.Errorf("error %v", err)
	}
	if v == 0 {
		t.Errorf("could not get CPU counts: %v", v)
	}
}

func TestCPUTimeStat_String(t *testing.T) {
	v := TimesStat{
		CPU:    "cpu0",
		User:   100.1,
		System: 200.1,
		Idle:   300.1,
	}
	e := `{"cpu":"cpu0","user":100.1,"system":200.1,"idle":300.1,"nice":0.0,"iowait":0.0,"irq":0.0,"softirq":0.0,"steal":0.0,"guest":0.0,"guestNice":0.0,"stolen":0.0}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("CPUTimesStat string is invalid: %v", v)
	}
}

func TestCpuInfo(t *testing.T) {
	v, err := Info()
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(v) == 0 {
		t.Errorf("could not get CPU Info")
	}
	for _, vv := range v {
		if vv.ModelName == "" {
			t.Errorf("could not get CPU Info: %v", vv)
		}
	}
}

func testCPUPercent(t *testing.T, percpu bool) {
	numcpu := runtime.NumCPU()
	testCount := 3

	if runtime.GOOS != "windows" {
		testCount = 100
		v, err := Percent(time.Millisecond, percpu)
		if err != nil {
			t.Errorf("error %v", err)
		}
		// Skip CircleCI which CPU num is different
		if os.Getenv("CIRCLECI") != "true" {
			if (percpu && len(v) != numcpu) || (!percpu && len(v) != 1) {
				t.Fatalf("wrong number of entries from CPUPercent: %v", v)
			}
		}
	}
	for i := 0; i < testCount; i++ {
		duration := time.Duration(10) * time.Microsecond
		v, err := Percent(duration, percpu)
		if err != nil {
			t.Errorf("error %v", err)
		}
		for _, percent := range v {
			// Check for slightly greater then 100% to account for any rounding issues.
			if percent < 0.0 || percent > 100.0001*float64(numcpu) {
				t.Fatalf("CPUPercent value is invalid: %f", percent)
			}
		}
	}
}

func testCPUPercentLastUsed(t *testing.T, percpu bool) {

	numcpu := runtime.NumCPU()
	testCount := 10

	if runtime.GOOS != "windows" {
		testCount = 2
		v, err := Percent(time.Millisecond, percpu)
		if err != nil {
			t.Errorf("error %v", err)
		}
		// Skip CircleCI which CPU num is different
		if os.Getenv("CIRCLECI") != "true" {
			if (percpu && len(v) != numcpu) || (!percpu && len(v) != 1) {
				t.Fatalf("wrong number of entries from CPUPercent: %v", v)
			}
		}
	}
	for i := 0; i < testCount; i++ {
		v, err := Percent(0, percpu)
		if err != nil {
			t.Errorf("error %v", err)
		}
		time.Sleep(1 * time.Millisecond)
		for _, percent := range v {
			// Check for slightly greater then 100% to account for any rounding issues.
			if percent < 0.0 || percent > 100.0001*float64(numcpu) {
				t.Fatalf("CPUPercent value is invalid: %f", percent)
			}
		}
	}

}

func TestCPUPercent(t *testing.T) {
	testCPUPercent(t, false)
}

func TestCPUPercentPerCpu(t *testing.T) {
	testCPUPercent(t, true)
}

func TestCPUPercentIntervalZero(t *testing.T) {
	testCPUPercentLastUsed(t, false)
}

func TestCPUPercentIntervalZeroPerCPU(t *testing.T) {
	testCPUPercentLastUsed(t, true)
}

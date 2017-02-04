// +build openbsd

package cpu

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/internal/common"
)

// sys/sched.h
const (
	CPUser    = 0
	CPNice    = 1
	CPSys     = 2
	CPIntr    = 3
	CPIdle    = 4
	CPUStates = 5
)

// sys/sysctl.h
const (
	CTLKern     = 1  // "high kernel": proc, limits
	KernCptime  = 40 // KERN_CPTIME
	KernCptime2 = 71 // KERN_CPTIME2
)

var ClocksPerSec = float64(128)

func init() {
	getconf, err := exec.LookPath("/usr/bin/getconf")
	if err != nil {
		return
	}
	out, err := invoke.Command(getconf, "CLK_TCK")
	// ignore errors
	if err == nil {
		i, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if err == nil {
			ClocksPerSec = float64(i)
		}
	}
}

func Times(percpu bool) ([]TimesStat, error) {
	var ret []TimesStat

	var ncpu int
	if percpu {
		ncpu, _ = Counts(true)
	} else {
		ncpu = 1
	}

	for i := 0; i < ncpu; i++ {
		var cpuTimes [CPUStates]int64
		var mib []int32
		if percpu {
			mib = []int32{CTLKern, KernCptime}
		} else {
			mib = []int32{CTLKern, KernCptime2, int32(i)}
		}
		buf, _, err := common.CallSyscall(mib)
		if err != nil {
			return ret, err
		}

		br := bytes.NewReader(buf)
		err = binary.Read(br, binary.LittleEndian, &cpuTimes)
		if err != nil {
			return ret, err
		}
		c := TimesStat{
			User:   float64(cpuTimes[CPUser]) / ClocksPerSec,
			Nice:   float64(cpuTimes[CPNice]) / ClocksPerSec,
			System: float64(cpuTimes[CPSys]) / ClocksPerSec,
			Idle:   float64(cpuTimes[CPIdle]) / ClocksPerSec,
			Irq:    float64(cpuTimes[CPIntr]) / ClocksPerSec,
		}
		if !percpu {
			c.CPU = "cpu-total"
		} else {
			c.CPU = fmt.Sprintf("cpu%d", i)
		}
		ret = append(ret, c)
	}

	return ret, nil
}

// Returns only one (minimal) CPUInfoStat on OpenBSD
func Info() ([]InfoStat, error) {
	var ret []InfoStat

	c := InfoStat{}

	v, err := syscall.Sysctl("hw.model")
	if err != nil {
		return nil, err
	}
	c.ModelName = v

	return append(ret, c), nil
}

package cpu

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/internal/common"
)

// sys/resource.h
const (
	CPUser    = 0
	CPNice    = 1
	CPSys     = 2
	CPIntr    = 3
	CPIdle    = 4
	CPUStates = 5
)

var ClocksPerSec = float64(128)
var cpuMatch = regexp.MustCompile(`^CPU:`)
var originMatch = regexp.MustCompile(`Origin\s*=\s*"(.+)"\s+Id\s*=\s*(.+)\s+Family\s*=\s*(.+)\s+Model\s*=\s*(.+)\s+Stepping\s*=\s*(.+)`)
var featuresMatch = regexp.MustCompile(`Features=.+<(.+)>`)
var featuresMatch2 = regexp.MustCompile(`Features2=[a-f\dx]+<(.+)>`)
var cpuEnd = regexp.MustCompile(`^Trying to mount root`)
var cpuCores = regexp.MustCompile(`FreeBSD/SMP: (\d*) package\(s\) x (\d*) core\(s\)`)

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

	var sysctlCall string
	var ncpu int
	if percpu {
		sysctlCall = "kern.cp_times"
		ncpu, _ = Counts(true)
	} else {
		sysctlCall = "kern.cp_time"
		ncpu = 1
	}

	cpuTimes, err := common.DoSysctrl(sysctlCall)
	if err != nil {
		return ret, err
	}

	for i := 0; i < ncpu; i++ {
		offset := CPUStates * i
		user, err := strconv.ParseFloat(cpuTimes[CPUser+offset], 64)
		if err != nil {
			return ret, err
		}
		nice, err := strconv.ParseFloat(cpuTimes[CPNice+offset], 64)
		if err != nil {
			return ret, err
		}
		sys, err := strconv.ParseFloat(cpuTimes[CPSys+offset], 64)
		if err != nil {
			return ret, err
		}
		idle, err := strconv.ParseFloat(cpuTimes[CPIdle+offset], 64)
		if err != nil {
			return ret, err
		}
		intr, err := strconv.ParseFloat(cpuTimes[CPIntr+offset], 64)
		if err != nil {
			return ret, err
		}

		c := TimesStat{
			User:   float64(user / ClocksPerSec),
			Nice:   float64(nice / ClocksPerSec),
			System: float64(sys / ClocksPerSec),
			Idle:   float64(idle / ClocksPerSec),
			Irq:    float64(intr / ClocksPerSec),
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

// Returns only one InfoStat on FreeBSD.  The information regarding core
// count, however is accurate and it is assumed that all InfoStat attributes
// are the same across CPUs.
func Info() ([]InfoStat, error) {
	const dmesgBoot = "/var/run/dmesg.boot"

	c, num, err := parseDmesgBoot(dmesgBoot)
	if err != nil {
		return nil, err
	}
	var vals []string
	if vals, err = common.DoSysctrl("hw.clockrate"); err != nil {
		return nil, err
	}
	if c.Mhz, err = strconv.ParseFloat(vals[0], 64); err != nil {
		return nil, fmt.Errorf("Unable to parse FreeBSD CPU clock rate: %v", err)
	}

	if vals, err = common.DoSysctrl("hw.ncpu"); err != nil {
		return nil, err
	}
	var i64 int64
	if i64, err = strconv.ParseInt(vals[0], 10, 32); err != nil {
		return nil, fmt.Errorf("Unable to parse FreeBSD cores: %v", err)
	}
	c.Cores = int32(i64)

	if vals, err = common.DoSysctrl("hw.model"); err != nil {
		return nil, err
	}
	c.ModelName = strings.Join(vals, " ")

	ret := make([]InfoStat, num)
	for i := 0; i < num; i++ {
		ret[i] = c
	}

	return ret, nil
}

func parseDmesgBoot(fileName string) (InfoStat, int, error) {
	c := InfoStat{}
	lines, _ := common.ReadLines(fileName)
	var cpuNum int
	for _, line := range lines {
		if matches := cpuEnd.FindStringSubmatch(line); matches != nil {
			break
		} else if matches := originMatch.FindStringSubmatch(line); matches != nil {
			c.VendorID = matches[1]
			c.Family = matches[3]
			c.Model = matches[4]
			t, err := strconv.ParseInt(matches[5], 10, 32)
			if err != nil {
				return c, 0, fmt.Errorf("Unable to parse FreeBSD CPU stepping information from %q: %v", line, err)
			}
			c.Stepping = int32(t)
		} else if matches := featuresMatch.FindStringSubmatch(line); matches != nil {
			for _, v := range strings.Split(matches[1], ",") {
				c.Flags = append(c.Flags, strings.ToLower(v))
			}
		} else if matches := featuresMatch2.FindStringSubmatch(line); matches != nil {
			for _, v := range strings.Split(matches[1], ",") {
				c.Flags = append(c.Flags, strings.ToLower(v))
			}
		} else if matches := cpuCores.FindStringSubmatch(line); matches != nil {
			t, err := strconv.ParseInt(matches[1], 10, 32)
			if err != nil {
				return c, 0, fmt.Errorf("Unable to parse FreeBSD CPU Nums from %q: %v", line, err)
			}
			cpuNum = int(t)
			t2, err := strconv.ParseInt(matches[2], 10, 32)
			if err != nil {
				return c, 0, fmt.Errorf("Unable to parse FreeBSD CPU cores from %q: %v", line, err)
			}
			c.Cores = int32(t2)
		}
	}

	return c, cpuNum, nil
}

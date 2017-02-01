package process

import (
	"fmt"
	"os"
	"os/user"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/internal/common"
	"github.com/stretchr/testify/assert"
)

var mu sync.Mutex

func testGetProcess() Process {
	checkPid := os.Getpid() // process.test
	ret, _ := NewProcess(int32(checkPid))
	return *ret
}

func Test_Pids(t *testing.T) {
	ret, err := Pids()
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(ret) == 0 {
		t.Errorf("could not get pids %v", ret)
	}
}

func Test_Pids_Fail(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}

	mu.Lock()
	defer mu.Unlock()

	invoke = common.FakeInvoke{Suffix: "fail"}
	ret, err := Pids()
	invoke = common.Invoke{}
	if err != nil {
		t.Errorf("error %v", err)
	}
	if len(ret) != 9 {
		t.Errorf("wrong getted pid nums: %v/%d", ret, len(ret))
	}
}
func Test_Pid_exists(t *testing.T) {
	checkPid := os.Getpid()

	ret, err := PidExists(int32(checkPid))
	if err != nil {
		t.Errorf("error %v", err)
	}

	if ret == false {
		t.Errorf("could not get process exists: %v", ret)
	}
}

func Test_NewProcess(t *testing.T) {
	checkPid := os.Getpid()

	ret, err := NewProcess(int32(checkPid))
	if err != nil {
		t.Errorf("error %v", err)
	}
	empty := &Process{}
	if runtime.GOOS != "windows" { // Windows pid is 0
		if empty == ret {
			t.Errorf("error %v", ret)
		}
	}

}

func Test_Process_memory_maps(t *testing.T) {
	checkPid := os.Getpid()

	ret, err := NewProcess(int32(checkPid))

	mmaps, err := ret.MemoryMaps(false)
	if err != nil {
		t.Errorf("memory map get error %v", err)
	}
	empty := MemoryMapsStat{}
	for _, m := range *mmaps {
		if m == empty {
			t.Errorf("memory map get error %v", m)
		}
	}
}
func Test_Process_MemoryInfo(t *testing.T) {
	p := testGetProcess()

	v, err := p.MemoryInfo()
	if err != nil {
		t.Errorf("geting memory info error %v", err)
	}
	empty := MemoryInfoStat{}
	if v == nil || *v == empty {
		t.Errorf("could not get memory info %v", v)
	}
}

func Test_Process_CmdLine(t *testing.T) {
	p := testGetProcess()

	v, err := p.Cmdline()
	if err != nil {
		t.Errorf("geting cmdline error %v", err)
	}
	if !strings.Contains(v, "process.test") {
		t.Errorf("invalid cmd line %v", v)
	}
}

func Test_Process_CmdLineSlice(t *testing.T) {
	p := testGetProcess()

	v, err := p.CmdlineSlice()
	if err != nil {
		t.Fatalf("geting cmdline slice error %v", err)
	}
	if !reflect.DeepEqual(v, os.Args) {
		t.Errorf("returned cmdline slice not as expected:\nexp: %v\ngot: %v", os.Args, v)
	}
}

func Test_Process_Ppid(t *testing.T) {
	p := testGetProcess()

	v, err := p.Ppid()
	if err != nil {
		t.Errorf("geting ppid error %v", err)
	}
	if v == 0 {
		t.Errorf("return value is 0 %v", v)
	}
}

func Test_Process_Status(t *testing.T) {
	p := testGetProcess()

	v, err := p.Status()
	if err != nil {
		t.Errorf("geting status error %v", err)
	}
	if v != "R" && v != "S" {
		t.Errorf("could not get state %v", v)
	}
}

func Test_Process_Terminal(t *testing.T) {
	p := testGetProcess()

	_, err := p.Terminal()
	if err != nil {
		t.Errorf("geting terminal error %v", err)
	}

	/*
		if v == "" {
			t.Errorf("could not get terminal %v", v)
		}
	*/
}

func Test_Process_IOCounters(t *testing.T) {
	p := testGetProcess()

	v, err := p.IOCounters()
	if err != nil {
		t.Errorf("geting iocounter error %v", err)
		return
	}
	empty := &IOCountersStat{}
	if v == empty {
		t.Errorf("error %v", v)
	}
}

func Test_Process_NumCtx(t *testing.T) {
	p := testGetProcess()

	_, err := p.NumCtxSwitches()
	if err != nil {
		t.Errorf("geting numctx error %v", err)
		return
	}
}

func Test_Process_Nice(t *testing.T) {
	p := testGetProcess()

	n, err := p.Nice()
	if err != nil {
		t.Errorf("geting nice error %v", err)
	}
	if n != 0 && n != 20 && n != 8 {
		t.Errorf("invalid nice: %d", n)
	}
}
func Test_Process_NumThread(t *testing.T) {
	p := testGetProcess()

	n, err := p.NumThreads()
	if err != nil {
		t.Errorf("geting NumThread error %v", err)
	}
	if n < 0 {
		t.Errorf("invalid NumThread: %d", n)
	}
}

func Test_Process_Name(t *testing.T) {
	p := testGetProcess()

	n, err := p.Name()
	if err != nil {
		t.Errorf("geting name error %v", err)
	}
	if !strings.Contains(n, "process.test") {
		t.Errorf("invalid Exe %s", n)
	}
}
func Test_Process_Exe(t *testing.T) {
	p := testGetProcess()

	n, err := p.Exe()
	if err != nil {
		t.Errorf("geting Exe error %v", err)
	}
	if !strings.Contains(n, "process.test") {
		t.Errorf("invalid Exe %s", n)
	}
}

func Test_Process_CpuPercent(t *testing.T) {
	p := testGetProcess()
	percent, err := p.Percent(0)
	if err != nil {
		t.Errorf("error %v", err)
	}
	duration := time.Duration(1000) * time.Microsecond
	time.Sleep(duration)
	percent, err = p.Percent(0)
	if err != nil {
		t.Errorf("error %v", err)
	}

	numcpu := runtime.NumCPU()
	//	if percent < 0.0 || percent > 100.0*float64(numcpu) { // TODO
	if percent < 0.0 {
		t.Fatalf("CPUPercent value is invalid: %f, %d", percent, numcpu)
	}
}

func Test_Process_CpuPercentLoop(t *testing.T) {
	p := testGetProcess()
	numcpu := runtime.NumCPU()

	for i := 0; i < 2; i++ {
		duration := time.Duration(100) * time.Microsecond
		percent, err := p.Percent(duration)
		if err != nil {
			t.Errorf("error %v", err)
		}
		//	if percent < 0.0 || percent > 100.0*float64(numcpu) { // TODO
		if percent < 0.0 {
			t.Fatalf("CPUPercent value is invalid: %f, %d", percent, numcpu)
		}
	}
}

func Test_Process_CreateTime(t *testing.T) {
	p := testGetProcess()

	c, err := p.CreateTime()
	if err != nil {
		t.Errorf("error %v", err)
	}

	if c < 1420000000 {
		t.Errorf("process created time is wrong.")
	}

	gotElapsed := time.Since(time.Unix(int64(c/1000), 0))
	maxElapsed := time.Duration(5 * time.Second)

	if gotElapsed >= maxElapsed {
		t.Errorf("this process has not been running for %v", gotElapsed)
	}
}

func Test_Parent(t *testing.T) {
	p := testGetProcess()

	c, err := p.Parent()
	if err != nil {
		t.Fatalf("error %v", err)
	}
	if c == nil {
		t.Fatalf("could not get parent")
	}
	if c.Pid == 0 {
		t.Fatalf("wrong parent pid")
	}
}

func Test_Connections(t *testing.T) {
	p := testGetProcess()

	c, err := p.Connections()
	if err != nil {
		t.Fatalf("error %v", err)
	}
	// TODO:
	// Since go test open no conneciton, ret is empty.
	// should invoke child process or other solutions.
	if len(c) != 0 {
		t.Fatalf("wrong connections")
	}
}

func Test_Children(t *testing.T) {
	p, err := NewProcess(1)
	if err != nil {
		t.Fatalf("new process error %v", err)
	}

	c, err := p.Children()
	if err != nil {
		t.Fatalf("error %v", err)
	}
	if len(c) == 0 {
		t.Fatalf("children is empty")
	}
}

func Test_Username(t *testing.T) {
	myPid := os.Getpid()
	currentUser, _ := user.Current()
	myUsername := currentUser.Username

	process, _ := NewProcess(int32(myPid))
	pidUsername, _ := process.Username()
	assert.Equal(t, myUsername, pidUsername)
}

func Test_CPUTimes(t *testing.T) {
	pid := os.Getpid()
	process, err := NewProcess(int32(pid))
	assert.Nil(t, err)

	spinSeconds := 0.2
	cpuTimes0, err := process.Times()
	assert.Nil(t, err)

	// Spin for a duration of spinSeconds
	t0 := time.Now()
	tGoal := t0.Add(time.Duration(spinSeconds*1000) * time.Millisecond)
	assert.Nil(t, err)
	for time.Now().Before(tGoal) {
		// This block intentionally left blank
	}

	cpuTimes1, err := process.Times()
	assert.Nil(t, err)

	if cpuTimes0 == nil || cpuTimes1 == nil {
		t.FailNow()
	}
	measuredElapsed := cpuTimes1.Total() - cpuTimes0.Total()
	message := fmt.Sprintf("Measured %fs != spun time of %fs\ncpuTimes0=%v\ncpuTimes1=%v",
		measuredElapsed, spinSeconds, cpuTimes0, cpuTimes1)
	assert.True(t, measuredElapsed > float64(spinSeconds)/5, message)
	assert.True(t, measuredElapsed < float64(spinSeconds)*5, message)
}

func Test_OpenFiles(t *testing.T) {
	pid := os.Getpid()
	p, err := NewProcess(int32(pid))
	assert.Nil(t, err)

	v, err := p.OpenFiles()
	assert.Nil(t, err)
	assert.NotEmpty(t, v) // test always open files.

	for _, vv := range v {
		assert.NotEqual(t, "", vv.Path)
	}

}

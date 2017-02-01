// +build openbsd

package process

import (
	"bytes"
	"C"
	"encoding/binary"
	"strings"
	"syscall"
	"unsafe"

	cpu "github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/internal/common"
	net "github.com/shirou/gopsutil/net"
	mem "github.com/shirou/gopsutil/mem"
)

// MemoryInfoExStat is different between OSes
type MemoryInfoExStat struct {
}

type MemoryMapsStat struct {
}

func Pids() ([]int32, error) {
	var ret []int32
	procs, err := processes()
	if err != nil {
		return ret, nil
	}

	for _, p := range procs {
		ret = append(ret, p.Pid)
	}

	return ret, nil
}

func (p *Process) Ppid() (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}

	return k.Ppid, nil
}
func (p *Process) Name() (string, error) {
	k, err := p.getKProc()
	if err != nil {
		return "", err
	}

	return common.IntToString(k.Comm[:]), nil
}
func (p *Process) Exe() (string, error) {
	return "", common.ErrNotImplementedError
}

func (p *Process) CmdlineSlice() ([]string, error) {
	mib := []int32{CTLKern, KernProcArgs, p.Pid, KernProcArgv }
	buf, _, err := common.CallSyscall(mib)

	if err != nil {
		return nil, err
	}

	argc := 0
	argvp := unsafe.Pointer(&buf[0])
	argv := *(**C.char)(unsafe.Pointer(argvp))
	size := unsafe.Sizeof(argv)
	var strParts []string

	for argv != nil {
		strParts = append(strParts, C.GoString(argv))

		argc++
		argv = *(**C.char)(unsafe.Pointer(uintptr(argvp) + uintptr(argc) * size))
	}
	return strParts, nil
}

func (p *Process) Cmdline() (string, error) {
	argv, err := p.CmdlineSlice()
	if err != nil {
		return "", err
	}
	return strings.Join(argv, " "), nil
}

func (p *Process) CreateTime() (int64, error) {
	return 0, common.ErrNotImplementedError
}
func (p *Process) Cwd() (string, error) {
	return "", common.ErrNotImplementedError
}
func (p *Process) Parent() (*Process, error) {
	return p, common.ErrNotImplementedError
}
func (p *Process) Status() (string, error) {
	k, err := p.getKProc()
	if err != nil {
		return "", err
	}
	var s string
	switch k.Stat {
	case SIDL:
	case SRUN:
	case SONPROC:
		s = "R"
	case SSLEEP:
		s = "S"
	case SSTOP:
		s = "T"
	case SDEAD:
		s = "Z"
	}

	return s, nil
}
func (p *Process) Uids() ([]int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	uids := make([]int32, 0, 3)

	uids = append(uids, int32(k.Ruid), int32(k.Uid), int32(k.Svuid))

	return uids, nil
}
func (p *Process) Gids() ([]int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	gids := make([]int32, 0, 3)
	gids = append(gids, int32(k.Rgid), int32(k.Ngroups), int32(k.Svgid))

	return gids, nil
}
func (p *Process) Terminal() (string, error) {
	k, err := p.getKProc()
	if err != nil {
		return "", err
	}

	ttyNr := uint64(k.Tdev)

	termmap, err := getTerminalMap()
	if err != nil {
		return "", err
	}

	return termmap[ttyNr], nil
}
func (p *Process) Nice() (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}
	return int32(k.Nice), nil
}
func (p *Process) IOnice() (int32, error) {
	return 0, common.ErrNotImplementedError
}
func (p *Process) Rlimit() ([]RlimitStat, error) {
	var rlimit []RlimitStat
	return rlimit, common.ErrNotImplementedError
}
func (p *Process) IOCounters() (*IOCountersStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	return &IOCountersStat{
		ReadCount:  uint64(k.Uru_inblock),
		WriteCount: uint64(k.Uru_oublock),
	}, nil
}
func (p *Process) NumCtxSwitches() (*NumCtxSwitchesStat, error) {
	return nil, common.ErrNotImplementedError
}
func (p *Process) NumFDs() (int32, error) {
	return 0, common.ErrNotImplementedError
}
func (p *Process) NumThreads() (int32, error) {
	/* not supported, just return 1 */
	return 1, nil
}
func (p *Process) Threads() (map[string]string, error) {
	ret := make(map[string]string, 0)
	return ret, common.ErrNotImplementedError
}
func (p *Process) Times() (*cpu.TimesStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	return &cpu.TimesStat{
		CPU:    "cpu",
		User:   float64(k.Uutime_sec) + float64(k.Uutime_usec)/1000000,
		System: float64(k.Ustime_sec) + float64(k.Ustime_usec)/1000000,
	}, nil
}
func (p *Process) CPUAffinity() ([]int32, error) {
	return nil, common.ErrNotImplementedError
}
func (p *Process) MemoryInfo() (*MemoryInfoStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	pageSize, err := mem.GetPageSize()
	if err != nil {
		return nil, err
	}

	return &MemoryInfoStat{
		RSS: uint64(k.Vm_rssize) * pageSize,
		VMS: uint64(k.Vm_tsize) + uint64(k.Vm_dsize) +
			uint64(k.Vm_ssize),
	}, nil
}
func (p *Process) MemoryInfoEx() (*MemoryInfoExStat, error) {
	return nil, common.ErrNotImplementedError
}

func (p *Process) Children() ([]*Process, error) {
	pids, err := common.CallPgrep(invoke, p.Pid)
	if err != nil {
		return nil, err
	}
	ret := make([]*Process, 0, len(pids))
	for _, pid := range pids {
		np, err := NewProcess(pid)
		if err != nil {
			return nil, err
		}
		ret = append(ret, np)
	}
	return ret, nil
}

func (p *Process) OpenFiles() ([]OpenFilesStat, error) {
	return nil, common.ErrNotImplementedError
}

func (p *Process) Connections() ([]net.ConnectionStat, error) {
	return nil, common.ErrNotImplementedError
}

func (p *Process) NetIOCounters(pernic bool) ([]net.IOCountersStat, error) {
	return nil, common.ErrNotImplementedError
}

func (p *Process) IsRunning() (bool, error) {
	return true, common.ErrNotImplementedError
}
func (p *Process) MemoryMaps(grouped bool) (*[]MemoryMapsStat, error) {
	var ret []MemoryMapsStat
	return &ret, common.ErrNotImplementedError
}

func processes() ([]Process, error) {
	results := make([]Process, 0, 50)

	buf, length, err := CallKernProcSyscall(KernProcAll, 0)

	if err != nil {
		return results, err
	}

	// get kinfo_proc size
	count := int(length / uint64(sizeOfKinfoProc))

	// parse buf to procs
	for i := 0; i < count; i++ {
		b := buf[i*sizeOfKinfoProc : (i+1)*sizeOfKinfoProc]
		k, err := parseKinfoProc(b)
		if err != nil {
			continue
		}
		p, err := NewProcess(int32(k.Pid))
		if err != nil {
			continue
		}

		results = append(results, *p)
	}

	return results, nil
}

func parseKinfoProc(buf []byte) (KinfoProc, error) {
	var k KinfoProc
	br := bytes.NewReader(buf)
	err := common.Read(br, binary.LittleEndian, &k)
	return k, err
}

func (p *Process) getKProc() (*KinfoProc, error) {
	buf, length, err := CallKernProcSyscall(KernProcPID, p.Pid)
	if err != nil {
		return nil, err
	}
	if length != sizeOfKinfoProc {
		return nil, err
	}

	k, err := parseKinfoProc(buf)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func NewProcess(pid int32) (*Process, error) {
	p := &Process{Pid: pid}

	return p, nil
}

func CallKernProcSyscall(op int32, arg int32) ([]byte, uint64, error) {
	mib := []int32{CTLKern, KernProc, op, arg, sizeOfKinfoProc, 0}
	mibptr := unsafe.Pointer(&mib[0])
	miblen := uint64(len(mib))
	length := uint64(0)
	_, _, err := syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(mibptr),
		uintptr(miblen),
		0,
		uintptr(unsafe.Pointer(&length)),
		0,
		0)
	if err != 0 {
		return nil, length, err
	}

	count := int32(length / uint64(sizeOfKinfoProc))
	mib = []int32{CTLKern, KernProc, op, arg, sizeOfKinfoProc, count}
	mibptr = unsafe.Pointer(&mib[0])
	miblen = uint64(len(mib))
	// get proc info itself
	buf := make([]byte, length)
	_, _, err = syscall.Syscall6(
		syscall.SYS___SYSCTL,
		uintptr(mibptr),
		uintptr(miblen),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&length)),
		0,
		0)
	if err != 0 {
		return buf, length, err
	}

	return buf, length, nil
}

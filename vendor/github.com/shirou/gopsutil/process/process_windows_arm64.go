//go:build windows && arm64
// +build windows,arm64

package process

import (
	"errors"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/internal/common"
)

type PROCESS_MEMORY_COUNTERS struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uint32
	WorkingSetSize             uint32
	QuotaPeakPagedPoolUsage    uint32
	QuotaPagedPoolUsage        uint32
	QuotaPeakNonPagedPoolUsage uint32
	QuotaNonPagedPoolUsage     uint32
	PagefileUsage              uint32
	PeakPagefileUsage          uint32
}

func queryPebAddress(procHandle syscall.Handle, is32BitProcess bool) (uint64, error) {
	if is32BitProcess {
		//we are on a 32-bit process reading an external 32-bit process
		var info processBasicInformation32

		ret, _, _ := common.ProcNtQueryInformationProcess.Call(
			uintptr(procHandle),
			uintptr(common.ProcessBasicInformation),
			uintptr(unsafe.Pointer(&info)),
			uintptr(unsafe.Sizeof(info)),
			uintptr(0),
		)
		if int(ret) >= 0 {
			return uint64(info.PebBaseAddress), nil
		}
	} else {
		//we are on a 32-bit process reading an external 64-bit process
		if common.ProcNtWow64QueryInformationProcess64.Find() == nil { //avoid panic
			var info processBasicInformation64

			ret, _, _ := common.ProcNtWow64QueryInformationProcess64.Call(
				uintptr(procHandle),
				uintptr(common.ProcessBasicInformation),
				uintptr(unsafe.Pointer(&info)),
				uintptr(unsafe.Sizeof(info)),
				uintptr(0),
			)
			if int(ret) >= 0 {
				return info.PebBaseAddress, nil
			}
		}
	}

	//return 0 on error
	return 0, errors.New("could not query PEB address")
}

func readProcessMemory(h syscall.Handle, is32BitProcess bool, address uint64, size uint) []byte {
	if is32BitProcess {
		var read uint

		buffer := make([]byte, size)

		ret, _, _ := common.ProcNtReadVirtualMemory.Call(
			uintptr(h),
			uintptr(address),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(size),
			uintptr(unsafe.Pointer(&read)),
		)
		if int(ret) >= 0 && read > 0 {
			return buffer[:read]
		}
	} else {
		//reading a 64-bit process from a 32-bit one
		if common.ProcNtWow64ReadVirtualMemory64.Find() == nil { //avoid panic
			var read uint64

			buffer := make([]byte, size)

			ret, _, _ := common.ProcNtWow64ReadVirtualMemory64.Call(
				uintptr(h),
				uintptr(address&0xFFFFFFFF), //the call expects a 64-bit value
				uintptr(address>>32),
				uintptr(unsafe.Pointer(&buffer[0])),
				uintptr(size), //the call expects a 64-bit value
				uintptr(0),    //but size is 32-bit so pass zero as the high dword
				uintptr(unsafe.Pointer(&read)),
			)
			if int(ret) >= 0 && read > 0 {
				return buffer[:uint(read)]
			}
		}
	}

	//if we reach here, an error happened
	return nil
}

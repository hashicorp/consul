// +build windows

package host

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/StackExchange/wmi"

	"github.com/shirou/gopsutil/internal/common"
	process "github.com/shirou/gopsutil/process"
)

var (
	procGetSystemTimeAsFileTime = common.Modkernel32.NewProc("GetSystemTimeAsFileTime")
	osInfo                      *Win32_OperatingSystem
)

type Win32_OperatingSystem struct {
	Version        string
	Caption        string
	ProductType    uint32
	BuildNumber    string
	LastBootUpTime time.Time
}

func Info() (*InfoStat, error) {
	ret := &InfoStat{
		OS: runtime.GOOS,
	}

	{
		hostname, err := os.Hostname()
		if err == nil {
			ret.Hostname = hostname
		}
	}

	{
		platform, family, version, err := PlatformInformation()
		if err == nil {
			ret.Platform = platform
			ret.PlatformFamily = family
			ret.PlatformVersion = version
		} else {
			return ret, err
		}
	}

	{
		boot, err := BootTime()
		if err == nil {
			ret.BootTime = boot
			ret.Uptime, _ = Uptime()
		}
	}

	{
		hostID, err := getMachineGuid()
		if err == nil {
			ret.HostID = hostID
		}
	}

	{
		procs, err := process.Pids()
		if err == nil {
			ret.Procs = uint64(len(procs))
		}
	}

	return ret, nil
}

func getMachineGuid() (string, error) {
	var h syscall.Handle
	err := syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, syscall.StringToUTF16Ptr(`SOFTWARE\Microsoft\Cryptography`), 0, syscall.KEY_READ, &h)
	if err != nil {
		return "", err
	}
	defer syscall.RegCloseKey(h)

	const windowsRegBufLen = 74 // len(`{`) + len(`abcdefgh-1234-456789012-123345456671` * 2) + len(`}`) // 2 == bytes/UTF16
	const uuidLen = 36

	var regBuf [windowsRegBufLen]uint16
	bufLen := uint32(windowsRegBufLen)
	var valType uint32
	err = syscall.RegQueryValueEx(h, syscall.StringToUTF16Ptr(`MachineGuid`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
	if err != nil {
		return "", err
	}

	hostID := syscall.UTF16ToString(regBuf[:])
	hostIDLen := len(hostID)
	if hostIDLen != uuidLen {
		return "", fmt.Errorf("HostID incorrect: %q\n", hostID)
	}

	return hostID, nil
}

func GetOSInfo() (Win32_OperatingSystem, error) {
	var dst []Win32_OperatingSystem
	q := wmi.CreateQuery(&dst, "")
	err := wmi.Query(q, &dst)
	if err != nil {
		return Win32_OperatingSystem{}, err
	}

	osInfo = &dst[0]

	return dst[0], nil
}

func Uptime() (uint64, error) {
	if osInfo == nil {
		_, err := GetOSInfo()
		if err != nil {
			return 0, err
		}
	}
	now := time.Now()
	t := osInfo.LastBootUpTime.Local()
	return uint64(now.Sub(t).Seconds()), nil
}

func bootTime(up uint64) uint64 {
	return uint64(time.Now().Unix()) - up
}

func BootTime() (uint64, error) {
	if cachedBootTime != 0 {
		return cachedBootTime, nil
	}
	up, err := Uptime()
	if err != nil {
		return 0, err
	}
	cachedBootTime = bootTime(up)
	return cachedBootTime, nil
}

func PlatformInformation() (platform string, family string, version string, err error) {
	if osInfo == nil {
		_, err = GetOSInfo()
		if err != nil {
			return
		}
	}

	// Platform
	platform = strings.Trim(osInfo.Caption, " ")

	// PlatformFamily
	switch osInfo.ProductType {
	case 1:
		family = "Standalone Workstation"
	case 2:
		family = "Server (Domain Controller)"
	case 3:
		family = "Server"
	}

	// Platform Version
	version = fmt.Sprintf("%s Build %s", osInfo.Version, osInfo.BuildNumber)

	return
}

func Users() ([]UserStat, error) {
	var ret []UserStat

	return ret, nil
}

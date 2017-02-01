// +build linux

package common

import (
	"os"
	"os/exec"
	"strings"
)

func DoSysctrl(mib string) ([]string, error) {
	err := os.Setenv("LC_ALL", "C")
	if err != nil {
		return []string{}, err
	}
	sysctl, err := exec.LookPath("/sbin/sysctl")
	if err != nil {
		return []string{}, err
	}
	out, err := exec.Command(sysctl, "-n", mib).Output()
	if err != nil {
		return []string{}, err
	}
	v := strings.Replace(string(out), "{ ", "", 1)
	v = strings.Replace(string(v), " }", "", 1)
	values := strings.Fields(string(v))

	return values, nil
}

func NumProcs() (uint64, error) {
	f, err := os.Open(HostProc())
	if err != nil {
		return 0, err
	}

	list, err := f.Readdir(-1)
	defer f.Close()
	if err != nil {
		return 0, err
	}
	return uint64(len(list)), err
}

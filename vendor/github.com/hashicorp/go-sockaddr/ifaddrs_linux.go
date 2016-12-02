package sockaddr

import (
	"errors"
	"os/exec"
)

// defaultLinuxIfNameCmd is the comamnd to run on Linux to get the default
// interface.
func defaultLinuxIfNameCmd() []string {
	return []string{"/sbin/ip", "route"}
}

// getDefaultIfName is a Linux-specific function for extracting the name of the
// interface from ip(8).
func getDefaultIfName() (string, error) {
	var cmd []string = defaultLinuxIfNameCmd()
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return "", err
	}

	var ifName string
	if ifName, err = parseDefaultIfNameFromIPCmd(string(out)); err != nil {
		return "", errors.New("No default interface found")
	}
	return ifName, nil
}

package sockaddr

import (
	"errors"
	"os/exec"
)

// defaultSolarisIfNameCmd is the comamnd to run on Solaris to get the default
// interface
func defaultSolarisIfNameCmd() []string {
	return []string{"/usr/sbin/route", "-n", "get", "default"}
}

// getDefaultIfName is an Solaris-specific function for extracting the name of
// the interface from route(8).
func getDefaultIfName() (string, error) {
	var cmd []string = defaultSolarisIfNameCmd()
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return "", err
	}

	var ifName string
	if ifName, err = parseDefaultIfNameFromRoute(string(out)); err != nil {
		return "", errors.New("No default interface found")
	}
	return ifName, nil
}

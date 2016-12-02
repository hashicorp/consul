// +build darwin dragonfly freebsd netbsd openbsd

package sockaddr

import (
	"errors"
	"os/exec"
)

// defaultBSDIfNameCmd is the comamnd to run on BSDs to get the default
// interface
func defaultBSDIfNameCmd() []string {
	return []string{"/sbin/route", "-n", "get", "default"}
}

// getDefaultIfName is a *BSD-specific function for extracting the name of the
// interface from route(8).
func getDefaultIfName() (string, error) {
	var cmd []string = defaultBSDIfNameCmd()
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

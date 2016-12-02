package sockaddr

import (
	"errors"
	"os/exec"
)

// defaultWindowsIfNameCmd is the comamnd to run on Windows to get the default
// interface.
func defaultWindowsIfNameCmd() []string {
	return []string{"netstat", "-rn"}
}

// defaultWindowsIfNameCmd is the comamnd to run on Windows to get the default
// interface.
func defaultWindowsIPConfigCmd() []string {
	return []string{"ipconfig"}
}

// getDefaultIfName is a Windows-specific function for extracting the name of
// the interface from `netstat -rn` and `ipconfig`.
func getDefaultIfName() (string, error) {
	ipAddr, err := getWindowsIPOnDefaultRoute()
	if err != nil {
		return "", err
	}

}

func getWindowsIPOnDefaultRoute() (string, error) {
	var cmd []string = defaultWindowsIfNameCmd()
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return "", err
	}

	var defaultIPAddr string
	if defaultIPAddr, err = parseDefaultIfNameFromWindowsNetstatRN(string(out)); err != nil {
		return "", errors.New("No IP on default route found")
	}
	return defaultIPAddr, nil
}

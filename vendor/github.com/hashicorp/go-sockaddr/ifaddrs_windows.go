package sockaddr

import "os/exec"

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
	var cmd []string = defaultWindowsIfNameCmd()
	ifNameOut, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return "", err
	}

	cmd = defaultWindowsIPConfigCmd()
	ipconfigOut, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return "", err
	}

	ifName, err := parseDefaultIfNameWindows(string(ifNameOut), string(ipconfigOut))
	if err != nil {
		return "", err
	}

	return ifName, nil
}

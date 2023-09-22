package envoy

import (
	"errors"
	"os/exec"
	"regexp"
)

const (
	envoyVersionFlag = "--version"
)

// execCommand lets us mock out the exec.Command function
var execCommand = exec.Command

func execEnvoyVersion(binary string) (string, error) {
	cmd := execCommand(binary, envoyVersionFlag)

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	version, err := parseEnvoyVersionNumber(string(output))
	if err != nil {
		return "", err
	}
	return version, nil
}

func parseEnvoyVersionNumber(fullVersion string) (string, error) {
	// Use a regular expression to match the major.minor.patch version string in the fullVersion
	// Example input:
	// `envoy  version: 69958e4fe32da561376d8b1d367b5e6942dfba24/1.24.1/Distribution/RELEASE/BoringSSL`
	re := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(fullVersion)

	// If no matches were found, return an error
	if len(matches) == 0 {
		return "", errors.New("unable to parse Envoy version from output")
	}

	// Return the first match (the major.minor.patch version string)
	return matches[0], nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdscommon

import (
	_ "embed"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// File containing the canonical range of supported Envoy versions for this version of Consul.
// This file should contain exactly one point release for each major release of Envoy, per line.
// All other contents must be blank lines or comments. Comments must be on their own line starting with '#'.
//
//go:embed ENVOY_VERSIONS
var envoyVersionsRaw string

// initEnvoyVersions calls parseEnvoyVersions and panics if it returns an error. Used to set EnvoyVersions.
func initEnvoyVersions() []string {
	versions, err := parseEnvoyVersions(envoyVersionsRaw)
	if err != nil {
		panic(err)
	}
	return versions
}

// parseEnvoyVersions parses the ENVOY_VERSIONS file and returns a list of supported Envoy versions.
func parseEnvoyVersions(raw string) ([]string, error) {
	lines := strings.Split(raw, "\n")
	var versionLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue // skip empty lines and comments
		}

		// Assume all remaining lines are valid Envoy versions in the format "X.Y.Z".
		versionParts := strings.Split(trimmed, ".")
		if len(versionParts) != 3 {
			return nil, fmt.Errorf("invalid version in ENVOY_VERSIONS: %s", line)
		}
		for _, v := range versionParts {
			if _, err := strconv.Atoi(v); err != nil {
				return nil, fmt.Errorf("invalid version in ENVOY_VERSIONS: %s", line)
			}
		}
		versionLines = append(versionLines, trimmed)
	}

	// Ensure sorted in descending order.
	// We do this here as well as tests because other code (e.g. Makefile) may depend on the order
	// of these values, so we want early detection in case tests are not run before compilation.
	if !slices.IsSortedFunc(versionLines, func(v1, v2 string) int {
		return strings.Compare(v2, v1)
	}) {
		return nil, fmt.Errorf("ENVOY_VERSIONS must be sorted in descending order")
	}

	return versionLines, nil
}

// EnvoyVersions lists the latest officially supported versions of envoy.
//
// This list must be sorted by semver descending. Only one point release for
// each major release should be present.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var EnvoyVersions = initEnvoyVersions()

// UnsupportedEnvoyVersions lists any unsupported Envoy versions (mainly minor versions) that fall
// within the range of EnvoyVersions above.
// For example, if patch 1.21.3 (patch 3) had a breaking change, and was not supported
// even though 1.21 is a supported major release, you would then add 1.21.3 to this list.
// This list will be empty in most cases.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var UnsupportedEnvoyVersions = []string{}

// GetMaxEnvoyMajorVersion grabs the first value in EnvoyVersions and strips the last number off in order
// to return the maximum supported Envoy "major" version.
// For example, if the input string is "1.14.1", the function would return "1.14".
func GetMaxEnvoyMajorVersion() string {
	s := strings.Split(getMaxEnvoyVersion(), ".")
	return s[0] + "." + s[1]
}

// GetMinEnvoyMajorVersion grabs the last value in EnvoyVersions and strips the patch number off in order
// to return the minimum supported Envoy "major" version.
// For example, if the input string is "1.12.1", the function would return "1.12".
func GetMinEnvoyMajorVersion() string {
	s := strings.Split(getMinEnvoyVersion(), ".")
	return s[0] + "." + s[1]
}

// getMaxEnvoyVersion returns the first (highest) value in EnvoyVersions.
func getMaxEnvoyVersion() string {
	return EnvoyVersions[0]
}

// getMinEnvoyVersion returns the last (lowest) value in EnvoyVersions.
func getMinEnvoyVersion() string {
	return EnvoyVersions[len(EnvoyVersions)-1]
}

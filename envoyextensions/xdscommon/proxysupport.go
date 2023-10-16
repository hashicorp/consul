// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdscommon

import "strings"

// EnvoyVersions lists the latest officially supported versions of envoy.
//
// This list must be sorted by semver descending. Only one point release for
// each major release should be present.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var EnvoyVersions = []string{
	"1.27.0",
	"1.26.4",
	"1.25.9",
	"1.24.10",
}

// UnsupportedEnvoyVersions lists any unsupported Envoy versions (mainly minor versions) that fall
// within the range of EnvoyVersions above.
// For example, if patch 1.21.3 (patch 3) had a breaking change, and was not supported
// even though 1.21 is a supported major release, you would then add 1.21.3 to this list.
// This list will be empty in most cases.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var UnsupportedEnvoyVersions = []string{}

// GetMaxEnvoyMinorVersion grabs the first value in EnvoyVersions and strips the patch number off in order
// to return the maximum supported Envoy minor version
// For example, if the input string is "1.14.1", the function would return "1.14".
func GetMaxEnvoyMinorVersion() string {
	s := strings.Split(EnvoyVersions[0], ".")
	return s[0] + "." + s[1]
}

// GetMinEnvoyMinorVersion grabs the last value in EnvoyVersions and strips the patch number off in order
// to return the minimum supported Envoy minor version
// For example, if the input string is "1.12.1", the function would return "1.12".
func GetMinEnvoyMinorVersion() string {
	s := strings.Split(EnvoyVersions[len(EnvoyVersions)-1], ".")
	return s[0] + "." + s[1]
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package version

import (
	_ "embed"
	"fmt"
	"strings"
)

var (
	// The git commit that was compiled. These will be filled in by the
	// compiler.
	GitCommit string

	// The next version number that will be released. This will be updated after every release
	// Version must conform to the format expected by github.com/hashicorp/go-version
	// for tests to work.
	// A pre-release marker for the version can also be specified (e.g -dev). If this is omitted
	// then it means that it is a final release. Otherwise, this is a pre-release
	// such as "dev" (in development), "beta", "rc1", etc.
	//go:embed VERSION
	fullVersion string

	Version, VersionPrerelease, _ = strings.Cut(strings.TrimSpace(fullVersion), "-")

	// https://semver.org/#spec-item-10
	VersionMetadata = ""

	// The date/time of the build (actually the HEAD commit in git, to preserve stability)
	// This isn't just informational, but is also used by the licensing system. Default is chosen to be flagantly wrong.
	BuildDate string = "1970-01-01T00:00:01Z"
)

// BuildInfo includes all available version info for this build
type BuildInfo struct {
	SHA          string
	BuildDate    string
	HumanVersion string
	FIPS         string
}

// GetHumanVersion composes the parts of the version in a way that's suitable
// for displaying to humans.
func GetHumanVersion() string {
	version := Version
	release := VersionPrerelease
	metadata := VersionMetadata

	if release != "" {
		version += fmt.Sprintf("-%s", release)
	}

	if IsFIPS() {
		version += ".fips1402"
	}

	if metadata != "" {
		version += fmt.Sprintf("+%s", metadata)
	}

	// Strip off any single quotes added by the git information.
	return strings.ReplaceAll(version, "'", "")
}

// GetBuildInfo returns all available version information for this build.
func GetBuildInfo() *BuildInfo {
	return &BuildInfo{
		SHA:          GitCommit,
		BuildDate:    BuildDate,
		HumanVersion: GetHumanVersion(),
		FIPS:         GetFIPSInfo(),
	}
}

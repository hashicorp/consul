package version

import (
	"fmt"
	"strings"
)

var (
	// The git commit that was compiled. These will be filled in by the
	// compiler.
	GitCommit string

	// The main version number that is being run at the moment.
	//
	// Version must conform to the format expected by github.com/hashicorp/go-version
	// for tests to work.
	Version = "1.13.0"

	// https://semver.org/#spec-item-10
	VersionMetadata = ""

	// A pre-release marker for the version. If this is "" (empty string)
	// then it means that it is a final release. Otherwise, this is a pre-release
	// such as "dev" (in development), "beta", "rc1", etc.
	VersionPrerelease = "dev"
)

// GetHumanVersion composes the parts of the version in a way that's suitable
// for displaying to humans.
func GetHumanVersion() string {
	version := Version
	release := VersionPrerelease
	metadata := VersionMetadata

	if release != "" {
		version += fmt.Sprintf("-%s", release)
	}

	if metadata != "" {
		version += fmt.Sprintf("+%s", metadata)
	}

	// Strip off any single quotes added by the git information.
	return strings.ReplaceAll(version, "'", "")
}

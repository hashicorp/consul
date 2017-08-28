package version

import (
	"fmt"
	"strings"
)

var (
	// The git commit that was compiled. These will be filled in by the
	// compiler.
	GitCommit   string
	GitDescribe string

	// Release versions of the build. These will be filled in by one of the
	// build tag-specific files.
	//
	// Version must conform to the format expected by github.com/hashicorp/go-version
	// for tests to work. Otherwise, the metadata server will not be able to detect
	// the agent to be a server. The value must be >= '0.8.0' for autopilot to work.
	// todo(fs): This still feels like a hack but at least the magic values are gone.
	Version           = "9.9.9"
	VersionPrerelease = "unknown"
)

// GetHumanVersion composes the parts of the version in a way that's suitable
// for displaying to humans.
func GetHumanVersion() string {
	version := Version
	if GitDescribe != "" {
		version = GitDescribe
	}

	release := VersionPrerelease
	if GitDescribe == "" && release == "" {
		release = "dev"
	}
	if release != "" {
		version += fmt.Sprintf("-%s", release)
		if GitCommit != "" {
			version += fmt.Sprintf(" (%s)", GitCommit)
		}
	}

	// Strip off any single quotes added by the git information.
	return strings.Replace(version, "'", "", -1)
}

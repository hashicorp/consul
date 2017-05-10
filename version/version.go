package version

import (
	"fmt"
)

var (
	// The git commit that was compiled. These will be filled in by the
	// compiler.
	Name      string
	GitCommit string

	// Release versions of the build. These will be filled in by one of the
	// build tag-specific files.
	Version    string
	VersionPre string
)

// GetHumanVersion composes the parts of the version in a way that's suitable
// for displaying to humans.
func GetHumanVersion() string {
	if VersionPre == "" {
		return fmt.Sprintf("%s %s", Name, Version)
	}
	return fmt.Sprintf("%s %s (%s)", Name, Version, GitCommit)
}

package utils

import (
	"flag"

	"github.com/hashicorp/go-version"
)

var (
	TargetImageName string
	TargetVersion   string

	LatestImageName string
	LatestVersion   string

	FollowLog bool
)

const (
	DefaultImageNameOSS   = "consul"
	DefaultImageNameENT   = "hashicorp/consul-enterprise"
	ImageVersionSuffixENT = "-ent"
)

func init() {
	flag.StringVar(&TargetImageName, "target-image", defaultImageName, "docker image name to be used under test (Default: "+defaultImageName+")")
	flag.StringVar(&TargetVersion, "target-version", "local", "docker image version to be used as UUT (unit under test)")

	flag.StringVar(&LatestImageName, "latest-image", defaultImageName, "docker image name to be used under test (Default: "+defaultImageName+")")
	flag.StringVar(&LatestVersion, "latest-version", "latest", "docker image to be used as latest")

	flag.BoolVar(&FollowLog, "follow-log", true, "follow container log in output (Default: true)")
}

func DockerImage(image, version string) string {
	v := image + ":" + version
	if image == DefaultImageNameENT && isSemVer(version) {
		// Enterprise versions get a suffix.
		v += ImageVersionSuffixENT
	}
	return v
}

func isSemVer(ver string) bool {
	_, err := version.NewVersion(ver)
	return err == nil
}

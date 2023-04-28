package utils

import (
	"flag"

	"github.com/hashicorp/go-version"
)

var (
	TargetImage 	string
	TargetVersion   string

	LatestImage 	string
	LatestVersion   string
)

const (
	DefaultImageNameOSS   = "consul"
	DefaultImageNameENT   = "hashicorp/consul-enterprise"
	ImageVersionSuffixENT = "-ent"
)

func init() {
	flag.StringVar(&TargetImage, "target-image", defaultImageName, "docker image name to be used under test (Default: "+defaultImageName+")")
	flag.StringVar(&TargetVersion, "target-version", "local", "docker image version to be used as UUT (unit under test)")

	flag.StringVar(&LatestImage, "latest-image", defaultImageName, "docker image name to be used under test (Default: "+defaultImageName+")")
	flag.StringVar(&LatestVersion, "latest-version", "latest", "docker image to be used as latest")
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
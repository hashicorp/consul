package utils

import (
	"flag"
)

var (
	TargetImage   string
	TargetVersion string

	LatestImage   string
	LatestVersion string
)

func init() {
	flag.StringVar(&TargetImage, "target-image", DefaultImageName, "docker image name to be used under test (Default: "+DefaultImageName+")")
	flag.StringVar(&TargetVersion, "target-version", "local", "docker image version to be used as UUT (unit under test)")

	flag.StringVar(&LatestImage, "latest-image", DefaultImageName, "docker image name to be used under test (Default: "+DefaultImageName+")")
	flag.StringVar(&LatestVersion, "latest-version", "latest", "docker image to be used as latest")
}

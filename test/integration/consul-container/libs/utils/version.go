// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

	FollowLog       bool
	Version_1_14, _ = version.NewVersion("1.14")
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

// ensure version a >= b
func VersionGTE(a, b string) bool {
	av := version.Must(version.NewVersion(a))
	bv := version.Must(version.NewVersion(b))
	return av.GreaterThanOrEqual(bv)
}

// ensure version a < b
func VersionLT(a, b string) bool {
	av := version.Must(version.NewVersion(a))
	bv := version.Must(version.NewVersion(b))
	return av.LessThan(bv)
}

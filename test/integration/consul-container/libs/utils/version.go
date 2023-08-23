// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package utils

import (
	"flag"
	"strings"

	"github.com/hashicorp/go-version"
)

var (
	targetImageName string
	TargetVersion   string

	LatestImageName string
	LatestVersion   string

	FollowLog bool

	Debug           bool
	Version_1_14, _ = version.NewVersion("1.14")
)

const (
	DefaultImageNameCE    = "hashicorp/consul"
	DefaultImageNameENT   = "hashicorp/consul-enterprise"
	ImageVersionSuffixENT = "-ent"
)

func init() {
	flag.BoolVar(&Debug, "debug", false, "run consul with dlv to enable live debugging")
	flag.StringVar(&targetImageName, "target-image", defaultImageName, "docker image name to be used under test (Default: "+defaultImageName+")")
	flag.StringVar(&TargetVersion, "target-version", "local", "docker image version to be used as UUT (unit under test)")

	flag.StringVar(&LatestImageName, "latest-image", defaultImageName, "docker image name to be used under test (Default: "+defaultImageName+")")
	flag.StringVar(&LatestVersion, "latest-version", "latest", "docker image to be used as latest")

	flag.BoolVar(&FollowLog, "follow-log", true, "follow container log in output (Default: true)")

}

func GetTargetImageName() string {
	if Debug {
		return targetImageName + "-dbg"
	}
	return targetImageName
}

func GetLatestImageName() string {
	if Debug {
		return LatestImageName + "-dbg"
	}
	return LatestImageName
}

func IsEnterprise() bool { return isInEnterpriseRepo }

func DockerImage(image, version string) string {
	v := image + ":" + version
	if strings.Contains(image, DefaultImageNameENT) && isSemVer(version) {
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

// SideCarVersion returns version based on the agent
// version in the test: if agent has local, the sidecar
// version is target-version; otherwise use "latest-version"
func SideCarVersion(agentVersion string) string {
	imageVersion := ""
	if strings.Contains(agentVersion, "local") {
		imageVersion = "target-version"
	} else {
		imageVersion = "latest-version"
	}

	return imageVersion
}

//go:build !consulent
// +build !consulent

package utils

import "flag"

var TargetImage = flag.String("target-image", "consul", "docker image name to be used under test (Default: consul)")
var TargetVersion = flag.String("target-version", "local", "docker image version to be used as UUT (unit under test)")
var LatestImage = flag.String("latest-image", "consul", "docker image name to be used under test (Default: consul)")
var LatestVersion = flag.String("latest-version", "1.11", "docker image to be used as latest")

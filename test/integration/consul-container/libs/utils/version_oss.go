//go:build !consulent
// +build !consulent

package utils

import "flag"

var TargetImage = flag.String("target-version", "local", "docker image to be used as UUT (unit under test)")
var LatestImage = flag.String("latest-version", "1.11", "docker image to be used as latest")

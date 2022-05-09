//go:build !consulent
// +build !consulent

package upgrade

import "flag"

var targetImage = flag.String("target-version", "local", "docker image to be used as UUT (unit under test)")
var latestImage = flag.String("latest-version", "1.11", "docker image to be used as latest")

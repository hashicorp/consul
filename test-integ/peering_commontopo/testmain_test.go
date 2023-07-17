package peering

import (
	"flag"
	"os"
	"testing"
)

// By default, disallow running multiple commonTopo tests
// in parallel. While possible, running many at once tends to
// overwhelm Docker. We still want to allow -parallel > 1
// because several subtests can run  in parallel safely.
//
// commonTopo tests should do:
//
//	if allowParallelCommonTopo {
//		t.Parallel()
//	}
var allowParallelCommonTopo = false

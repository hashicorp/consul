package peering

import (
	"flag"
	"testing"
)

// Test that use commonTopo should implement sharedTopoSuite.
//
// Tests that use commonTopo are either cooperative or non-cooperative. Non-cooperative
// uses of commonTopo include is anything that may interfere with other tests, namely
// mutations, such as:
// - any calls to commonTopo.Relaunch; this is generally disruptive to other tests
// - stopping or disabling nodes
// - ...
//
// Cooperative tests should just call testFuncMayReuseCommonTopo() to ensure they
// are run in the correct `sharetopo` mode. They should also ensure they are included
// in the commonTopoSuites slice in TestSuitesOnSharedTopo.
type sharedTopoSuite interface {
	testName() string
	setup(*testing.T, *commonTopo)
	test(*testing.T, *commonTopo)
}

var flagNoShareTopo = flag.Bool("no-share-topo", false, "do not share topology; run each test in its own")

func runShareableSuites(t *testing.T, suites []sharedTopoSuite) {
	if !*flagNoShareTopo {
		t.Skip(`Will run as part of "TestSuitesOnSharedTopo"`)
	}
	ct := NewCommonTopo(t)
	for _, s := range suites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range suites {
		s := s
		t.Run(s.testName(), func(t *testing.T) {
			t.Parallel()
			s.test(t, ct)
		})
	}
}

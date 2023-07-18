package peering

import (
	"flag"
	"testing"
)

// Test that use commonTopo should implement commonTopoSuite.
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
type commonTopoSuite interface {
	testName() string
	setup(*testing.T, *commonTopo)
	test(*testing.T, *commonTopo)
}

var commonTopologyFlag = flag.Bool("commontopo", false, "run tests with common topology")

func commonTopologyTest(t *testing.T) {
	t.Helper()
	if *commonTopologyFlag {
		t.Skip(`Skipping: duplicate test run. See "TestSuitesOnSharedTopo"`)
	}
}

func setupAndRunTestSuite(t *testing.T, suites []commonTopoSuite, shareTopology, runParallel bool) {
	if shareTopology {
		commonTopologyTest(t)
	}

	if runParallel {
		t.Parallel()
	}

	ct := NewCommonTopo(t)
	for _, s := range suites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range suites {
		s := s
		t.Run(s.testName(), func(t *testing.T) {
			if runParallel {
				t.Parallel()
			}
			s.test(t, ct)
		})
	}
}

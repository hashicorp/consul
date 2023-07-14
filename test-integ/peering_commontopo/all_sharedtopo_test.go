package peering

import (
	"flag"
	"testing"
)

var FlagNoReuseCommonTopo *bool = flag.Bool("no-reuse-common-topo", false,
	"run tests that can use the common topo in separate instances")

type commonTopoSuite interface {
	testName() string
	setup(*testing.T, *commonTopo)
	test(*testing.T, *commonTopo)
}

func testFuncMayReuseCpmmonTopo(t *testing.T, suites []commonTopoSuite) {
	t.Helper()
	if !*FlagNoReuseCommonTopo {
		t.Skip("NoReuseCommonTopo unset")
	}
	if allowParallelCommonTopo {
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
			t.Parallel()
			s.test(t, ct)
		})
	}
}

// Tests compatible with commonTopo should implement commonTopoSuite and be added
// to the commonTopoSuites slice inside.
//
// func setup is executed in serial with others. They should ensure any resources
// added to ct.Cfg et all do not collide with other resources (e.g. with a prefix)
//
// func test is executed in parallel with others in a subtest. (test() should not
// call t.Parallel() itself.)
func TestSuitesOnSharedTopo(t *testing.T) {
	if *FlagNoReuseCommonTopo {
		t.Skip("NoReuseCommonTopo set")
	}
	if allowParallelCommonTopo {
		t.Parallel()
	}
	ct := NewCommonTopo(t)
	commonTopoSuites := []commonTopoSuite{}
	commonTopoSuites = append(commonTopoSuites, ac1BasicSuites...)
	commonTopoSuites = append(commonTopoSuites, ac2DiscoChainSuites...)
	commonTopoSuites = append(commonTopoSuites, ac3SvcDefaultsSuites...)
	commonTopoSuites = append(commonTopoSuites, ac4ProxyDefaultsSuites...)
	commonTopoSuites = append(commonTopoSuites, serviceMeshDisabledSuites...)

	for _, s := range commonTopoSuites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range commonTopoSuites {
		s := s
		t.Run(s.testName(), func(t *testing.T) {
			t.Parallel()
			s.test(t, ct)
		})
	}
}

func TestJustCommonTopo(t *testing.T) {
	if !*FlagNoReuseCommonTopo {
		t.Skip("NoReuseCommonTopo unset")
	}
	t.Parallel()
	ct := NewCommonTopo(t)
	ct.Launch(t)
}

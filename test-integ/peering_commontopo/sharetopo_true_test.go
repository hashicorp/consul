//go:build sharetopo

package peering

import (
	"testing"
)

func testFuncMayShareCommonTopo(t *testing.T, suites []commonTopoSuite) {
	t.Helper()
	t.Skip("sharetopo set; test will run in shared topo in TestSuitesOnSharedTopo")
}

func TestSuitesOnSharedTopo(t *testing.T) {
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

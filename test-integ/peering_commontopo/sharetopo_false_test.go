//go:build !sharetopo

package peering

import (
	"testing"
)

func testFuncMayShareCommonTopo(t *testing.T, suites []commonTopoSuite) {
	t.Helper()
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

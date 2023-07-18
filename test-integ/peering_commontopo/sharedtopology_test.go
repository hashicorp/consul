package peering

import (
	"testing"
)

// Tests compatible with commonTopo should implement commonTopoSuite and be added
// to the commonTopoSuites slice inside.
func TestSuitesOnSharedTopo(t *testing.T) {
	if !*commonTopologyFlag {
		t.Skip(`Skipping: run "go test -commontopo" to run this test suite`)
	}
	ct := NewCommonTopo(t)

	commonTopoSuites := []commonTopoSuite{}
	commonTopoSuites = append(commonTopoSuites, ac1BasicSuites...)
	commonTopoSuites = append(commonTopoSuites, ac2DiscoChainSuites...)
	commonTopoSuites = append(commonTopoSuites, ac3SvcDefaultsSuites...)
	commonTopoSuites = append(commonTopoSuites, ac4ProxyDefaultsSuites...)
	commonTopoSuites = append(commonTopoSuites, ac5_1NoSvcMeshSuites...)

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

func TestCommonTopologySetup(t *testing.T) {
	if !*commonTopologyFlag {
		t.Skip(`Skipping: run "go test -commontopo" to run this test suite`)
	}
	t.Parallel()
	ct := NewCommonTopo(t)
	ct.Launch(t)
}

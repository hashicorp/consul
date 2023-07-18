package peering

import (
	"testing"
)

// Tests that can share topo must implement sharedTopoSuite and be appended to the sharedTopoSuites
// slice inside
func TestSuitesOnSharedTopo(t *testing.T) {
	if *flagNoShareTopo {
		t.Skip(`shared topo suites disabled by -no-share-topo`)
	}
	ct := NewCommonTopo(t)

	sharedTopoSuites := []sharedTopoSuite{}
	sharedTopoSuites = append(sharedTopoSuites, ac1BasicSuites...)
	sharedTopoSuites = append(sharedTopoSuites, ac2DiscoChainSuites...)
	sharedTopoSuites = append(sharedTopoSuites, ac3SvcDefaultsSuites...)
	sharedTopoSuites = append(sharedTopoSuites, ac4ProxyDefaultsSuites...)
	sharedTopoSuites = append(sharedTopoSuites, ac5_1NoSvcMeshSuites...)

	for _, s := range sharedTopoSuites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range sharedTopoSuites {
		s := s
		t.Run(s.testName(), func(t *testing.T) {
			t.Parallel()
			s.test(t, ct)
		})
	}
}

func TestCommonTopologySetup(t *testing.T) {
	ct := NewCommonTopo(t)
	ct.Launch(t)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"flag"
	"testing"
)

// Tests that use commonTopo should implement sharedTopoSuite.
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

var flagNoShareTopo = flag.Bool("no-share-topo", false, "do not share topology; run each test in its own isolated topology")

func runShareableSuites(t *testing.T, suites []sharedTopoSuite) {
	t.Helper()
	if !*flagNoShareTopo {
		names := []string{}
		for _, s := range suites {
			names = append(names, s.testName())
		}
		t.Skipf(`Will run as part of "TestSuitesOnSharedTopo": %v`, names)
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

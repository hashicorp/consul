package xds

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func TestFirstHealthyTarget(t *testing.T) {
	passing := proxycfg.TestUpstreamNodesInStatus(t, "passing")
	warning := proxycfg.TestUpstreamNodesInStatus(t, "warning")
	critical := proxycfg.TestUpstreamNodesInStatus(t, "critical")

	warnOnlyPassingTarget := structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
		Service:    "all-warn",
		Namespace:  "default",
		Partition:  "default",
		Datacenter: "dc1",
	})
	warnOnlyPassingTarget.Subset.OnlyPassing = true
	failOnlyPassingTarget := structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
		Service:    "all-fail",
		Namespace:  "default",
		Partition:  "default",
		Datacenter: "dc1",
	})
	failOnlyPassingTarget.Subset.OnlyPassing = true

	targets := map[string]*structs.DiscoveryTarget{
		"all-ok.default.dc1": structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:    "all-ok",
			Namespace:  "default",
			Partition:  "default",
			Datacenter: "dc1",
		}),
		"all-warn.default.dc1": structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:    "all-warn",
			Namespace:  "default",
			Partition:  "default",
			Datacenter: "dc1",
		}),
		"all-fail.default.default.dc1": structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:    "all-fail",
			Namespace:  "default",
			Partition:  "default",
			Datacenter: "dc1",
		}),
		"all-warn-onlypassing.default.dc1": warnOnlyPassingTarget,
		"all-fail-onlypassing.default.dc1": failOnlyPassingTarget,
	}
	targetHealth := map[string]structs.CheckServiceNodes{
		"all-ok.default.dc1":               passing,
		"all-warn.default.dc1":             warning,
		"all-fail.default.default.dc1":     critical,
		"all-warn-onlypassing.default.dc1": warning,
		"all-fail-onlypassing.default.dc1": critical,
	}

	cases := []struct {
		primary   string
		secondary []string
		expect    string
	}{
		{
			primary: "all-ok.default.dc1",
			expect:  "all-ok.default.dc1",
		},
		{
			primary: "all-warn.default.dc1",
			expect:  "all-warn.default.dc1",
		},
		{
			primary: "all-fail.default.default.dc1",
			expect:  "all-fail.default.default.dc1",
		},
		{
			primary: "all-warn-onlypassing.default.dc1",
			expect:  "all-warn-onlypassing.default.dc1",
		},
		{
			primary: "all-fail-onlypassing.default.dc1",
			expect:  "all-fail-onlypassing.default.dc1",
		},
		{
			primary: "all-ok.default.dc1",
			secondary: []string{
				"all-warn.default.dc1",
			},
			expect: "all-ok.default.dc1",
		},
		{
			primary: "all-warn.default.dc1",
			secondary: []string{
				"all-ok.default.dc1",
			},
			expect: "all-warn.default.dc1",
		},
		{
			primary: "all-warn-onlypassing.default.dc1",
			secondary: []string{
				"all-ok.default.dc1",
			},
			expect: "all-ok.default.dc1",
		},
		{
			primary: "all-fail.default.default.dc1",
			secondary: []string{
				"all-ok.default.dc1",
			},
			expect: "all-ok.default.dc1",
		},
		{
			primary: "all-fail-onlypassing.default.dc1",
			secondary: []string{
				"all-ok.default.dc1",
			},
			expect: "all-ok.default.dc1",
		},
		{
			primary: "all-fail.default.default.dc1",
			secondary: []string{
				"all-warn-onlypassing.default.dc1",
				"all-warn.default.dc1",
				"all-ok.default.dc1",
			},
			expect: "all-warn.default.dc1",
		},
	}

	for _, tc := range cases {
		tc := tc
		name := fmt.Sprintf("%s and %v", tc.primary, tc.secondary)
		t.Run(name, func(t *testing.T) {
			targetID := firstHealthyTarget(targets, targetHealth, tc.primary, tc.secondary)
			require.Equal(t, tc.expect, targetID)
		})
	}
}

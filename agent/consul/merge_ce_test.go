// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package consul

import (
	"testing"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestMerge_CE_LAN(t *testing.T) {
	type testcase struct {
		segment   string
		server    bool
		partition string
		members   []*serf.Member
		expect    string
	}

	const thisNodeID = "ee954a2f-80de-4b34-8780-97b942a50a99"

	run := func(t *testing.T, tc testcase) {
		delegate := &lanMergeDelegate{
			dc:        "dc1",
			nodeID:    types.NodeID(thisNodeID),
			nodeName:  "node0",
			segment:   tc.segment,
			server:    tc.server,
			partition: tc.partition,
		}

		err := delegate.NotifyMerge(tc.members)

		if tc.expect == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expect)
		}
	}

	cases := map[string]testcase{
		"node in a segment": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:      "dc1",
					name:    "node1",
					build:   "0.7.5",
					segment: "alpha",
				}),
			},
			expect: `Member 'node1' part of segment 'alpha'; Network Segments are a Consul Enterprise feature`,
		},
		"node in a partition": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:        "dc1",
					name:      "node1",
					build:     "0.7.5",
					partition: "part1",
				}),
			},
			expect: `Member 'node1' part of partition 'part1'; Partitions are a Consul Enterprise feature`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/consul/version"
)

func TestMerge_LAN(t *testing.T) {
	type testcase struct {
		members []*serf.Member
		expect  string
	}

	const thisNodeID = "ee954a2f-80de-4b34-8780-97b942a50a99"

	run := func(t *testing.T, tc testcase) {
		delegate := &lanMergeDelegate{
			dc:       "dc1",
			nodeID:   types.NodeID(thisNodeID),
			nodeName: "node0",
		}

		err := delegate.NotifyMerge(tc.members)

		if tc.expect == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expect)
		}
	}

	cases := map[string]testcase{
		"client in the wrong datacenter": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc2",
					name:   "node1",
					server: false,
					build:  "0.7.5",
				}),
			},
			expect: "wrong datacenter",
		},
		"server in the wrong datacenter": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc2",
					name:   "node1",
					server: true,
					build:  "0.7.5",
				}),
			},
			expect: "wrong datacenter",
		},
		"node ID conflict with delegate's ID but same node name with same casing": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node0",
					id:     thisNodeID,
					server: true,
					build:  "0.7.5",
				}),
			},
			expect: "",
		},
		"node ID conflict with delegate's ID but same node name with different casing": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "NoDe0",
					id:     thisNodeID,
					server: true,
					build:  "0.7.5",
				}),
			},
			expect: "",
		},
		"node ID conflict with delegate's ID": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node1",
					id:     thisNodeID,
					server: true,
					build:  "0.7.5",
				}),
			},
			expect: "with this agent's ID",
		},
		"cluster with existing conflicting node IDs": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node1",
					id:     "6185913b-98d7-4441-bd8f-f7f7d854a4af",
					server: true,
					build:  "0.8.5",
				}),
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node2",
					id:     "6185913b-98d7-4441-bd8f-f7f7d854a4af",
					server: true,
					build:  "0.9.0",
				}),
			},
			expect: "with member",
		},
		"cluster with existing conflicting node IDs, but version is old enough to skip the check": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node1",
					id:     "6185913b-98d7-4441-bd8f-f7f7d854a4af",
					server: true,
					build:  "0.8.5",
				}),
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node2",
					id:     "6185913b-98d7-4441-bd8f-f7f7d854a4af",
					server: true,
					build:  "0.8.4",
				}),
			},
			expect: "with member",
		},
		"good cluster": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node1",
					server: true,
					build:  "0.8.5",
				}),
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node2",
					server: true,
					build:  "0.8.5",
				}),
			},
			expect: "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestMerge_WAN(t *testing.T) {
	type testcase struct {
		members []*serf.Member
		expect  string
		setupFn func(t *testing.T, delegate *wanMergeDelegate)
	}

	run := func(t *testing.T, tc testcase) {
		delegate := &wanMergeDelegate{
			localDatacenter: "dc1",
		}
		if tc.setupFn != nil {
			tc.setupFn(t, delegate)
		}
		err := delegate.NotifyMerge(tc.members)
		if tc.expect == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expect)
		}
	}

	cases := map[string]testcase{
		"not a server": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc2",
					name:   "node1",
					server: false,
					build:  "0.7.5",
				}),
			},
			expect: "not a server",
		},
		"good cluster": {
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc2",
					name:   "node1",
					server: true,
					build:  "0.7.5",
				}),
				makeTestNode(t, testMember{
					dc:     "dc3",
					name:   "node2",
					server: true,
					build:  "0.7.5",
				}),
			},
		},
		"federation disabled and local join allowed": {
			setupFn: func(t *testing.T, delegate *wanMergeDelegate) {
				delegate.SetWANFederationDisabled(true)
			},
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc1",
					name:   "node1",
					server: true,
					build:  "0.7.5",
				}),
			},
		},
		"federation disabled and remote join blocked": {
			setupFn: func(t *testing.T, delegate *wanMergeDelegate) {
				delegate.SetWANFederationDisabled(true)
			},
			members: []*serf.Member{
				makeTestNode(t, testMember{
					dc:     "dc2",
					name:   "node1",
					server: true,
					build:  "0.7.5",
				}),
			},
			expect: `WAN federation is disabled`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

type testMember struct {
	dc        string
	name      string
	id        string
	server    bool
	build     string
	segment   string
	partition string
}

func (tm testMember) role() string {
	if tm.server {
		return "consul"
	}
	return "node"
}

func makeTestNode(t *testing.T, tm testMember) *serf.Member {
	if tm.id == "" {
		uuid, err := uuid.GenerateUUID()
		require.NoError(t, err)
		tm.id = uuid
	}
	m := &serf.Member{
		Name: tm.name,
		Tags: map[string]string{
			"role":    tm.role(),
			"dc":      tm.dc,
			"id":      tm.id,
			"port":    "8300",
			"segment": tm.segment,
			"build":   tm.build,
			"vsn":     "2",
			"vsn_max": "3",
			"vsn_min": "2",
			"fips":    version.GetFIPSInfo(),
		},
	}
	if tm.partition != "" {
		m.Tags["ap"] = tm.partition
	}
	return m
}

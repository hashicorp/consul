// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package metadata_test

import (
	"net"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/metadata"
)

func TestServer_Key_params(t *testing.T) {
	ipv4a := net.ParseIP("127.0.0.1")
	ipv4b := net.ParseIP("1.2.3.4")

	tests := []struct {
		name  string
		sd1   *metadata.Server
		sd2   *metadata.Server
		equal bool
	}{
		{
			name: "Addr inequality",
			sd1: &metadata.Server{
				Name:       "s1",
				Datacenter: "dc1",
				Port:       8300,
				Addr:       &net.IPAddr{IP: ipv4a},
			},
			sd2: &metadata.Server{
				Name:       "s1",
				Datacenter: "dc1",
				Port:       8300,
				Addr:       &net.IPAddr{IP: ipv4b},
			},
			equal: true,
		},
	}

	for _, test := range tests {
		if test.sd1.Key().Equal(test.sd2.Key()) != test.equal {
			t.Errorf("Expected a %v result from test %s", test.equal, test.name)
		}

		// Test Key to make sure it actually works as a key
		m := make(map[metadata.Key]bool)
		m[*test.sd1.Key()] = true
		if _, found := m[*test.sd2.Key()]; found != test.equal {
			t.Errorf("Expected a %v result from map test %s", test.equal, test.name)
		}
	}
}

func TestIsConsulServer(t *testing.T) {
	mustVersion := func(s string) *version.Version {
		v, err := version.NewVersion(s)
		require.NoError(t, err)
		return v
	}

	newCase := func(variant string) (in serf.Member, expect *metadata.Server) {
		m := serf.Member{
			Name: "foo",
			Addr: net.IP([]byte{127, 0, 0, 1}),
			Port: 5454,
			Tags: map[string]string{
				"role":          "consul",
				"id":            "asdf",
				"dc":            "east-aws",
				"port":          "10000",
				"build":         "0.8.0",
				"wan_join_port": "1234",
				"grpc_port":     "9876",
				"grpc_tls_port": "9877",
				"vsn":           "1",
				"expect":        "3",
				"raft_vsn":      "3",
				"use_tls":       "1",
			},
			Status: serf.StatusLeft,
		}

		expected := &metadata.Server{
			Name:                "foo",
			ShortName:           "foo",
			ID:                  "asdf",
			Datacenter:          "east-aws",
			Segment:             "",
			Port:                10000,
			SegmentAddrs:        map[string]string{},
			SegmentPorts:        map[string]int{},
			WanJoinPort:         1234,
			LanJoinPort:         5454,
			ExternalGRPCPort:    9876,
			ExternalGRPCTLSPort: 9877,
			Bootstrap:           false,
			Expect:              3,
			Addr: &net.TCPAddr{
				IP:   net.IP([]byte{127, 0, 0, 1}),
				Port: 10000,
			},
			Build:        *mustVersion("0.8.0"),
			Version:      1,
			RaftVersion:  3,
			Status:       serf.StatusLeft,
			UseTLS:       true,
			ReadReplica:  false,
			FeatureFlags: map[string]int{},
		}

		switch variant {
		case "normal":
		case "read-replica":
			m.Tags["read_replica"] = "1"
			expected.ReadReplica = true
		case "non-voter":
			m.Tags["nonvoter"] = "1"
			expected.ReadReplica = true
		case "expect-3":
			m.Tags["expect"] = "3"
			expected.Expect = 3
		case "bootstrapped":
			m.Tags["bootstrap"] = "1"
			m.Tags["disabled"] = "1"
			expected.Bootstrap = true
		case "optionals":
			// grpc_port, wan_join_port, raft_vsn, and expect are optional and
			// should default to zero.
			delete(m.Tags, "grpc_port")
			delete(m.Tags, "wan_join_port")
			delete(m.Tags, "raft_vsn")
			delete(m.Tags, "expect")
			expected.RaftVersion = 0
			expected.Expect = 0
			expected.WanJoinPort = 0
			expected.ExternalGRPCPort = 0
		case "feature-namespaces":
			m.Tags["ft_ns"] = "1"
			expected.FeatureFlags = map[string]int{"ns": 1}
		case "no-role":
			delete(m.Tags, "role")
			//
		case "missing-grpc-port":
			delete(m.Tags, "grpc_port")
			expected.ExternalGRPCPort = 0
		case "missing-grpc-tls-port":
			delete(m.Tags, "grpc_tls_port")
			expected.ExternalGRPCTLSPort = 0
		case "missing-both-grpc-ports":
			delete(m.Tags, "grpc_port")
			delete(m.Tags, "grpc_tls_port")
			expected.ExternalGRPCPort = 0
			expected.ExternalGRPCTLSPort = 0
		case "bad-both-grpc-ports":
			m.Tags["grpc_port"] = ""
			m.Tags["grpc_tls_port"] = ""
		case "bad-grpc-port":
			m.Tags["grpc_port"] = "three"
			m.Tags["grpc_tls_port"] = ""
		case "bad-grpc-tls-port":
			m.Tags["grpc_port"] = ""
			m.Tags["grpc_tls_port"] = "three"
		case "negative-grpc-port":
			m.Tags["grpc_port"] = "-1"
			m.Tags["grpc_tls_port"] = ""
		case "negative-grpc-tls-port":
			m.Tags["grpc_port"] = ""
			m.Tags["grpc_tls_port"] = "-1"
		case "zero-grpc-port":
			m.Tags["grpc_port"] = "0"
			m.Tags["grpc_tls_port"] = ""
		case "zero-grpc-tls-port":
			m.Tags["grpc_port"] = ""
			m.Tags["grpc_tls_port"] = "0"
		default:
			t.Fatalf("unhandled variant: %s", variant)
		}

		return m, expected
	}

	run := func(t *testing.T, variant string, expectOK bool) {
		m, expected := newCase(variant)
		ok, parts := metadata.IsConsulServer(m)

		if expectOK {
			require.True(t, ok, "expected a valid consul server")
			require.Equal(t, expected, parts)
		} else {
			ok, _ := metadata.IsConsulServer(m)
			require.False(t, ok, "expected to not be a consul server")
		}
	}

	cases := map[string]bool{
		"normal":             true,
		"read-replica":       true,
		"non-voter":          true,
		"expect-3":           true,
		"bootstrapped":       true,
		"optionals":          true,
		"feature-namespaces": true,
		"no-role":            false,
		//
		"missing-grpc-port":       true,
		"missing-grpc-tls-port":   true,
		"missing-both-grpc-ports": true,
		"bad-both-grpc-ports":     false,
		"bad-grpc-port":           false,
		"negative-grpc-port":      false,
		"zero-grpc-port":          false,
		"bad-grpc-tls-port":       false,
		"negative-grpc-tls-port":  false,
		"zero-grpc-tls-port":      false,
	}

	for variant, expectOK := range cases {
		t.Run(variant, func(t *testing.T) {
			run(t, variant, expectOK)
		})
	}
}

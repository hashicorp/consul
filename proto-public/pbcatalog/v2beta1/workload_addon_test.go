// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package catalogv2beta1

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetMeshPort(t *testing.T) {
	cases := map[string]struct {
		ports map[string]*WorkloadPort
		exp   string
	}{
		"nil ports": {
			ports: nil,
			exp:   "",
		},
		"empty ports": {
			ports: make(map[string]*WorkloadPort),
			exp:   "",
		},
		"no mesh ports": {
			ports: map[string]*WorkloadPort{
				"p1": {Port: 1000, Protocol: Protocol_PROTOCOL_HTTP},
				"p2": {Port: 2000, Protocol: Protocol_PROTOCOL_TCP},
			},
			exp: "",
		},
		"one mesh port": {
			ports: map[string]*WorkloadPort{
				"p1": {Port: 1000, Protocol: Protocol_PROTOCOL_HTTP},
				"p2": {Port: 2000, Protocol: Protocol_PROTOCOL_TCP},
				"p3": {Port: 3000, Protocol: Protocol_PROTOCOL_MESH},
			},
			exp: "p3",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			workload := Workload{
				Ports: c.ports,
			}
			meshPort, ok := workload.GetMeshPortName()
			if c.exp != "" {
				require.True(t, ok)
				require.Equal(t, c.exp, meshPort)
			}
		})
	}
}

func TestWorkloadIsMeshEnabled(t *testing.T) {
	cases := map[string]struct {
		ports map[string]*WorkloadPort
		exp   bool
	}{
		"no ports": {
			ports: nil,
			exp:   false,
		},
		"no mesh": {
			ports: map[string]*WorkloadPort{
				"p1": {Port: 8080},
				"p2": {Port: 8081},
			},
			exp: false,
		},
		"with mesh": {
			ports: map[string]*WorkloadPort{
				"p1": {Port: 8080},
				"p2": {Port: 8081, Protocol: Protocol_PROTOCOL_MESH},
			},
			exp: false,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := &Workload{
				Ports: c.ports,
			}
			require.Equal(t, c.exp, w.IsMeshEnabled())
		})
	}
}

func TestGetAddressesForPort(t *testing.T) {
	cases := map[string]struct {
		addresses    []*WorkloadAddress
		ports        map[string]*WorkloadPort
		portName     string
		expAddresses []*WorkloadAddress
	}{
		"empty addresses": {
			addresses:    nil,
			ports:        nil,
			portName:     "doesn't matter",
			expAddresses: nil,
		},
		"addresses without selected port": {
			addresses:    []*WorkloadAddress{{Host: "1.1.1.1"}},
			ports:        nil,
			portName:     "not-found",
			expAddresses: nil,
		},
		"single selected address": {
			addresses: []*WorkloadAddress{
				{Host: "1.1.1.1", Ports: []string{"p1", "p2"}},
				{Host: "2.2.2.2", Ports: []string{"p3", "p4"}},
			},
			ports: map[string]*WorkloadPort{
				"p1": {Port: 8080},
				"p2": {Port: 8081},
				"p3": {Port: 8082},
				"p4": {Port: 8083},
			},
			portName: "p1",
			expAddresses: []*WorkloadAddress{
				{Host: "1.1.1.1", Ports: []string{"p1", "p2"}},
			},
		},
		"multiple selected addresses": {
			addresses: []*WorkloadAddress{
				{Host: "1.1.1.1", Ports: []string{"p1", "p2"}},
				{Host: "2.2.2.2", Ports: []string{"p3", "p4"}},
				{Host: "3.3.3.3"},
				{Host: "3.3.3.3", Ports: []string{"p1"}, External: true},
			},
			ports: map[string]*WorkloadPort{
				"p1": {Port: 8080},
				"p2": {Port: 8081},
				"p3": {Port: 8082},
				"p4": {Port: 8083},
			},
			portName: "p1",
			expAddresses: []*WorkloadAddress{
				{Host: "1.1.1.1", Ports: []string{"p1", "p2"}},
				{Host: "3.3.3.3"},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			workload := Workload{
				Addresses: c.addresses,
				Ports:     c.ports,
			}

			actualAddresses := workload.GetNonExternalAddressesForPort(c.portName)
			require.Equal(t, c.expAddresses, actualAddresses)
		})
	}
}

func TestGetFirstNonExternalMeshAddress(t *testing.T) {
	cases := map[string]struct {
		workload   *Workload
		expAddress *WorkloadAddress
	}{
		"empty addresses": {
			workload:   &Workload{},
			expAddress: nil,
		},
		"no mesh port": {
			workload: &Workload{
				Addresses: []*WorkloadAddress{{Host: "1.1.1.1"}},
				Ports: map[string]*WorkloadPort{
					"tcp": {Port: 8080},
				},
			},
			expAddress: nil,
		},
		"only external mesh ports": {
			workload: &Workload{
				Addresses: []*WorkloadAddress{{Host: "1.1.1.1", External: true}},
				Ports: map[string]*WorkloadPort{
					"mesh": {Port: 8080, Protocol: Protocol_PROTOCOL_MESH},
				},
			},
			expAddress: nil,
		},
		"only external and internal mesh ports": {
			workload: &Workload{
				Addresses: []*WorkloadAddress{
					{Host: "1.1.1.1"},
					{Host: "2.2.2.2", External: true},
				},
				Ports: map[string]*WorkloadPort{
					"mesh": {Port: 8080, Protocol: Protocol_PROTOCOL_MESH},
				},
			},
			expAddress: &WorkloadAddress{Host: "1.1.1.1"},
		},
		"multiple internal addresses for mesh port": {
			workload: &Workload{
				Addresses: []*WorkloadAddress{
					{Host: "1.1.1.1"},
					{Host: "2.2.2.2", External: true},
					{Host: "3.3.3.3"},
				},
				Ports: map[string]*WorkloadPort{
					"mesh": {Port: 8080, Protocol: Protocol_PROTOCOL_MESH},
				},
			},
			expAddress: &WorkloadAddress{Host: "1.1.1.1"},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actualAddress := c.workload.GetFirstNonExternalMeshAddress()
			require.Equal(t, actualAddress, c.expAddress)
		})
	}
}

func TestGetPortsByProtocol(t *testing.T) {
	cases := map[string]struct {
		w   *Workload
		exp map[Protocol][]string
	}{
		"nil": {
			w:   nil,
			exp: nil,
		},
		"ports with protocols": {
			w: &Workload{
				Ports: map[string]*WorkloadPort{
					"p1": {Port: 8080, Protocol: Protocol_PROTOCOL_TCP},
					"p2": {Port: 8081, Protocol: Protocol_PROTOCOL_HTTP},
					"p3": {Port: 8082, Protocol: Protocol_PROTOCOL_HTTP2},
					"p4": {Port: 8083, Protocol: Protocol_PROTOCOL_TCP},
					"p5": {Port: 8084, Protocol: Protocol_PROTOCOL_MESH},
					"p6": {Port: 8085, Protocol: Protocol_PROTOCOL_MESH},
				},
			},
			exp: map[Protocol][]string{
				Protocol_PROTOCOL_TCP:   {"p1", "p4"},
				Protocol_PROTOCOL_HTTP:  {"p2"},
				Protocol_PROTOCOL_HTTP2: {"p3"},
				Protocol_PROTOCOL_MESH:  {"p5", "p6"},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			portsByProtocol := c.w.GetPortsByProtocol()
			for protocol, ports := range portsByProtocol {
				sort.Strings(ports)
				portsByProtocol[protocol] = ports
			}
			require.Equal(t, c.exp, portsByProtocol)
		})
	}
}

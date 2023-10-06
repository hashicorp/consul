package catalogv2beta1

import (
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

func TestGetAddressesForPort(t *testing.T) {
	cases := map[string]struct {
		addresses    []*WorkloadAddress
		portName     string
		expAddresses []*WorkloadAddress
	}{
		"empty addresses": {
			addresses:    nil,
			portName:     "doesn't matter",
			expAddresses: nil,
		},
		"addresses without selected port": {
			addresses:    []*WorkloadAddress{{Host: "1.1.1.1"}},
			portName:     "not-found",
			expAddresses: nil,
		},
		"single selected addresses": {
			addresses: []*WorkloadAddress{
				{Host: "1.1.1.1", Ports: []string{"p1", "p2"}},
				{Host: "2.2.2.2", Ports: []string{"p3", "p4"}},
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
			}

			actualAddresses := workload.GetNonExternalAddressesForPort(c.portName)
			require.Equal(t, actualAddresses, c.expAddresses)
		})
	}
}

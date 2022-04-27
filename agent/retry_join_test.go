package agent

import (
	"bytes"
	"net"
	"strconv"
	"testing"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestAgentRetryNewDiscover(t *testing.T) {
	d, err := newDiscover()
	require.NoError(t, err)
	expected := []string{
		"aliyun", "aws", "azure", "digitalocean", "gce", "k8s", "linode",
		"mdns", "os", "packet", "scaleway", "softlayer", "tencentcloud",
		"triton", "vsphere",
	}
	require.Equal(t, expected, d.Names())
}

func TestAgentRetryJoinAddrs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	lanSelfIP := "192.168.0.14"
	wanSelfIP := "192.168.0.15"
	selfAddrs := []string{
		net.JoinHostPort(lanSelfIP, strconv.Itoa(consul.DefaultLANSerfPort)),
		net.JoinHostPort(wanSelfIP, strconv.Itoa(consul.DefaultWANSerfPort)),
	}

	d, err := newDiscover()
	require.NoError(t, err)

	tests := []struct {
		name     string
		exclude  []string
		input    []string
		expected []string
	}{
		{"handles nil", selfAddrs, nil, []string{}},
		{"handles empty input", selfAddrs, []string{}, []string{}},
		{"handles one element",
			selfAddrs,
			[]string{"192.168.0.12"},
			[]string{"192.168.0.12"},
		},
		{"handles two elements",
			selfAddrs,
			[]string{"192.168.0.12", "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
		},
		{"tries to resolve aws things, which fails but that is fine",
			selfAddrs,
			[]string{"192.168.0.12", "provider=aws region=eu-west-1 tag_key=consul tag_value=tag access_key_id=a secret_access_key=a"},
			[]string{"192.168.0.12"},
		},
		{"ignores self entry with same IP and port",
			selfAddrs,
			[]string{"192.168.0.12", selfAddrs[0], "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
		},
		{"ignores multiple self entries with same IP and port",
			selfAddrs,
			[]string{"192.168.0.12", selfAddrs[0], selfAddrs[1], "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
		},
		{"does not ignore self entry with different port",
			selfAddrs,
			[]string{"192.168.0.12", net.JoinHostPort(lanSelfIP, strconv.Itoa(consul.DefaultLANSerfPort+1)), "192.168.0.13"},
			[]string{"192.168.0.12", net.JoinHostPort(lanSelfIP, strconv.Itoa(consul.DefaultLANSerfPort+1)), "192.168.0.13"},
		},
		{"ignores self when no port is supplied (LAN)",
			selfAddrs,
			[]string{"192.168.0.12", lanSelfIP, "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
		},
		{"ignores self when no port is supplied (WAN)",
			selfAddrs,
			[]string{"192.168.0.12", wanSelfIP, "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
		},
		{"ignores self LAN and WAN entries when no port is supplied",
			selfAddrs,
			[]string{"192.168.0.12", lanSelfIP, wanSelfIP, "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
		},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := testutil.LoggerWithOutput(t, &buf)

			output := retryJoinAddrs(d, retryJoinSerfVariant, "LAN", test.input, test.exclude, logger)
			bufout := buf.String()
			require.Equal(t, test.expected, output, bufout)
			if i == 4 {
				require.Contains(t, bufout, `Using provider "aws"`)
			}
		})
	}
	t.Run("handles nil discover", func(t *testing.T) {
		require.Equal(t, []string{}, retryJoinAddrs(nil, retryJoinSerfVariant, "LAN", []string{"a"}, selfAddrs, nil))
	})
}

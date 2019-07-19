package agent

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentRetryNewDiscover(t *testing.T) {
	d, err := newDiscover()
	require.NoError(t, err)
	expected := []string{
		"aliyun", "aws", "azure", "digitalocean", "gce", "k8s", "mdns",
		"os", "packet", "scaleway", "softlayer", "triton", "vsphere",
	}
	require.Equal(t, expected, d.Names())
}

func TestAgentRetryJoinAddrs(t *testing.T) {
	d, err := newDiscover()
	require.NoError(t, err)

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"handles nil", nil, []string{}},
		{"handles empty input", []string{}, []string{}},
		{"handles one element",
			[]string{"192.168.0.12"},
			[]string{"192.168.0.12"},
		},
		{"handles two elements",
			[]string{"192.168.0.12", "192.168.0.13"},
			[]string{"192.168.0.12", "192.168.0.13"},
		},
		{"tries to resolve aws things, which fails but that is fine",
			[]string{"192.168.0.12", "provider=aws region=eu-west-1 tag_key=consul tag_value=tag access_key_id=a secret_access_key=a"},
			[]string{"192.168.0.12"},
		},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.New(&buf, "logger: ", log.Lshortfile)
			require.Equal(t, test.expected, retryJoinAddrs(d, "LAN", test.input, logger), buf.String())
			if i == 4 {
				require.Contains(t, buf.String(), `Using provider "aws"`)
			}
		})
	}
	t.Run("handles nil discover", func(t *testing.T) {
		require.Equal(t, []string{}, retryJoinAddrs(nil, "LAN", []string{"a"}, nil))
	})
}

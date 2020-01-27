package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpiffeIDAgentURI(t *testing.T) {
	agent := &SpiffeIDAgent{
		Host:       "1234.consul",
		Datacenter: "dc1",
		Agent:      "123",
	}

	require.Equal(t, "spiffe://1234.consul/agent/client/dc/dc1/id/123", agent.URI().String())
}

func TestSpiffeIDAgentAuthorize(t *testing.T) {
	agent := &SpiffeIDAgent{
		Host:  "1234.consul",
		Agent: "uuid",
	}

	auth, match := agent.Authorize(nil)
	require.False(t, auth)
	require.False(t, match)
}

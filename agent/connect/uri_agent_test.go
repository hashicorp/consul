package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpiffeIDAgentURI(t *testing.T) {
	agent := &SpiffeIDAgent{
		Host:  "1234.consul",
		Agent: "uuid",
	}

	require.Equal(t, "spiffe://1234.consul/agent/uuid", agent.URI().String())
}

func TestSpiffeIDAgentAuthorize(t *testing.T) {
	agent := &SpiffeIDAgent{
		Host:  "1234.consul",
		Agent: "uuid",
	}

	auth, match := agent.Authorize(nil)
	require.True(t, auth)
	require.True(t, match)
}

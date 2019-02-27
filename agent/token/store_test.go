package token

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStore_RegularTokens(t *testing.T) {
	t.Parallel()

	type tokens struct {
		userSource   TokenSource
		user         string
		agent        string
		agentSource  TokenSource
		master       string
		masterSource TokenSource
		repl         string
		replSource   TokenSource
	}

	tests := []struct {
		name      string
		set       tokens
		raw       tokens
		effective tokens
	}{
		{
			name:      "set user - config",
			set:       tokens{user: "U", userSource: TokenSourceConfig},
			raw:       tokens{user: "U", userSource: TokenSourceConfig},
			effective: tokens{user: "U", agent: "U"},
		},
		{
			name:      "set user - api",
			set:       tokens{user: "U", userSource: TokenSourceAPI},
			raw:       tokens{user: "U", userSource: TokenSourceAPI},
			effective: tokens{user: "U", agent: "U"},
		},
		{
			name:      "set agent - config",
			set:       tokens{agent: "A", agentSource: TokenSourceConfig},
			raw:       tokens{agent: "A", agentSource: TokenSourceConfig},
			effective: tokens{agent: "A"},
		},
		{
			name:      "set agent - api",
			set:       tokens{agent: "A", agentSource: TokenSourceAPI},
			raw:       tokens{agent: "A", agentSource: TokenSourceAPI},
			effective: tokens{agent: "A"},
		},
		{
			name:      "set user and agent",
			set:       tokens{agent: "A", user: "U"},
			raw:       tokens{agent: "A", user: "U"},
			effective: tokens{agent: "A", user: "U"},
		},
		{
			name:      "set repl - config",
			set:       tokens{repl: "R", replSource: TokenSourceConfig},
			raw:       tokens{repl: "R", replSource: TokenSourceConfig},
			effective: tokens{repl: "R"},
		},
		{
			name:      "set repl - api",
			set:       tokens{repl: "R", replSource: TokenSourceAPI},
			raw:       tokens{repl: "R", replSource: TokenSourceAPI},
			effective: tokens{repl: "R"},
		},
		{
			name:      "set master - config",
			set:       tokens{master: "M", masterSource: TokenSourceConfig},
			raw:       tokens{master: "M", masterSource: TokenSourceConfig},
			effective: tokens{master: "M"},
		},
		{
			name:      "set master - api",
			set:       tokens{master: "M", masterSource: TokenSourceAPI},
			raw:       tokens{master: "M", masterSource: TokenSourceAPI},
			effective: tokens{master: "M"},
		},
		{
			name:      "set all",
			set:       tokens{user: "U", agent: "A", repl: "R", master: "M"},
			raw:       tokens{user: "U", agent: "A", repl: "R", master: "M"},
			effective: tokens{user: "U", agent: "A", repl: "R", master: "M"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := new(Store)
			s.UpdateUserToken(tt.set.user, tt.set.userSource)
			s.UpdateAgentToken(tt.set.agent, tt.set.agentSource)
			s.UpdateReplicationToken(tt.set.repl, tt.set.replSource)
			s.UpdateAgentMasterToken(tt.set.master, tt.set.masterSource)

			require.Equal(t, tt.effective.user, s.UserToken())
			require.Equal(t, tt.effective.agent, s.AgentToken())
			require.Equal(t, tt.effective.master, s.AgentMasterToken())
			require.Equal(t, tt.effective.repl, s.ReplicationToken())

			tok, src := s.UserTokenAndSource()
			require.Equal(t, tt.raw.user, tok)
			require.Equal(t, tt.raw.userSource, src)

			tok, src = s.AgentTokenAndSource()
			require.Equal(t, tt.raw.agent, tok)
			require.Equal(t, tt.raw.agentSource, src)

			tok, src = s.AgentMasterTokenAndSource()
			require.Equal(t, tt.raw.master, tok)
			require.Equal(t, tt.raw.masterSource, src)

			tok, src = s.ReplicationTokenAndSource()
			require.Equal(t, tt.raw.repl, tok)
			require.Equal(t, tt.raw.replSource, src)
		})
	}
}

func TestStore_AgentMasterToken(t *testing.T) {
	t.Parallel()
	s := new(Store)

	verify := func(want bool, toks ...string) {
		for _, tok := range toks {
			require.Equal(t, want, s.IsAgentMasterToken(tok))
		}
	}

	verify(false, "", "nope")

	s.UpdateAgentMasterToken("master", TokenSourceConfig)
	verify(true, "master")
	verify(false, "", "nope")

	s.UpdateAgentMasterToken("another", TokenSourceConfig)
	verify(true, "another")
	verify(false, "", "nope", "master")

	s.UpdateAgentMasterToken("", TokenSourceConfig)
	verify(false, "", "nope", "master", "another")
}

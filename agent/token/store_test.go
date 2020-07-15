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
			require.True(t, s.UpdateUserToken(tt.set.user, tt.set.userSource))
			require.True(t, s.UpdateAgentToken(tt.set.agent, tt.set.agentSource))
			require.True(t, s.UpdateReplicationToken(tt.set.repl, tt.set.replSource))
			require.True(t, s.UpdateAgentMasterToken(tt.set.master, tt.set.masterSource))

			// If they don't change then they return false.
			require.False(t, s.UpdateUserToken(tt.set.user, tt.set.userSource))
			require.False(t, s.UpdateAgentToken(tt.set.agent, tt.set.agentSource))
			require.False(t, s.UpdateReplicationToken(tt.set.repl, tt.set.replSource))
			require.False(t, s.UpdateAgentMasterToken(tt.set.master, tt.set.masterSource))

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

func TestStore_Notify(t *testing.T) {
	t.Parallel()
	s := new(Store)

	newNotification := func(t *testing.T, s *Store, kind TokenKind) Notifier {
		n := s.Notify(kind)
		require.NotNil(t, n.Ch)
		return n
	}

	requireNotNotified := func(t *testing.T, ch <-chan struct{}) {
		require.Empty(t, ch)
	}

	requireNotifiedOnce := func(t *testing.T, ch <-chan struct{}) {
		require.Len(t, ch, 1)
		// drain the channel
		<-ch
		// just to be safe
		require.Empty(t, ch)
	}

	agentNotifier := newNotification(t, s, TokenKindAgent)
	userNotifier := newNotification(t, s, TokenKindUser)
	agentMasterNotifier := newNotification(t, s, TokenKindAgentMaster)
	replicationNotifier := newNotification(t, s, TokenKindReplication)
	replicationNotifier2 := newNotification(t, s, TokenKindReplication)

	// perform an update of the user token
	require.True(t, s.UpdateUserToken("edcae2a2-3b51-4864-b412-c7a568f49cb1", TokenSourceConfig))
	// do it again to ensure it doesn't block even though nothing has read from the 1 buffered chan yet
	require.True(t, s.UpdateUserToken("47788919-f944-476a-bda5-446d64be1df8", TokenSourceAPI))

	// ensure notifications were sent to the user and all notifiers
	requireNotNotified(t, agentNotifier.Ch)
	requireNotifiedOnce(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentMasterNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// now update the agent token which should send notificaitons to the agent and all notifier
	require.True(t, s.UpdateAgentToken("5d748ec2-d536-461f-8e2a-1f7eae98d559", TokenSourceAPI))

	requireNotifiedOnce(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentMasterNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// now update the agent master token which should send notificaitons to the agent master and all notifier
	require.True(t, s.UpdateAgentMasterToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotifiedOnce(t, agentMasterNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// now update the replication token which should send notificaitons to the replication and all notifier
	require.True(t, s.UpdateReplicationToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier.Ch)
	requireNotNotified(t, agentMasterNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier2.Ch)

	s.StopNotify(replicationNotifier2)

	// now update the replication token which should send notificaitons to the replication and all notifier
	require.True(t, s.UpdateReplicationToken("eb0b56b9-fa65-4ae1-902a-c64457c62ac6", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier.Ch)
	requireNotNotified(t, agentMasterNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// request updates but that are not changes
	require.False(t, s.UpdateAgentToken("5d748ec2-d536-461f-8e2a-1f7eae98d559", TokenSourceAPI))
	require.False(t, s.UpdateAgentMasterToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))
	require.False(t, s.UpdateUserToken("47788919-f944-476a-bda5-446d64be1df8", TokenSourceAPI))
	require.False(t, s.UpdateReplicationToken("eb0b56b9-fa65-4ae1-902a-c64457c62ac6", TokenSourceAPI))

	// ensure that notifications were not sent
	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentMasterNotifier.Ch)
}

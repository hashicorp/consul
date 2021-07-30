package token

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStore_RegularTokens(t *testing.T) {
	type tokens struct {
		userSource  TokenSource
		user        string
		agent       string
		agentSource TokenSource
		root        string
		rootSource  TokenSource
		repl        string
		replSource  TokenSource
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
			name:      "set root - config",
			set:       tokens{root: "M", rootSource: TokenSourceConfig},
			raw:       tokens{root: "M", rootSource: TokenSourceConfig},
			effective: tokens{root: "M"},
		},
		{
			name:      "set root - api",
			set:       tokens{root: "M", rootSource: TokenSourceAPI},
			raw:       tokens{root: "M", rootSource: TokenSourceAPI},
			effective: tokens{root: "M"},
		},
		{
			name:      "set all",
			set:       tokens{user: "U", agent: "A", repl: "R", root: "M"},
			raw:       tokens{user: "U", agent: "A", repl: "R", root: "M"},
			effective: tokens{user: "U", agent: "A", repl: "R", root: "M"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := new(Store)
			if tt.set.user != "" {
				require.True(t, s.UpdateUserToken(tt.set.user, tt.set.userSource))
			}

			if tt.set.agent != "" {
				require.True(t, s.UpdateAgentToken(tt.set.agent, tt.set.agentSource))
			}

			if tt.set.repl != "" {
				require.True(t, s.UpdateReplicationToken(tt.set.repl, tt.set.replSource))
			}

			if tt.set.root != "" {
				require.True(t, s.UpdateAgentRootToken(tt.set.root, tt.set.rootSource))
			}

			// If they don't change then they return false.
			require.False(t, s.UpdateUserToken(tt.set.user, tt.set.userSource))
			require.False(t, s.UpdateAgentToken(tt.set.agent, tt.set.agentSource))
			require.False(t, s.UpdateReplicationToken(tt.set.repl, tt.set.replSource))
			require.False(t, s.UpdateAgentRootToken(tt.set.root, tt.set.rootSource))

			require.Equal(t, tt.effective.user, s.UserToken())
			require.Equal(t, tt.effective.agent, s.AgentToken())
			require.Equal(t, tt.effective.root, s.AgentRootToken())
			require.Equal(t, tt.effective.repl, s.ReplicationToken())

			tok, src := s.UserTokenAndSource()
			require.Equal(t, tt.raw.user, tok)
			require.Equal(t, tt.raw.userSource, src)

			tok, src = s.AgentTokenAndSource()
			require.Equal(t, tt.raw.agent, tok)
			require.Equal(t, tt.raw.agentSource, src)

			tok, src = s.AgentRootTokenAndSource()
			require.Equal(t, tt.raw.root, tok)
			require.Equal(t, tt.raw.rootSource, src)

			tok, src = s.ReplicationTokenAndSource()
			require.Equal(t, tt.raw.repl, tok)
			require.Equal(t, tt.raw.replSource, src)
		})
	}
}

func TestStore_AgentRootToken(t *testing.T) {
	s := new(Store)

	verify := func(want bool, toks ...string) {
		for _, tok := range toks {
			require.Equal(t, want, s.IsAgentRootToken(tok))
		}
	}

	verify(false, "", "nope")

	s.UpdateAgentRootToken("root", TokenSourceConfig)
	verify(true, "root")
	verify(false, "", "nope")

	s.UpdateAgentRootToken("another", TokenSourceConfig)
	verify(true, "another")
	verify(false, "", "nope", "root")

	s.UpdateAgentRootToken("", TokenSourceConfig)
	verify(false, "", "nope", "root", "another")
}

func TestStore_Notify(t *testing.T) {
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
	agentRootNotifier := newNotification(t, s, TokenKindAgentRoot)
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
	requireNotNotified(t, agentRootNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// now update the agent token which should send notificaitons to the agent and all notifier
	require.True(t, s.UpdateAgentToken("5d748ec2-d536-461f-8e2a-1f7eae98d559", TokenSourceAPI))

	requireNotifiedOnce(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRootNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// now update the agent root token which should send notificaitons to the agent root and all notifier
	require.True(t, s.UpdateAgentRootToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotifiedOnce(t, agentRootNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// now update the replication token which should send notificaitons to the replication and all notifier
	require.True(t, s.UpdateReplicationToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRootNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier2.Ch)

	s.StopNotify(replicationNotifier2)

	// now update the replication token which should send notificaitons to the replication and all notifier
	require.True(t, s.UpdateReplicationToken("eb0b56b9-fa65-4ae1-902a-c64457c62ac6", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRootNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)

	// request updates but that are not changes
	require.False(t, s.UpdateAgentToken("5d748ec2-d536-461f-8e2a-1f7eae98d559", TokenSourceAPI))
	require.False(t, s.UpdateAgentRootToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))
	require.False(t, s.UpdateUserToken("47788919-f944-476a-bda5-446d64be1df8", TokenSourceAPI))
	require.False(t, s.UpdateReplicationToken("eb0b56b9-fa65-4ae1-902a-c64457c62ac6", TokenSourceAPI))

	// ensure that notifications were not sent
	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRootNotifier.Ch)
}

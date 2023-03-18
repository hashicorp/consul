package token

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStore_RegularTokens(t *testing.T) {
	type tokens struct {
		userSource         TokenSource
		user               string
		agent              string
		agentSource        TokenSource
		recovery           string
		recoverySource     TokenSource
		repl               string
		replSource         TokenSource
		registration       string
		registrationSource TokenSource
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
			name:      "set recovery - config",
			set:       tokens{recovery: "M", recoverySource: TokenSourceConfig},
			raw:       tokens{recovery: "M", recoverySource: TokenSourceConfig},
			effective: tokens{recovery: "M"},
		},
		{
			name:      "set recovery - api",
			set:       tokens{recovery: "M", recoverySource: TokenSourceAPI},
			raw:       tokens{recovery: "M", recoverySource: TokenSourceAPI},
			effective: tokens{recovery: "M"},
		},
		{
			name:      "set registration - config",
			set:       tokens{registration: "G", registrationSource: TokenSourceConfig},
			raw:       tokens{registration: "G", registrationSource: TokenSourceConfig},
			effective: tokens{registration: "G"},
		},
		{
			name:      "set registration - api",
			set:       tokens{registration: "G", registrationSource: TokenSourceAPI},
			raw:       tokens{registration: "G", registrationSource: TokenSourceAPI},
			effective: tokens{registration: "G"},
		},
		{
			name:      "set all",
			set:       tokens{user: "U", agent: "A", repl: "R", recovery: "M", registration: "G"},
			raw:       tokens{user: "U", agent: "A", repl: "R", recovery: "M", registration: "G"},
			effective: tokens{user: "U", agent: "A", repl: "R", recovery: "M", registration: "G"},
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

			if tt.set.recovery != "" {
				require.True(t, s.UpdateAgentRecoveryToken(tt.set.recovery, tt.set.recoverySource))
			}

			if tt.set.registration != "" {
				require.True(t, s.UpdateConfigFileRegistrationToken(tt.set.registration, tt.set.registrationSource))
			}

			// If they don't change then they return false.
			require.False(t, s.UpdateUserToken(tt.set.user, tt.set.userSource))
			require.False(t, s.UpdateAgentToken(tt.set.agent, tt.set.agentSource))
			require.False(t, s.UpdateReplicationToken(tt.set.repl, tt.set.replSource))
			require.False(t, s.UpdateAgentRecoveryToken(tt.set.recovery, tt.set.recoverySource))
			require.False(t, s.UpdateConfigFileRegistrationToken(tt.set.registration, tt.set.registrationSource))

			require.Equal(t, tt.effective.user, s.UserToken())
			require.Equal(t, tt.effective.agent, s.AgentToken())
			require.Equal(t, tt.effective.recovery, s.AgentRecoveryToken())
			require.Equal(t, tt.effective.repl, s.ReplicationToken())
			require.Equal(t, tt.effective.registration, s.ConfigFileRegistrationToken())

			tok, src := s.UserTokenAndSource()
			require.Equal(t, tt.raw.user, tok)
			require.Equal(t, tt.raw.userSource, src)

			tok, src = s.AgentTokenAndSource()
			require.Equal(t, tt.raw.agent, tok)
			require.Equal(t, tt.raw.agentSource, src)

			tok, src = s.AgentRecoveryTokenAndSource()
			require.Equal(t, tt.raw.recovery, tok)
			require.Equal(t, tt.raw.recoverySource, src)

			tok, src = s.ReplicationTokenAndSource()
			require.Equal(t, tt.raw.repl, tok)
			require.Equal(t, tt.raw.replSource, src)

			tok, src = s.ConfigFileRegistrationTokenAndSource()
			require.Equal(t, tt.raw.registration, tok)
			require.Equal(t, tt.raw.registrationSource, src)
		})
	}
}

func TestStore_AgentRecoveryToken(t *testing.T) {
	s := new(Store)

	verify := func(want bool, toks ...string) {
		for _, tok := range toks {
			require.Equal(t, want, s.IsAgentRecoveryToken(tok))
		}
	}

	verify(false, "", "nope")

	s.UpdateAgentRecoveryToken("recovery", TokenSourceConfig)
	verify(true, "recovery")
	verify(false, "", "nope")

	s.UpdateAgentRecoveryToken("another", TokenSourceConfig)
	verify(true, "another")
	verify(false, "", "nope", "recovery")

	s.UpdateAgentRecoveryToken("", TokenSourceConfig)
	verify(false, "", "nope", "recovery", "another")
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
	agentRecoveryNotifier := newNotification(t, s, TokenKindAgentRecovery)
	replicationNotifier := newNotification(t, s, TokenKindReplication)
	replicationNotifier2 := newNotification(t, s, TokenKindReplication)
	registrationNotifier := newNotification(t, s, TokenKindConfigFileRegistration)

	// perform an update of the user token
	require.True(t, s.UpdateUserToken("edcae2a2-3b51-4864-b412-c7a568f49cb1", TokenSourceConfig))
	// do it again to ensure it doesn't block even though nothing has read from the 1 buffered chan yet
	require.True(t, s.UpdateUserToken("47788919-f944-476a-bda5-446d64be1df8", TokenSourceAPI))

	// ensure notifications were sent to the user notifier and all other notifiers were not notified.
	requireNotNotified(t, agentNotifier.Ch)
	requireNotifiedOnce(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRecoveryNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)
	requireNotNotified(t, registrationNotifier.Ch)

	// update the agent token which should send a notification to the agent notifier.
	require.True(t, s.UpdateAgentToken("5d748ec2-d536-461f-8e2a-1f7eae98d559", TokenSourceAPI))

	requireNotifiedOnce(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRecoveryNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)
	requireNotNotified(t, registrationNotifier.Ch)

	// update the agent recovery token which should send a notification to the agent recovery notifier.
	require.True(t, s.UpdateAgentRecoveryToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotifiedOnce(t, agentRecoveryNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)
	requireNotNotified(t, registrationNotifier.Ch)

	// update the replication token which should send a notification to the replication notifier.
	require.True(t, s.UpdateReplicationToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRecoveryNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier2.Ch)
	requireNotNotified(t, registrationNotifier.Ch)

	s.StopNotify(replicationNotifier2)

	// update the replication token which should send a notification to the replication notifier.
	require.True(t, s.UpdateReplicationToken("eb0b56b9-fa65-4ae1-902a-c64457c62ac6", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotifiedOnce(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRecoveryNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)
	requireNotNotified(t, registrationNotifier.Ch)

	// update the config file registration token which should send a notification to the replication notifier.
	require.True(t, s.UpdateConfigFileRegistrationToken("82fe7362-7d83-4f43-bb27-c35f1f15083c", TokenSourceAPI))

	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRecoveryNotifier.Ch)
	requireNotNotified(t, replicationNotifier2.Ch)
	requireNotifiedOnce(t, registrationNotifier.Ch)

	// request updates that are not changes
	require.False(t, s.UpdateAgentToken("5d748ec2-d536-461f-8e2a-1f7eae98d559", TokenSourceAPI))
	require.False(t, s.UpdateAgentRecoveryToken("789badc8-f850-43e1-8742-9b9f484957cc", TokenSourceAPI))
	require.False(t, s.UpdateUserToken("47788919-f944-476a-bda5-446d64be1df8", TokenSourceAPI))
	require.False(t, s.UpdateReplicationToken("eb0b56b9-fa65-4ae1-902a-c64457c62ac6", TokenSourceAPI))
	require.False(t, s.UpdateConfigFileRegistrationToken("82fe7362-7d83-4f43-bb27-c35f1f15083c", TokenSourceAPI))

	// ensure that notifications were not sent
	requireNotNotified(t, agentNotifier.Ch)
	requireNotNotified(t, userNotifier.Ch)
	requireNotNotified(t, replicationNotifier.Ch)
	requireNotNotified(t, agentRecoveryNotifier.Ch)
	requireNotNotified(t, registrationNotifier.Ch)
}

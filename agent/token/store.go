package token

import (
	"sync"

	"crypto/subtle"
)

type TokenSource bool

const (
	TokenSourceConfig TokenSource = false
	TokenSourceAPI    TokenSource = true
)

type TokenKind int

const (
	TokenKindAgent TokenKind = iota
	TokenKindAgentRecovery
	TokenKindUser
	TokenKindReplication
)

type watcher struct {
	kind TokenKind
	ch   chan<- struct{}
}

// Notifier holds the channel used to notify a watcher
// of token updates as well as some internal tracking
// information to allow for deregistering the notifier.
type Notifier struct {
	id int
	Ch <-chan struct{}
}

// Store is used to hold the special ACL tokens used by Consul agents. It is
// designed to update the tokens on the fly, so the token store itself should be
// plumbed around and used to get tokens at runtime, don't save the resulting
// tokens.
type Store struct {
	// l synchronizes access to the token store.
	l sync.RWMutex

	// userToken is passed along for requests when the user didn't supply a
	// token, and may be left blank to use the anonymous token. This will
	// also be used for agent operations if the agent token isn't set.
	userToken string

	// userTokenSource indicates where this token originated from
	userTokenSource TokenSource

	// agentToken is used for internal agent operations like self-registering
	// with the catalog and anti-entropy, but should never be used for
	// user-initiated operations.
	agentToken string

	// agentTokenSource indicates where this token originated from
	agentTokenSource TokenSource

	// agentRecoveryToken is a special token that's only used locally for
	// access to the /v1/agent utility operations if the servers aren't
	// available.
	agentRecoveryToken string

	// agentRecoveryTokenSource indicates where this token originated from
	agentRecoveryTokenSource TokenSource

	// replicationToken is a special token that's used by servers to
	// replicate data from the primary datacenter.
	replicationToken string

	// replicationTokenSource indicates where this token originated from
	replicationTokenSource TokenSource

	watchers     map[int]watcher
	watcherIndex int

	persistence *fileStore
	// persistenceLock is used to synchronize access to the persisted token store
	// within the data directory. This will prevent loading while writing as well as
	// multiple concurrent writes.
	persistenceLock sync.RWMutex

	// enterpriseTokens contains tokens only used in consul-enterprise
	enterpriseTokens
}

// Notify will set up a watch for when tokens of the desired kind is changed
func (t *Store) Notify(kind TokenKind) Notifier {
	// buffering ensures that notifications aren't missed if the watcher
	// isn't already in a select and that our notifications don't
	// block returning from the Update* methods.
	ch := make(chan struct{}, 1)

	w := watcher{
		kind: kind,
		ch:   ch,
	}

	t.l.Lock()
	defer t.l.Unlock()
	if t.watchers == nil {
		t.watchers = make(map[int]watcher)
	}
	// we specifically want to avoid the zero-value to prevent accidental stop-notification requests
	t.watcherIndex += 1
	t.watchers[t.watcherIndex] = w

	return Notifier{id: t.watcherIndex, Ch: ch}
}

// StopNotify stops the token store from sending notifications to the specified notifiers chan
func (t *Store) StopNotify(n Notifier) {
	t.l.Lock()
	defer t.l.Unlock()
	delete(t.watchers, n.id)
}

// anyKindAllowed returns true if any of the kinds in the `check` list are
// set to be allowed in the `allowed` map.
//
// Note: this is mostly just a convenience to simplify the code in
// sendNotificationLocked and prevent more nested looping with breaks/continues
// and other state tracking.
func anyKindAllowed(allowed TokenKind, check []TokenKind) bool {
	for _, kind := range check {
		if allowed == kind {
			return true
		}
	}
	return false
}

// sendNotificationLocked will iterate through all watchers and notify them that a
// token they are watching has been updated.
//
// NOTE: this function explicitly does not attempt to send the kind or new token value
// along through the channel. With that approach watchers could potentially miss updates
// if the buffered chan fills up. Instead with this approach we just notify that any
// token they care about has been udpated and its up to the caller to retrieve the
// new value (after receiving from the chan). With this approach its entirely possible
// for the watcher to be notified twice before actually retrieving the token after the first
// read from the chan. This is better behavior than missing events. It can cause some
// churn temporarily but in common cases its not expected that these tokens would be updated
// frequently enough to cause this to happen.
func (t *Store) sendNotificationLocked(kinds ...TokenKind) {
	for _, watcher := range t.watchers {
		if !anyKindAllowed(watcher.kind, kinds) {
			// ignore this watcher as it doesn't want events for these kinds of token
			continue
		}

		select {
		case watcher.ch <- struct{}{}:
		default:
			// its already pending a notification
		}
	}
}

// UpdateUserToken replaces the current user token in the store.
// Returns true if it was changed.
func (t *Store) UpdateUserToken(token string, source TokenSource) bool {
	t.l.Lock()
	changed := t.userToken != token || t.userTokenSource != source
	t.userToken = token
	t.userTokenSource = source
	if changed {
		t.sendNotificationLocked(TokenKindUser)
	}
	t.l.Unlock()
	return changed
}

// UpdateAgentToken replaces the current agent token in the store.
// Returns true if it was changed.
func (t *Store) UpdateAgentToken(token string, source TokenSource) bool {
	t.l.Lock()
	changed := t.agentToken != token || t.agentTokenSource != source
	t.agentToken = token
	t.agentTokenSource = source
	if changed {
		t.sendNotificationLocked(TokenKindAgent)
	}
	t.l.Unlock()
	return changed
}

// UpdateAgentRecoveryToken replaces the current agent recovery token in the store.
// Returns true if it was changed.
func (t *Store) UpdateAgentRecoveryToken(token string, source TokenSource) bool {
	t.l.Lock()
	changed := t.agentRecoveryToken != token || t.agentRecoveryTokenSource != source
	t.agentRecoveryToken = token
	t.agentRecoveryTokenSource = source
	if changed {
		t.sendNotificationLocked(TokenKindAgentRecovery)
	}
	t.l.Unlock()
	return changed
}

// UpdateReplicationToken replaces the current replication token in the store.
// Returns true if it was changed.
func (t *Store) UpdateReplicationToken(token string, source TokenSource) bool {
	t.l.Lock()
	changed := t.replicationToken != token || t.replicationTokenSource != source
	t.replicationToken = token
	t.replicationTokenSource = source
	if changed {
		t.sendNotificationLocked(TokenKindReplication)
	}
	t.l.Unlock()
	return changed
}

// UserToken returns the best token to use for user operations.
func (t *Store) UserToken() string {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.userToken
}

// AgentToken returns the best token to use for internal agent operations.
func (t *Store) AgentToken() string {
	t.l.RLock()
	defer t.l.RUnlock()

	if tok := t.enterpriseAgentToken(); tok != "" {
		return tok
	}

	if t.agentToken != "" {
		return t.agentToken
	}
	return t.userToken
}

func (t *Store) AgentRecoveryToken() string {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.agentRecoveryToken
}

// ReplicationToken returns the replication token.
func (t *Store) ReplicationToken() string {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.replicationToken
}

// UserToken returns the best token to use for user operations.
func (t *Store) UserTokenAndSource() (string, TokenSource) {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.userToken, t.userTokenSource
}

// AgentToken returns the best token to use for internal agent operations.
func (t *Store) AgentTokenAndSource() (string, TokenSource) {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.agentToken, t.agentTokenSource
}

func (t *Store) AgentRecoveryTokenAndSource() (string, TokenSource) {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.agentRecoveryToken, t.agentRecoveryTokenSource
}

// ReplicationToken returns the replication token.
func (t *Store) ReplicationTokenAndSource() (string, TokenSource) {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.replicationToken, t.replicationTokenSource
}

// IsAgentRecoveryToken checks to see if a given token is the agent recovery token.
// This will never match an empty token for safety.
func (t *Store) IsAgentRecoveryToken(token string) bool {
	t.l.RLock()
	defer t.l.RUnlock()

	return (token != "") && (subtle.ConstantTimeCompare([]byte(token), []byte(t.agentRecoveryToken)) == 1)
}

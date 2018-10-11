package token

import (
	"sync"
)

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

	// agentToken is used for internal agent operations like self-registering
	// with the catalog and anti-entropy, but should never be used for
	// user-initiated operations.
	agentToken string

	// agentMasterToken is a special token that's only used locally for
	// access to the /v1/agent utility operations if the servers aren't
	// available.
	agentMasterToken string

	// aclReplicationToken is a special token that's used by servers to
	// replicate ACLs from the ACL datacenter.
	aclReplicationToken string
}

// UpdateUserToken replaces the current user token in the store.
func (t *Store) UpdateUserToken(token string) {
	t.l.Lock()
	t.userToken = token
	t.l.Unlock()
}

// UpdateAgentToken replaces the current agent token in the store.
func (t *Store) UpdateAgentToken(token string) {
	t.l.Lock()
	t.agentToken = token
	t.l.Unlock()
}

// UpdateAgentMasterToken replaces the current agent master token in the store.
func (t *Store) UpdateAgentMasterToken(token string) {
	t.l.Lock()
	t.agentMasterToken = token
	t.l.Unlock()
}

// UpdateACLReplicationToken replaces the current ACL replication token in the store.
func (t *Store) UpdateACLReplicationToken(token string) {
	t.l.Lock()
	t.aclReplicationToken = token
	t.l.Unlock()
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

	if t.agentToken != "" {
		return t.agentToken
	}
	return t.userToken
}

// ACLReplicationToken returns the ACL replication token.
func (t *Store) ACLReplicationToken() string {
	t.l.RLock()
	defer t.l.RUnlock()

	return t.aclReplicationToken
}

// IsAgentMasterToken checks to see if a given token is the agent master token.
// This will never match an empty token for safety.
func (t *Store) IsAgentMasterToken(token string) bool {
	t.l.RLock()
	defer t.l.RUnlock()

	return (token != "") && (token == t.agentMasterToken)
}

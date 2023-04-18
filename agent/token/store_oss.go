//go:build !consulent
// +build !consulent

package token

type EnterpriseConfig struct {
}

// Stub for enterpriseTokens
type enterpriseTokens struct {
}

// enterpriseAgentToken OSS stub
func (t *Store) enterpriseAgentToken() string {
	return ""
}

// loadEnterpriseTokens is a noop stub for the func defined agent_ent.go
func loadEnterpriseTokens(_ *Store, _ Config) {
}

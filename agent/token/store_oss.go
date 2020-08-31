// +build !consulent

package token

// Stub for enterpriseTokens
type enterpriseTokens struct {
}

// enterpriseAgentToken OSS stub
func (s *Store) enterpriseAgentToken() string {
	return ""
}

// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package token

type EnterpriseConfig struct {
}

// Stub for enterpriseTokens
type enterpriseTokens struct {
}

// enterpriseAgentToken CE stub
func (t *Store) enterpriseAgentToken() string {
	return ""
}

// loadEnterpriseTokens is a noop stub for the func defined agent_ent.go
func loadEnterpriseTokens(_ *Store, _ Config) {
}

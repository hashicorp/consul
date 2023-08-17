// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

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

//go:build !consulent
// +build !consulent

package agent

// enterpriseDelegate has no functions in CE
type enterpriseDelegate interface{}

//go:build !consulent
// +build !consulent

package autoconf

import (
	"testing"
)

// mockedEnterpriseConfig is pretty much just a stub in CE.
// It does contain an enterprise config for compatibility
// purposes but that in and of itself is just a stub.
type mockedEnterpriseConfig struct {
	EnterpriseConfig
}

func newMockedEnterpriseConfig(t *testing.T) *mockedEnterpriseConfig {
	return &mockedEnterpriseConfig{}
}

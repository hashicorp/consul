//go:build !consulent
// +build !consulent

package autoconf

import (
	"testing"
)

func newEnterpriseConfig(t *testing.T) EnterpriseConfig {
	return EnterpriseConfig{}
}

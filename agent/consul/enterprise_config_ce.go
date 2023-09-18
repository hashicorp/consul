//go:build !consulent
// +build !consulent

package consul

type EnterpriseConfig struct{}

func DefaultEnterpriseConfig() *EnterpriseConfig {
	return nil
}

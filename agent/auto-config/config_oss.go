//go:build !consulent
// +build !consulent

package autoconf

// EnterpriseConfig stub - only populated in Consul Enterprise
type EnterpriseConfig struct{}

// finalize is a noop for OSS
func (_ *EnterpriseConfig) validateAndFinalize() error {
	return nil
}

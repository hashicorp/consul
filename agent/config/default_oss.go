//go:build !consulent
// +build !consulent

package config

// DefaultEnterpriseSource returns the consul agent configuration for enterprise mode.
// These can be overridden by the user and therefore this source should be merged in the
// head and processed before user configuration.
func DefaultEnterpriseSource() Source {
	return LiteralSource{Name: "enterprise-defaults"}
}

// OverrideEnterpriseSource returns the consul agent configuration for the enterprise mode.
// This should be merged in the tail after the DefaultConsulSource.
func OverrideEnterpriseSource() Source {
	return LiteralSource{Name: "enterprise-overrides"}
}

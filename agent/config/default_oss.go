// +build !consulent

package config

// DefaultEnterpriseSource returns the consul agent configuration for enterprise mode.
// These can be overridden by the user and therefore this source should be merged in the
// head and processed before user configuration.
func DefaultEnterpriseSource() Source {
	return Source{
		Name:   "enterprise-defaults",
		Format: "hcl",
		Data:   ``,
	}
}

// OverrideEnterpriseSource returns the consul agent configuration for the enterprise mode.
// This should be merged in the tail after the DefaultConsulSource.
func OverrideEnterpriseSource() Source {
	return Source{
		Name:   "enterprise-overrides",
		Format: "hcl",
		Data:   ``,
	}
}

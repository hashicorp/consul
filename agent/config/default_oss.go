// +build !consulent

package config

// DefaultEnterpriseSource returns the consul agent configuration for enterprise mode.
// These can be overridden by the user and therefore this source should be merged in the
// head and processed before user configuration.
// TODO: return a rawSource (no decoding) instead of a FileSource
func DefaultEnterpriseSource() Source {
	return FileSource{
		Name:   "enterprise-defaults",
		Format: "hcl",
		Data:   ``,
	}
}

// OverrideEnterpriseSource returns the consul agent configuration for the enterprise mode.
// This should be merged in the tail after the DefaultConsulSource.
// TODO: return a rawSource (no decoding) instead of a FileSource
func OverrideEnterpriseSource() Source {
	return FileSource{
		Name:   "enterprise-overrides",
		Format: "hcl",
		Data:   ``,
	}
}

// +build !ent

package config

// DefaultEnterpriseSource returns the consul agent configuration for the enterprise mode.
// This should be merged in the tail after the DefaultConsulSource.
func DefaultEnterpriseSource() Source {
	return Source{
		Name:   "enterprise",
		Format: "hcl",
		Data:   ``,
	}
}

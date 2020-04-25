// +build !consulent

package config

func (_ *Builder) BuildEnterpriseRuntimeConfig(_ *Config) (EnterpriseRuntimeConfig, error) {
	return EnterpriseRuntimeConfig{}, nil
}

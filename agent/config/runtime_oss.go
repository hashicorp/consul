// +build !consulent

package config

type EnterpriseRuntimeConfig struct{}

func (c *RuntimeConfig) PartitionOrEmpty() string { return "" }

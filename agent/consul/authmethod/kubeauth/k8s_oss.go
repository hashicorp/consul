//go:build !consulent
// +build !consulent

package kubeauth

import "github.com/hashicorp/consul/agent/structs"

type enterpriseConfig struct{}

func enterpriseValidation(method *structs.ACLAuthMethod, config *Config) error {
	return nil
}

func (v *Validator) k8sEntMetaFromFields(fields map[string]string) *structs.EnterpriseMeta {
	return nil
}

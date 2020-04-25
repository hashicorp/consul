// +build !consulent

package kubeauth

import "github.com/hashicorp/consul/agent/structs"

type enterpriseConfig struct{}

func (v *Validator) k8sEntMetaFromFields(fields map[string]string) *structs.EnterpriseMeta {
	return structs.DefaultEnterpriseMeta()
}

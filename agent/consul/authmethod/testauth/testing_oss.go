//go:build !consulent
// +build !consulent

package testauth

import (
	"github.com/hashicorp/consul/agent/structs"
)

type enterpriseConfig struct{}

func (v *Validator) testAuthEntMetaFromFields(fields map[string]string) *structs.EnterpriseMeta {
	return nil
}

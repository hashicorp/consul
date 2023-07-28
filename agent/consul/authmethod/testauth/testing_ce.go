//go:build !consulent
// +build !consulent

package testauth

import "github.com/hashicorp/consul/acl"

type enterpriseConfig struct{}

func (v *Validator) testAuthEntMetaFromFields(fields map[string]string) *acl.EnterpriseMeta {
	return nil
}

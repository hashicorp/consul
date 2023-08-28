//go:build !consulent
// +build !consulent

package catalog

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func GetEnterpriseMetaFromResourceID(id *pbresource.ID) *acl.EnterpriseMeta {
	return acl.DefaultEnterpriseMeta()
}

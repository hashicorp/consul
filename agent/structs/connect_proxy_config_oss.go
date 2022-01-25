//go:build !consulent
// +build !consulent

package structs

func (us *Upstream) GetEnterpriseMeta() *EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (us *Upstream) DestinationID() ServiceID {
	return ServiceID{
		ID: us.DestinationName,
	}
}

func (us *Upstream) enterpriseStringPrefix() string {
	return ""
}

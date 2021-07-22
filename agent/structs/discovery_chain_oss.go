// +build !consulent

package structs

func (t *DiscoveryTarget) GetEnterpriseMetadata() *EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

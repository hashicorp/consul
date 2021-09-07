// +build !consulent

package acl

const DefaultPartitionName = ""

type EnterpriseConfig struct {
	// no fields in OSS
}

func (_ *EnterpriseConfig) Close() {
	// do nothing
}

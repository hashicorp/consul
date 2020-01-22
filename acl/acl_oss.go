// +build !consulent

package acl

type EnterpriseConfig struct {
	// no fields in OSS
}

func (_ *EnterpriseConfig) Close() {
	// do nothing
}

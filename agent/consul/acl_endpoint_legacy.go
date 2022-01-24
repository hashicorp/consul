package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

type LegacyACLGetPolicy struct{}

func (a *ACL) GetPolicy(*LegacyACLGetPolicy, *LegacyACLGetPolicy) error {
	return fmt.Errorf("ACL.GetPolicy: the legacy ACL system has been removed")
}

func (a *ACL) Bootstrap(*structs.DCSpecificRequest, *LegacyACLRequest) error {
	return fmt.Errorf("ACL.Bootstrap: the legacy ACL system has been removed")
}

type LegacyACLRequest struct{}

func (a *ACL) Apply(*LegacyACLRequest, *string) error {
	return fmt.Errorf("ACL.Apply: the legacy ACL system has been removed")
}

func (a *ACL) Get(*LegacyACLRequest, *LegacyACLRequest) error {
	return fmt.Errorf("ACL.Get: the legacy ACL system has been removed")
}

func (a *ACL) List(*structs.DCSpecificRequest, *LegacyACLRequest) error {
	return fmt.Errorf("ACL.List: the legacy ACL system has been removed")
}

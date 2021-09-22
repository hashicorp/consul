package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
)

func (a *ACL) Bootstrap(*structs.DCSpecificRequest, *structs.ACL) error {
	return fmt.Errorf("ACL.Bootstrap: the legacy ACL system has been removed")
}

type LegacyACLRequest struct{}

func (a *ACL) Apply(*LegacyACLRequest, *string) error {
	return fmt.Errorf("ACL.Apply: the legacy ACL system has been removed")
}

func (a *ACL) Get(*structs.ACLSpecificRequest, *structs.IndexedACLs) error {
	return fmt.Errorf("ACL.Get: the legacy ACL system has been removed")
}

func (a *ACL) List(*structs.DCSpecificRequest, *structs.IndexedACLs) error {
	return fmt.Errorf("ACL.List: the legacy ACL system has been removed")
}

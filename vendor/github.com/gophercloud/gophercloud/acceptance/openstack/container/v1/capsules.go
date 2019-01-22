package v1

import (
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	"github.com/gophercloud/gophercloud/openstack/container/v1/capsules"
)

// WaitForCapsuleStatus will poll a capsule's status until it either matches
// the specified status or the status becomes Failed.
func WaitForCapsuleStatus(client *gophercloud.ServiceClient, capsule *capsules.Capsule, status string) error {
	return tools.WaitFor(func() (bool, error) {
		latest, err := capsules.Get(client, capsule.UUID).Extract()
		if err != nil {
			return false, err
		}

		if latest.Status == status {
			// Success!
			return true, nil
		}

		if latest.Status == "Failed" {
			return false, fmt.Errorf("Capsule in FAILED state")
		}

		return false, nil
	})
}

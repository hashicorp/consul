package workflows

import (
	"github.com/gophercloud/gophercloud"
)

func createURL(client *gophercloud.ServiceClient) string {
	return client.ServiceURL("workflows")
}

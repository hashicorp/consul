package clusters

import (
	"github.com/gophercloud/gophercloud"
)

var apiName = "clusters"

func commonURL(client *gophercloud.ServiceClient) string {
	return client.ServiceURL(apiName)
}

func createURL(client *gophercloud.ServiceClient) string {
	return commonURL(client)
}

func getURL(c *gophercloud.ServiceClient, id string) string {
	return c.ServiceURL("clusters", id)
}

func listURL(client *gophercloud.ServiceClient) string {
	return client.ServiceURL("clusters")
}

package keypairs

import "github.com/gophercloud/gophercloud"

const resourcePath = "os-keypairs"

func resourceURL(c *gophercloud.ServiceClient) string {
	return c.ServiceURL(resourcePath)
}

func listURL(c *gophercloud.ServiceClient) string {
	return resourceURL(c)
}

func createURL(c *gophercloud.ServiceClient) string {
	return resourceURL(c)
}

func getURL(c *gophercloud.ServiceClient, name string) string {
	return c.ServiceURL(resourcePath, name)
}

func deleteURL(c *gophercloud.ServiceClient, name string) string {
	return getURL(c, name)
}

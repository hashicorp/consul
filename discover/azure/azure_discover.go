// Package azure provides node discovery for Microsoft Azure.
package azure

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
)

// Discover returns the private ip addresses of all Azure instances in a
// subscription with a certain tag name and value.
//
// cfg supports the following fields:
//
//  "tenant_id"         : The id of the tenant
//  "client_id"         : The id of the client
//  "subscription_id"   : The id of the subscription
//  "secret_access_key" : The authentication credential
//  "tag_name"          : The name of the tag to filter on
//  "tag_value"         : The value of the tag to filter on
//
func Discover(cfg map[string]string, l *log.Logger) ([]string, error) {

	// Only works for the Azure PublicCLoud for now; no ability to test other Environment
	tenantID := cfg["tenant_id"]
	oauthConfig, err := azure.PublicCloud.OAuthConfigForTenant(tenantID)
	if err != nil {
		return nil, err
	}

	// Get the ServicePrincipalToken for use searching the NetworkInterfaces
	clientID, secretKey := cfg["client_id"], cfg["secret_access_key"]
	sbt, err := azure.NewServicePrincipalToken(*oauthConfig, clientID, secretKey, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	// Setup the client using autorest; followed the structure from Terraform
	subscriptionID := cfg["subscription_id"]
	vmnet := network.NewInterfacesClient(subscriptionID)
	vmnet.Client.UserAgent = fmt.Sprint("Hashicorp-Consul")
	vmnet.Sender = autorest.CreateSender(autorest.WithLogging(l))
	vmnet.Authorizer = sbt

	// Get all network interfaces across resource groups
	// unless there is a compelling reason to restrict
	netres, err := vmnet.ListAll()
	if err != nil {
		return nil, err
	}

	// For now, ignore Primary interfaces, choose any PrivateIPAddress with the matching tags
	tagName, tagValue := cfg["tag_name"], cfg["tag_value"]
	var addrs []string
	for _, v := range *netres.Value {
		if v.Tags == nil {
			continue
		}
		if *(*v.Tags)[tagName] != tagValue {
			continue
		}
		if v.IPConfigurations == nil {
			continue
		}
		for _, x := range *v.IPConfigurations {
			if x.PrivateIPAddress == nil {
				continue
			}
			addrs = append(addrs, *x.PrivateIPAddress)
		}
	}
	return addrs, nil
}

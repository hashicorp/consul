// Package azure provides node discovery for Microsoft Azure.
package azure

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/hashicorp/go-discover/config"
)

// Discover returns the private ip addresses of all Azure instances in a
// subscription with a certain tag name and value. Only Azure public cloud
// is supported.
//
// cfg contains the configuration in "key=val key=val ..." format. The
// values are URL encoded.
//
// The supported keys are:
//
//  tenant_id         : The id of the tenant
//  client_id         : The id of the client
//  subscription_id   : The id of the subscription
//  secret_access_key : The authentication credential
//  tag_name          : The name of the tag to filter on
//  tag_value         : The value of the tag to filter on
//
// Example:
//
//  tenant_id=xxx client_id=xxx subscription_id=xxx secret_access_key=xxx tag_name=consul tag_value=xxx
//
func Discover(cfg string, l *log.Logger) ([]string, error) {
	m, err := config.Parse(cfg)
	if err != nil {
		return nil, err
	}

	tenantID := m["tenant_id"]
	clientID := m["client_id"]
	subscriptionID := m["subscription_id"]
	secretKey := m["secret_access_key"]
	tagName := m["tag_name"]
	tagValue := m["tag_value"]

	// Only works for the Azure PublicCLoud for now; no ability to test other Environment
	oauthConfig, err := azure.PublicCloud.OAuthConfigForTenant(tenantID)
	if err != nil {
		return nil, err
	}

	// Get the ServicePrincipalToken for use searching the NetworkInterfaces
	sbt, err := azure.NewServicePrincipalToken(*oauthConfig, clientID, secretKey, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	// Setup the client using autorest; followed the structure from Terraform
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

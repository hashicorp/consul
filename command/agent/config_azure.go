package agent

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
)

// discoverAzureHosts searches an Azure Subscription, returning a list of instance ips
// where AzureTag_Name = AzureTag_Value
func (c *Config) discoverAzureHosts(logger *log.Logger) ([]string, error) {
	var servers []string
	// Only works for the Azure PublicCLoud for now; no ability to test other Environment
	oauthConfig, err := azure.PublicCloud.OAuthConfigForTenant(c.RetryJoinAzure.TenantID)
	if err != nil {
		return nil, err
	}
	// Get the ServicePrincipalToken for use searching the NetworkInterfaces
	sbt, tokerr := azure.NewServicePrincipalToken(*oauthConfig,
		c.RetryJoinAzure.ClientID,
		c.RetryJoinAzure.SecretAccessKey,
		azure.PublicCloud.ResourceManagerEndpoint,
	)
	if tokerr != nil {
		return nil, tokerr
	}
	// Setup the client using autorest; followed the structure from Terraform
	vmnet := network.NewInterfacesClient(c.RetryJoinAzure.SubscriptionID)
	vmnet.Client.UserAgent = fmt.Sprint("Hashicorp-Consul")
	vmnet.Authorizer = sbt
	vmnet.Sender = autorest.CreateSender(autorest.WithLogging(logger))
	// Get all Network interfaces across ResourceGroups unless there is a compelling reason to restrict
	netres, neterr := vmnet.ListAll()
	if neterr != nil {
		return nil, neterr
	}
	// For now, ignore Primary interfaces, choose any PrivateIPAddress with the matching tags
	for _, oneint := range *netres.Value {
		// Make it a little more robust just in case there is actually no Tags
		if oneint.Tags != nil {
			if *(*oneint.Tags)[c.RetryJoinAzure.TagName] == c.RetryJoinAzure.TagValue {
				// Make it a little more robust just in case IPConfigurations nil
				if oneint.IPConfigurations != nil {
					for _, onecfg := range *oneint.IPConfigurations {
						// fmt.Println("Internal FQDN: ", *onecfg.Name, " IP: ", *onecfg.PrivateIPAddress)
						// Only get the address if there is private IP address
						if onecfg.PrivateIPAddress != nil {
							servers = append(servers, *onecfg.PrivateIPAddress)
						}
					}
				}
			}
		}
	}
	return servers, nil
}

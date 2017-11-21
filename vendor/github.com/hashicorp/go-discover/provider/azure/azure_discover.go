// Package azure provides node discovery for Microsoft Azure.
package azure

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
)

type Provider struct{}

func (p *Provider) Help() string {
	return `Microsoft Azure:

   provider:          "azure"
   tenant_id:         The id of the tenant
   client_id:         The id of the client
   subscription_id:   The id of the subscription
   secret_access_key: The authentication credential
   tag_name:          The name of the tag to filter on
   tag_value:         The value of the tag to filter on

   The only permission needed is the 'ListAll' method for 'NetworkInterfaces'.
   It is recommended you make a dedicated key used only for auto-joining.
`
}

func (p *Provider) Addrs(args map[string]string, l *log.Logger) ([]string, error) {
	if args["provider"] != "azure" {
		return nil, fmt.Errorf("discover-azure: invalid provider " + args["provider"])
	}

	if l == nil {
		l = log.New(ioutil.Discard, "", 0)
	}

	tenantID := args["tenant_id"]
	clientID := args["client_id"]
	subscriptionID := args["subscription_id"]
	secretKey := args["secret_access_key"]
	tagName := args["tag_name"]
	tagValue := args["tag_value"]

	// Only works for the Azure PublicCLoud for now; no ability to test other Environment
	oauthConfig, err := azure.PublicCloud.OAuthConfigForTenant(tenantID)
	if err != nil {
		return nil, fmt.Errorf("discover-azure: %s", err)
	}

	// Get the ServicePrincipalToken for use searching the NetworkInterfaces
	sbt, err := azure.NewServicePrincipalToken(*oauthConfig, clientID, secretKey, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("discover-azure: %s", err)
	}

	// Setup the client using autorest; followed the structure from Terraform
	vmnet := network.NewInterfacesClient(subscriptionID)
	vmnet.Client.UserAgent = "Hashicorp-Consul"
	vmnet.Sender = autorest.CreateSender(autorest.WithLogging(l))
	vmnet.Authorizer = sbt

	// Get all network interfaces across resource groups
	// unless there is a compelling reason to restrict
	netres, err := vmnet.ListAll()
	if err != nil {
		return nil, fmt.Errorf("discover-azure: %s", err)
	}

	if netres.Value == nil {
		return nil, fmt.Errorf("discover-azure: no interfaces")
	}

	// Choose any PrivateIPAddress with the matching tag
	var addrs []string
	for _, v := range *netres.Value {
		if v.Tags == nil {
			continue
		}
		tv := (*v.Tags)[tagName] // *string
		if tv == nil || *tv != tagValue {
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

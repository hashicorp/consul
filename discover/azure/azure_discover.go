package azure

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/arm/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
)

type Config struct {
	TagName         string
	TagValue        string
	SubscriptionID  string
	TenantID        string
	ClientID        string
	SecretAccessKey string
}

// Discover returns the ip addresses of all Azure instances in a
// subscription where TagName == TagValue.
func Discover(c *Config, l *log.Logger) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("[ERR] discover-azure: Missing configuration")
	}

	// Only works for the Azure PublicCLoud for now; no ability to test other Environment
	oauthConfig, err := azure.PublicCloud.OAuthConfigForTenant(c.TenantID)
	if err != nil {
		return nil, err
	}

	// Get the ServicePrincipalToken for use searching the NetworkInterfaces
	sbt, err := azure.NewServicePrincipalToken(*oauthConfig, c.ClientID, c.SecretAccessKey, azure.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	// Setup the client using autorest; followed the structure from Terraform
	vmnet := network.NewInterfacesClient(c.SubscriptionID)
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
		if *(*v.Tags)[c.TagName] != c.TagValue {
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

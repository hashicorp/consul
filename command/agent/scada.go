package agent

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/hashicorp/scada-client"
)

const (
	// providerService is the service name we use
	providerService = "consul"

	// resourceType is the type of resource we represent
	// when connecting to SCADA
	resourceType = "infrastructures"
)

// ProviderService returns the service information for the provider
func ProviderService(c *Config) *client.ProviderService {
	return &client.ProviderService{
		Service:        providerService,
		ServiceVersion: fmt.Sprintf("%s%s", c.Version, c.VersionPrerelease),
		Capabilities: map[string]int{
			"http": 1,
		},
		Meta: map[string]string{
			"server":     strconv.FormatBool(c.Server),
			"datacenter": c.Datacenter,
		},
		ResourceType: resourceType,
	}
}

// ProviderConfig returns the configuration for the SCADA provider
func ProviderConfig(c *Config) *client.ProviderConfig {
	return &client.ProviderConfig{
		Service: ProviderService(c),
		Handlers: map[string]client.CapabilityProvider{
			"http": nil,
		},
		ResourceGroup: c.AtlasCluster,
		Token:         c.AtlasToken,
	}
}

// NewProvider creates a new SCADA provider using the
// given configuration. Requests are routed to the
func NewProvider(c *Config, logOutput io.Writer) (*client.Provider, net.Listener, error) {
	// Get the configuration of the provider
	config := ProviderConfig(c)
	config.Logger = log.New(logOutput, "", log.LstdFlags)

	// TODO: REMOVE
	config.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	// TODO: Setup the handlers
	config.Handlers["http"] = nil

	// Create the provider
	provider, err := client.NewProvider(config)
	if err != nil {
		return nil, nil, err
	}
	return provider, nil, nil
}

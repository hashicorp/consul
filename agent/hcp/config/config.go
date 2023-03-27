package config

import (
	"crypto/tls"

	hcpcfg "github.com/hashicorp/hcp-sdk-go/config"
)

// CloudConfig defines configuration for connecting to HCP services
type CloudConfig struct {
	ResourceID   string
	ClientID     string
	ClientSecret string
	Hostname     string
	AuthURL      string
	ScadaAddress string

	// internal
	ManagementToken string
}

func (c *CloudConfig) HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error) {
	if c.ClientID != "" && c.ClientSecret != "" {
		opts = append(opts, hcpcfg.WithClientCredentials(c.ClientID, c.ClientSecret))
	}
	if c.AuthURL != "" {
		opts = append(opts, hcpcfg.WithAuth(c.AuthURL, &tls.Config{}))
	}
	if c.Hostname != "" {
		opts = append(opts, hcpcfg.WithAPI(c.Hostname, &tls.Config{}))
	}
	if c.ScadaAddress != "" {
		opts = append(opts, hcpcfg.WithSCADA(c.ScadaAddress, &tls.Config{}))
	}
	opts = append(opts, hcpcfg.FromEnv())
	return hcpcfg.NewHCPConfig(opts...)
}

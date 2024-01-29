// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"crypto/tls"

	"github.com/hashicorp/consul/types"
	hcpcfg "github.com/hashicorp/hcp-sdk-go/config"
	"github.com/hashicorp/hcp-sdk-go/resource"
)

// CloudConfigurer abstracts the cloud config methods needed to connect to HCP
// in an interface for easier testing.
type CloudConfigurer interface {
	HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error)
	Resource() (resource.Resource, error)
}

// CloudConfig defines configuration for connecting to HCP services
type CloudConfig struct {
	ResourceID   string
	ClientID     string
	ClientSecret string
	Hostname     string
	AuthURL      string
	ScadaAddress string

	// Management token used by HCP management plane.
	// Cannot be set via config files.
	ManagementToken string

	// TlsConfig for testing.
	TLSConfig *tls.Config

	NodeID   types.NodeID
	NodeName string
}

func (c *CloudConfig) WithTLSConfig(cfg *tls.Config) {
	c.TLSConfig = cfg
}

func (c *CloudConfig) Resource() (resource.Resource, error) {
	return resource.FromString(c.ResourceID)
}

func (c *CloudConfig) HCPConfig(opts ...hcpcfg.HCPConfigOption) (hcpcfg.HCPConfig, error) {
	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{}
	}
	if c.ClientID != "" && c.ClientSecret != "" {
		opts = append(opts, hcpcfg.WithClientCredentials(c.ClientID, c.ClientSecret))
	}
	if c.AuthURL != "" {
		opts = append(opts, hcpcfg.WithAuth(c.AuthURL, c.TLSConfig))
	}
	if c.Hostname != "" {
		opts = append(opts, hcpcfg.WithAPI(c.Hostname, c.TLSConfig))
	}
	if c.ScadaAddress != "" {
		opts = append(opts, hcpcfg.WithSCADA(c.ScadaAddress, c.TLSConfig))
	}
	opts = append(opts, hcpcfg.FromEnv(), hcpcfg.WithoutBrowserLogin())
	return hcpcfg.NewHCPConfig(opts...)
}

// IsConfigured returns whether the cloud configuration has been set either
// in the configuration file or via environment variables.
func (c *CloudConfig) IsConfigured() bool {
	return c.ResourceID != "" && c.ClientID != "" && c.ClientSecret != ""
}

// Merge returns a cloud configuration that is the combined the values of
// two configurations.
func Merge(o CloudConfig, n CloudConfig) CloudConfig {
	c := o
	if n.ResourceID != "" {
		c.ResourceID = n.ResourceID
	}

	if n.ClientID != "" {
		c.ClientID = n.ClientID
	}

	if n.ClientSecret != "" {
		c.ClientSecret = n.ClientSecret
	}

	if n.Hostname != "" {
		c.Hostname = n.Hostname
	}

	if n.AuthURL != "" {
		c.AuthURL = n.AuthURL
	}

	if n.ScadaAddress != "" {
		c.ScadaAddress = n.ScadaAddress
	}

	if n.ManagementToken != "" {
		c.ManagementToken = n.ManagementToken
	}

	if n.TLSConfig != nil {
		c.TLSConfig = n.TLSConfig
	}

	if n.NodeID != "" {
		c.NodeID = n.NodeID
	}

	if n.NodeName != "" {
		c.NodeName = n.NodeName
	}

	return c
}

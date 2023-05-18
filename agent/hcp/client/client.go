// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"strconv"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	hcptelemetry "github.com/hashicorp/hcp-sdk-go/clients/cloud-consul-telemetry-gateway/preview/2023-04-14/client/consul_telemetry_service"
	hcpgnm "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/client/global_network_manager_service"
	gnmmod "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/models"
	"github.com/hashicorp/hcp-sdk-go/httpclient"
	"github.com/hashicorp/hcp-sdk-go/resource"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/version"
)

// Client interface exposes HCP operations that can be invoked by Consul
//
//go:generate mockery --name Client --with-expecter --inpackage
type Client interface {
	FetchBootstrap(ctx context.Context) (*BootstrapConfig, error)
	FetchTelemetryConfig(ctx context.Context) (*TelemetryConfig, error)
	PushServerStatus(ctx context.Context, status *ServerStatus) error
	DiscoverServers(ctx context.Context) ([]string, error)
}

// MetricsConfig holds metrics specific configuration for the TelemetryConfig.
// The endpoint field overrides the TelemetryConfig endpoint.
type MetricsConfig struct {
	Filters  []string
	Endpoint string
}

// TelemetryConfig contains configuration for telemetry data forwarded by Consul servers
// to the HCP Telemetry gateway.
type TelemetryConfig struct {
	Endpoint      string
	Labels        map[string]string
	MetricsConfig *MetricsConfig
}

type BootstrapConfig struct {
	Name            string
	BootstrapExpect int
	GossipKey       string
	TLSCert         string
	TLSCertKey      string
	TLSCAs          []string
	ConsulConfig    string
	ManagementToken string
}

type hcpClient struct {
	hc       *httptransport.Runtime
	cfg      config.CloudConfig
	gnm      hcpgnm.ClientService
	tgw      hcptelemetry.ClientService
	resource resource.Resource
}

func NewClient(cfg config.CloudConfig) (Client, error) {
	client := &hcpClient{
		cfg: cfg,
	}

	var err error
	client.resource, err = resource.FromString(cfg.ResourceID)
	if err != nil {
		return nil, err
	}

	client.hc, err = httpClient(cfg)
	if err != nil {
		return nil, err
	}

	client.gnm = hcpgnm.New(client.hc, nil)
	client.tgw = hcptelemetry.New(client.hc, nil)

	return client, nil
}

func httpClient(c config.CloudConfig) (*httptransport.Runtime, error) {
	cfg, err := c.HCPConfig()
	if err != nil {
		return nil, err
	}

	return httpclient.New(httpclient.Config{
		HCPConfig:     cfg,
		SourceChannel: "consul " + version.GetHumanVersion(),
	})
}

// FetchTelemetryConfig obtains telemetry configuration from the Telemetry Gateway.
func (c *hcpClient) FetchTelemetryConfig(ctx context.Context) (*TelemetryConfig, error) {
	params := hcptelemetry.NewAgentTelemetryConfigParamsWithContext(ctx).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project).
		WithClusterID(c.resource.ID)

	resp, err := c.tgw.AgentTelemetryConfig(params, nil)
	if err != nil {
		return nil, err
	}

	payloadConfig := resp.Payload.TelemetryConfig
	return &TelemetryConfig{
		Endpoint: payloadConfig.Endpoint,
		Labels:   payloadConfig.Labels,
		MetricsConfig: &MetricsConfig{
			Filters:  payloadConfig.Metrics.IncludeList,
			Endpoint: payloadConfig.Metrics.Endpoint,
		},
	}, nil
}

func (c *hcpClient) FetchBootstrap(ctx context.Context) (*BootstrapConfig, error) {
	version := version.GetHumanVersion()
	params := hcpgnm.NewAgentBootstrapConfigParamsWithContext(ctx).
		WithID(c.resource.ID).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project).
		WithConsulVersion(&version)

	resp, err := c.gnm.AgentBootstrapConfig(params, nil)
	if err != nil {
		return nil, err
	}

	return bootstrapConfigFromHCP(resp.Payload), nil
}

func bootstrapConfigFromHCP(res *gnmmod.HashicorpCloudGlobalNetworkManager20220215AgentBootstrapResponse) *BootstrapConfig {
	var serverTLS gnmmod.HashicorpCloudGlobalNetworkManager20220215ServerTLS
	if res.Bootstrap.ServerTLS != nil {
		serverTLS = *res.Bootstrap.ServerTLS
	}

	return &BootstrapConfig{
		Name:            res.Bootstrap.ID,
		BootstrapExpect: int(res.Bootstrap.BootstrapExpect),
		GossipKey:       res.Bootstrap.GossipKey,
		TLSCert:         serverTLS.Cert,
		TLSCertKey:      serverTLS.PrivateKey,
		TLSCAs:          serverTLS.CertificateAuthorities,
		ConsulConfig:    res.Bootstrap.ConsulConfig,
		ManagementToken: res.Bootstrap.ManagementToken,
	}
}

func (c *hcpClient) PushServerStatus(ctx context.Context, s *ServerStatus) error {
	params := hcpgnm.NewAgentPushServerStateParamsWithContext(ctx).
		WithID(c.resource.ID).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project)

	params.SetBody(hcpgnm.AgentPushServerStateBody{
		ServerState: serverStatusToHCP(s),
	})

	_, err := c.gnm.AgentPushServerState(params, nil)
	return err
}

type ServerStatus struct {
	ID         string
	Name       string
	Version    string
	LanAddress string
	GossipPort int
	RPCPort    int
	Datacenter string

	Autopilot ServerAutopilot
	Raft      ServerRaft
	TLS       ServerTLSInfo
	ACL       ServerACLInfo

	ScadaStatus string
}

type ServerAutopilot struct {
	FailureTolerance int
	Healthy          bool
	MinQuorum        int
	NumServers       int
	NumVoters        int
}

type ServerRaft struct {
	IsLeader             bool
	KnownLeader          bool
	AppliedIndex         uint64
	TimeSinceLastContact time.Duration
}

type ServerACLInfo struct {
	Enabled bool
}

type ServerTLSInfo struct {
	Enabled              bool
	CertExpiry           time.Time
	CertName             string
	CertSerial           string
	VerifyIncoming       bool
	VerifyOutgoing       bool
	VerifyServerHostname bool
}

func serverStatusToHCP(s *ServerStatus) *gnmmod.HashicorpCloudGlobalNetworkManager20220215ServerState {
	if s == nil {
		return nil
	}
	return &gnmmod.HashicorpCloudGlobalNetworkManager20220215ServerState{
		Autopilot: &gnmmod.HashicorpCloudGlobalNetworkManager20220215AutoPilotInfo{
			FailureTolerance: int32(s.Autopilot.FailureTolerance),
			Healthy:          s.Autopilot.Healthy,
			MinQuorum:        int32(s.Autopilot.MinQuorum),
			NumServers:       int32(s.Autopilot.NumServers),
			NumVoters:        int32(s.Autopilot.NumVoters),
		},
		GossipPort: int32(s.GossipPort),
		ID:         s.ID,
		LanAddress: s.LanAddress,
		Name:       s.Name,
		Raft: &gnmmod.HashicorpCloudGlobalNetworkManager20220215RaftInfo{
			AppliedIndex:         strconv.FormatUint(s.Raft.AppliedIndex, 10),
			IsLeader:             s.Raft.IsLeader,
			KnownLeader:          s.Raft.KnownLeader,
			TimeSinceLastContact: s.Raft.TimeSinceLastContact.String(),
		},
		RPCPort: int32(s.RPCPort),
		TLS: &gnmmod.HashicorpCloudGlobalNetworkManager20220215TLSInfo{
			CertExpiry:           strfmt.DateTime(s.TLS.CertExpiry),
			CertName:             s.TLS.CertName,
			CertSerial:           s.TLS.CertSerial,
			Enabled:              s.TLS.Enabled,
			VerifyIncoming:       s.TLS.VerifyIncoming,
			VerifyOutgoing:       s.TLS.VerifyOutgoing,
			VerifyServerHostname: s.TLS.VerifyServerHostname,
		},
		Version:     s.Version,
		ScadaStatus: s.ScadaStatus,
		ACL: &gnmmod.HashicorpCloudGlobalNetworkManager20220215ACLInfo{
			Enabled: s.ACL.Enabled,
		},
		Datacenter: s.Datacenter,
	}
}

func (c *hcpClient) DiscoverServers(ctx context.Context) ([]string, error) {
	params := hcpgnm.NewAgentDiscoverParamsWithContext(ctx).
		WithID(c.resource.ID).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project)

	resp, err := c.gnm.AgentDiscover(params, nil)
	if err != nil {
		return nil, err
	}
	var servers []string
	for _, srv := range resp.Payload.Servers {
		if srv != nil {
			servers = append(servers, fmt.Sprintf("%s:%d", srv.LanAddress, srv.GossipPort))
		}
	}

	return servers, nil
}

// Enabled verifies if telemetry is enabled by ensuring a valid endpoint has been retrieved.
// It returns full metrics endpoint and true if a valid endpoint was obtained.
func (t *TelemetryConfig) Enabled() (string, bool) {
	endpoint := t.Endpoint
	if override := t.MetricsConfig.Endpoint; override != "" {
		endpoint = override
	}

	if endpoint == "" {
		return "", false
	}

	// The endpoint from the HCP gateway is a domain without scheme, and without the metrics path, so they must be added.
	return fmt.Sprintf("https://%s/v1/metrics", endpoint), true
}

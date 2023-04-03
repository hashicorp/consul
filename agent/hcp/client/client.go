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

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/version"
	hcpgnm "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/client/global_network_manager_service"
	gnmmod "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/models"
	"github.com/hashicorp/hcp-sdk-go/httpclient"
	"github.com/hashicorp/hcp-sdk-go/resource"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Client interface exposes HCP operations that can be invoked by Consul
//
//go:generate mockery --name Client --with-expecter --inpackage
type Client interface {
	FetchTelemetryConfig(ctx context.Context) (*TelemetryConfig, error)
	FetchBootstrap(ctx context.Context) (*BootstrapConfig, error)
	PushServerStatus(ctx context.Context, status *ServerStatus) error
	DiscoverServers(ctx context.Context) ([]string, error)
	InitMetricsClient(ctx context.Context, endpoint string) error
	ExportMetrics(context.Context, *metricdata.ResourceMetrics) error
}

// TODO: This will be fixed in a follow up PR (CC-4637)
// Stubbed design for now until CCM protos are available.
type TelemetryConfig struct {
	Endpoint string
	Filters  []string
}

type BootstrapConfig struct {
	Name            string
	BootstrapExpect int
	GossipKey       string
	TLSCert         string
	TLSCertKey      string
	TLSCAs          []string
	ConsulConfig    string
}

type hcpClient struct {
	hc       *httptransport.Runtime
	cfg      config.CloudConfig
	gnm      hcpgnm.ClientService
	resource resource.Resource
	exporter metric.Exporter
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

// TODO: This will be fixed in a follow up PR (CC-4637)
// stubbed for now until CCM protos are available.
func (c *hcpClient) FetchTelemetryConfig(ctx context.Context) (*TelemetryConfig, error) {
	return &TelemetryConfig{
		Endpoint: "ebda33ed66ab.ngrok.app:9090",
		Filters:  []string{"raft.apply$"},
	}, nil
}

func (c *hcpClient) FetchBootstrap(ctx context.Context) (*BootstrapConfig, error) {
	params := hcpgnm.NewAgentBootstrapConfigParamsWithContext(ctx).
		WithID(c.resource.ID).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project)

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
	}
}

func (c *hcpClient) PushServerStatus(ctx context.Context, s *ServerStatus) error {
	params := hcpgnm.NewAgentPushServerStateParamsWithContext(ctx).
		WithID(c.resource.ID).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project)

	params.SetBody(&gnmmod.HashicorpCloudGlobalNetworkManager20220215AgentPushServerStateRequest{
		ServerState: serverStatusToHCP(s),
	})

	_, err := c.gnm.AgentPushServerState(params, nil)
	return err
}

func (c *hcpClient) InitMetricsClient(ctx context.Context, endpoint string) error {
	hcpConfig, err := c.cfg.HCPConfig()
	if err != nil {
		return err
	}

	// TODO:In follow up PR (CC-4635) : Need to use oauth2 transport in the otlpmetrichttp client to add token.
	// We likely need to fork and update the otlpmetrichttp client, as there is no interface to do this currently.
	exp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(endpoint), otlpmetrichttp.WithTLSClientConfig(hcpConfig.APITLSConfig()))
	if err != nil {
		return err
	}

	c.exporter = exp
	return nil
}

func (c *hcpClient) ExportMetrics(ctx context.Context, metrics *metricdata.ResourceMetrics) error {
	if c.exporter == nil {
		return fmt.Errorf("metrics exporter must be initialized with InitTelemetryClient first")
	}

	return c.exporter.Export(ctx, *metrics)
}

type ServerStatus struct {
	ID         string
	Name       string
	Version    string
	LanAddress string
	GossipPort int
	RPCPort    int

	Autopilot ServerAutopilot
	Raft      ServerRaft
	TLS       ServerTLSInfo

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

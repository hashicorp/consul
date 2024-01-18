// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

// metricsGatewayPath is the default path for metrics export request on the Telemetry Gateway.
const metricsGatewayPath = "/v1/metrics"

// Client interface exposes HCP operations that can be invoked by Consul
//
//go:generate mockery --name Client --with-expecter --inpackage
type Client interface {
	FetchBootstrap(ctx context.Context) (*BootstrapConfig, error)
	FetchTelemetryConfig(ctx context.Context) (*TelemetryConfig, error)
	GetObservabilitySecret(ctx context.Context) (clientID, clientSecret string, err error)
	PushServerStatus(ctx context.Context, status *ServerStatus) error
	DiscoverServers(ctx context.Context) ([]string, error)
	GetCluster(ctx context.Context) (*Cluster, error)
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

type Cluster struct {
	Name         string
	HCPPortalURL string
	AccessLevel  *gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterConsulAccessLevel
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
		return nil, fmt.Errorf("failed to fetch from HCP: %w", err)
	}

	if err := validateAgentTelemetryConfigPayload(resp); err != nil {
		return nil, fmt.Errorf("invalid response payload: %w", err)
	}

	return convertAgentTelemetryResponse(ctx, resp, c.cfg)
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

// ServerStatus is used to collect server status information in order to push
// to HCP. Fields should mirror HashicorpCloudGlobalNetworkManager20220215ServerState
type ServerStatus struct {
	ID         string
	Name       string
	Version    string
	LanAddress string
	GossipPort int
	RPCPort    int
	Datacenter string

	Autopilot         ServerAutopilot
	Raft              ServerRaft
	ACL               ServerACLInfo
	ServerTLSMetadata ServerTLSMetadata

	// TODO: TLS will be deprecated in favor of ServerTLSInfo in GNM. Handle
	// removal in a subsequent PR
	// https://hashicorp.atlassian.net/browse/CC-7015
	TLS ServerTLSInfo

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

// ServerTLSInfo mirrors HashicorpCloudGlobalNetworkManager20220215TLSInfo
type ServerTLSInfo struct {
	Enabled                bool
	CertExpiry             time.Time
	CertIssuer             string
	CertName               string
	CertSerial             string
	CertificateAuthorities []CertificateMetadata
	VerifyIncoming         bool
	VerifyOutgoing         bool
	VerifyServerHostname   bool
}

// ServerTLSMetadata mirrors HashicorpCloudGlobalNetworkManager20220215ServerTLSMetadata
type ServerTLSMetadata struct {
	InternalRPC ServerTLSInfo
}

// CertificateMetadata mirrors HashicorpCloudGlobalNetworkManager20220215CertificateMetadata
type CertificateMetadata struct {
	CertExpiry time.Time
	CertName   string
	CertSerial string
}

func serverStatusToHCP(s *ServerStatus) *gnmmod.HashicorpCloudGlobalNetworkManager20220215ServerState {
	if s == nil {
		return nil
	}

	// Convert CA metadata
	caCerts := make([]*gnmmod.HashicorpCloudGlobalNetworkManager20220215CertificateMetadata,
		len(s.ServerTLSMetadata.InternalRPC.CertificateAuthorities))
	for ix, ca := range s.ServerTLSMetadata.InternalRPC.CertificateAuthorities {
		caCerts[ix] = &gnmmod.HashicorpCloudGlobalNetworkManager20220215CertificateMetadata{
			CertExpiry: strfmt.DateTime(ca.CertExpiry),
			CertName:   ca.CertName,
			CertSerial: ca.CertSerial,
		}
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
			// TODO: remove TLS in preference for ServerTLSMetadata.InternalRPC
			// when deprecation path is ready
			// https://hashicorp.atlassian.net/browse/CC-7015
			CertExpiry:           strfmt.DateTime(s.TLS.CertExpiry),
			CertName:             s.TLS.CertName,
			CertSerial:           s.TLS.CertSerial,
			Enabled:              s.TLS.Enabled,
			VerifyIncoming:       s.TLS.VerifyIncoming,
			VerifyOutgoing:       s.TLS.VerifyOutgoing,
			VerifyServerHostname: s.TLS.VerifyServerHostname,
		},
		ServerTLS: &gnmmod.HashicorpCloudGlobalNetworkManager20220215ServerTLSMetadata{
			InternalRPC: &gnmmod.HashicorpCloudGlobalNetworkManager20220215TLSInfo{
				CertExpiry:             strfmt.DateTime(s.ServerTLSMetadata.InternalRPC.CertExpiry),
				CertIssuer:             s.ServerTLSMetadata.InternalRPC.CertIssuer,
				CertName:               s.ServerTLSMetadata.InternalRPC.CertName,
				CertSerial:             s.ServerTLSMetadata.InternalRPC.CertSerial,
				Enabled:                s.ServerTLSMetadata.InternalRPC.Enabled,
				VerifyIncoming:         s.ServerTLSMetadata.InternalRPC.VerifyIncoming,
				VerifyOutgoing:         s.ServerTLSMetadata.InternalRPC.VerifyOutgoing,
				VerifyServerHostname:   s.ServerTLSMetadata.InternalRPC.VerifyServerHostname,
				CertificateAuthorities: caCerts,
			},
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

func (c *hcpClient) GetCluster(ctx context.Context) (*Cluster, error) {
	params := hcpgnm.NewGetClusterParamsWithContext(ctx).
		WithID(c.resource.ID).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project)

	resp, err := c.gnm.GetCluster(params, nil)
	if err != nil {
		return nil, err
	}

	return clusterFromHCP(resp.Payload), nil
}

func clusterFromHCP(payload *gnmmod.HashicorpCloudGlobalNetworkManager20220215GetClusterResponse) *Cluster {
	return &Cluster{
		Name:         payload.Cluster.ID,
		AccessLevel:  payload.Cluster.ConsulAccessLevel,
		HCPPortalURL: payload.Cluster.HcpPortalURL,
	}
}

func (c *hcpClient) GetObservabilitySecret(ctx context.Context) (string, string, error) {
	params := hcpgnm.NewGetObservabilitySecretParamsWithContext(ctx).
		WithID(c.resource.ID).
		WithLocationOrganizationID(c.resource.Organization).
		WithLocationProjectID(c.resource.Project)

	resp, err := c.gnm.GetObservabilitySecret(params, nil)
	if err != nil {
		return "", "", err
	}

	if len(resp.GetPayload().Keys) == 0 {
		return "", "", fmt.Errorf("no observability keys returned for cluster")
	}

	key := resp.GetPayload().Keys[len(resp.GetPayload().Keys)-1]
	return key.ClientID, key.ClientSecret, nil
}

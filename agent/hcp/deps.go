// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcp

import (
	"context"
	"fmt"
	"net/url"
	"time"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
	"github.com/hashicorp/go-hclog"
)

// Deps contains the interfaces that the rest of Consul core depends on for HCP integration.
type Deps struct {
	Client   hcpclient.Client
	Provider scada.Provider
	Sink     *telemetry.OTELSink
}

func NewDeps(cfg config.CloudConfig, logger hclog.Logger) (d Deps, err error) {
	d.Client, err = hcpclient.NewClient(cfg)
	if err != nil {
		return
	}

	d.Provider, err = scada.New(cfg, logger.Named("hcp.scada"))
	if err != nil {
		return
	}

	// Make telemetry config request here to HCP.
	// CCM errors should be ignored and not block HCP init.
	ctx := context.Background()
	url, telemetryErr := verifyCCMRegistration(ctx, d.Client)
	if telemetryErr != nil {
		return
	}

	metricsClientOpts := &hcpclient.TelemetryClientCfg{
		Logger:   logger,
		CloudCfg: &cfg,
	}

	sinkOpts := &telemetry.OTELSinkOpts{
		Ctx:    ctx,
		Logger: logger,
	}

	d.Sink, telemetryErr = initHCPSink(sinkOpts, metricsClientOpts, url)
	if telemetryErr != nil {
		logger.Error("Failed to init telemetry.")
	}

	return
}

func verifyCCMRegistration(ctx context.Context, client hcpclient.Client) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := client.FetchTelemetryConfig(reqCtx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch telemetry config %w", err)
	}

	endpoint := telemetryCfg.Endpoint
	if override := telemetryCfg.MetricsOverride.Endpoint; override != "" {
		endpoint = override
	}

	if endpoint == "" {
		return "", fmt.Errorf("server not registed with management plane")
	}

	// The endpoint from the HCP gateway is a domain without scheme, so it must be added.
	url, err := url.Parse(fmt.Sprintf("https://%s/v1/metrics", endpoint))
	if err != nil {
		return "", fmt.Errorf("failed to parse url: %w", err)
	}

	return url.String(), nil
}

func initHCPSink(sinkOpts *telemetry.OTELSinkOpts, clientCfg *hcpclient.TelemetryClientCfg, url string) (*telemetry.OTELSink, error) {
	// If the above succeeds, the server is registered with CCM, init metrics sink.
	metricsClient, err := hcpclient.NewMetricsClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init metrics client: %w", err)
	}

	sinkOpts.Reader = telemetry.NewOTELReader(metricsClient, url, 10*time.Second)

	return telemetry.NewOTELSink(sinkOpts)
}

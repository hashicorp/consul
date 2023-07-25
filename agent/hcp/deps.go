// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcp

import (
	"context"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
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
	Sink     metrics.MetricSink
}

func NewDeps(cfg config.CloudConfig, logger hclog.Logger) (Deps, error) {
	ctx := context.Background()
	ctx = hclog.WithContext(ctx, logger)

	client, err := hcpclient.NewClient(cfg, ctx)
	if err != nil {
		return Deps{}, fmt.Errorf("failed to init client: %w", err)
	}

	provider, err := scada.New(cfg, logger.Named("scada"))
	if err != nil {
		return Deps{}, fmt.Errorf("failed to init scada: %w", err)
	}

	metricsClient, err := hcpclient.NewMetricsClient(ctx, &cfg)
	if err != nil {
		logger.Error("failed to init metrics client", "error", err)
		return Deps{}, fmt.Errorf("failed to init metrics client: %w", err)
	}

	sink, err := sink(ctx, client, metricsClient)
	if err != nil {
		// Do not prevent server start if sink init fails, only log error.
		logger.Error("failed to init sink", "error", err)
	}

	return Deps{
		Client:   client,
		Provider: provider,
		Sink:     sink,
	}, nil
}

// sink initializes an OTELSink which forwards Consul metrics to HCP.
// The sink is only initialized if the server is registered with the management plane (CCM).
// This step should not block server initialization, errors are returned, only to be logged.
func sink(
	ctx context.Context,
	hcpClient hcpclient.Client,
	metricsClient hcpclient.MetricsClient,
) (metrics.MetricSink, error) {
	logger := hclog.FromContext(ctx).Named("sink")
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := hcpClient.FetchTelemetryConfig(reqCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch telemetry config: %w", err)
	}

	if !telemetryCfg.MetricsEnabled() {
		return nil, nil
	}

	cfgProvider, err := NewTelemetryConfigProvider(&TelemetryConfigProviderOpts{
		Ctx:             ctx,
		MetricsConfig:   telemetryCfg.MetricsConfig,
		HCPClient:       hcpClient,
		RefreshInterval: telemetryCfg.RefreshConfig.RefreshInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init config provider: %w", err)
	}

	sinkOpts := &telemetry.OTELSinkOpts{
		Ctx:            ctx,
		Reader:         telemetry.NewOTELReader(metricsClient, cfgProvider, telemetry.DefaultExportInterval),
		ConfigProvider: cfgProvider,
	}

	sink, err := telemetry.NewOTELSink(sinkOpts)
	if err != nil {
		return nil, fmt.Errorf("failed create OTELSink: %w", err)
	}

	logger.Debug("initialized HCP metrics sink")

	return sink, nil
}

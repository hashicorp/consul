// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	client, err := hcpclient.NewClient(cfg)
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
	metricsClient telemetry.MetricsClient,
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

	cfgProvider, err := NewHCPProvider(ctx, hcpClient, telemetryCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init config provider: %w", err)
	}

	reader := telemetry.NewOTELReader(metricsClient, cfgProvider)
	sinkOpts := &telemetry.OTELSinkOpts{
		Reader:         reader,
		ConfigProvider: cfgProvider,
	}

	sink, err := telemetry.NewOTELSink(ctx, sinkOpts)
	if err != nil {
		return nil, fmt.Errorf("failed create OTELSink: %w", err)
	}

	logger.Debug("initialized HCP metrics sink")

	return sink, nil
}

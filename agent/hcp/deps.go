// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

// Deps contains the interfaces that the rest of Consul core depends on for HCP integration.
type Deps struct {
	Config            config.CloudConfig
	Client            client.Client
	Provider          scada.Provider
	Sink              metrics.MetricSink
	TelemetryProvider *hcpProviderImpl
	DataDir           string
}

func NewDeps(cfg config.CloudConfig, logger hclog.Logger, dataDir string) (Deps, error) {
	ctx := context.Background()
	ctx = hclog.WithContext(ctx, logger)

	var hcpClient client.Client
	if cfg.ResourceID != "" {
		var err error
		hcpClient, err = client.NewClient(cfg)
		if err != nil {
			return Deps{}, fmt.Errorf("failed to init client: %w", err)
		}
	}

	provider, err := scada.New(logger.Named("scada"))
	if err != nil {
		return Deps{}, fmt.Errorf("failed to init scada: %w", err)
	}

	metricsProvider := NewHCPProvider(ctx)
	if err != nil {
		logger.Error("failed to init HCP metrics provider", "error", err)
		return Deps{}, fmt.Errorf("failed to init HCP metrics provider: %w", err)
	}

	metricsClient := client.NewMetricsClient(ctx, metricsProvider)

	sink, err := sink(ctx, metricsClient, metricsProvider)
	if err != nil {
		// Do not prevent server start if sink init fails, only log error.
		logger.Error("failed to init sink", "error", err)
	}

	return Deps{
		Config:            cfg,
		Client:            hcpClient,
		Provider:          provider,
		Sink:              sink,
		TelemetryProvider: metricsProvider,
		DataDir:           dataDir,
	}, nil
}

// sink initializes an OTELSink which forwards Consul metrics to HCP.
// This step should not block server initialization, errors are returned, only to be logged.
func sink(
	ctx context.Context,
	metricsClient telemetry.MetricsClient,
	cfgProvider *hcpProviderImpl,
) (metrics.MetricSink, error) {
	logger := hclog.FromContext(ctx)

	reader := telemetry.NewOTELReader(metricsClient, cfgProvider)
	sinkOpts := &telemetry.OTELSinkOpts{
		Reader:         reader,
		ConfigProvider: cfgProvider,
	}

	sink, err := telemetry.NewOTELSink(ctx, sinkOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTELSink: %w", err)
	}

	logger.Debug("initialized HCP metrics sink")

	return sink, nil
}

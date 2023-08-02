// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	Client   client.Client
	Provider scada.Provider
	Sink     metrics.MetricSink
}

func NewDeps(cfg config.CloudConfig, logger hclog.Logger) (Deps, error) {
	ctx := context.Background()
	ctx = hclog.WithContext(ctx, logger)

	hcpClient, err := client.NewClient(cfg)
	if err != nil {
		return Deps{}, fmt.Errorf("failed to init client: %w", err)
	}

	provider, err := scada.New(cfg, logger.Named("scada"))
	if err != nil {
		return Deps{}, fmt.Errorf("failed to init scada: %w", err)
	}

	metricsClient, err := client.NewMetricsClient(ctx, &cfg)
	if err != nil {
		logger.Error("failed to init metrics client", "error", err)
		return Deps{}, fmt.Errorf("failed to init metrics client: %w", err)
	}

	sink, err := sink(ctx, hcpClient, metricsClient)
	if err != nil {
		// Do not prevent server start if sink init fails, only log error.
		logger.Error("failed to init sink", "error", err)
	}

	return Deps{
		Client:   hcpClient,
		Provider: provider,
		Sink:     sink,
	}, nil
}

// sink initializes an OTELSink which forwards Consul metrics to HCP.
// This step should not block server initialization, errors are returned, only to be logged.
func sink(
	ctx context.Context,
	hcpClient client.Client,
	metricsClient telemetry.MetricsClient,
) (metrics.MetricSink, error) {
	logger := hclog.FromContext(ctx)

	provider := NewHCPProvider(ctx, hcpClient)

	reader := telemetry.NewOTELReader(metricsClient, provider, telemetry.DefaultExportInterval)
	sinkOpts := &telemetry.OTELSinkOpts{
		Reader:         reader,
		ConfigProvider: provider,
	}

	sink, err := telemetry.NewOTELSink(ctx, sinkOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to init OTEL sink: %w", err)
	}

	logger.Debug("initialized HCP metrics sink")

	return sink, nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-metrics"
	"go.opentelemetry.io/otel"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

// Deps contains the interfaces that the rest of Consul core depends on for HCP integration.
type Deps struct {
	Config            config.CloudConfig
	Provider          scada.Provider
	Sink              metrics.ShutdownSink
	TelemetryProvider *hcpProviderImpl
	DataDir           string
}

func NewDeps(cfg config.CloudConfig, logger hclog.Logger, dataDir string) (Deps, error) {
	ctx := context.Background()
	ctx = hclog.WithContext(ctx, logger)

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

	sink, err := newSink(ctx, metricsClient, metricsProvider)
	if err != nil {
		// Do not prevent server start if sink init fails, only log error.
		logger.Error("failed to init sink", "error", err)
	}

	return Deps{
		Config:            cfg,
		Provider:          provider,
		Sink:              sink,
		TelemetryProvider: metricsProvider,
		DataDir:           dataDir,
	}, nil
}

// newSink initializes an OTELSink which forwards Consul metrics to HCP.
// This step should not block server initialization, errors are returned, only to be logged.
func newSink(
	ctx context.Context,
	metricsClient telemetry.MetricsClient,
	cfgProvider *hcpProviderImpl,
) (metrics.ShutdownSink, error) {
	logger := hclog.FromContext(ctx)

	// Set the global OTEL error handler. Without this, on any failure to publish metrics in
	// otelExporter.Export, the default OTEL handler logs to stderr without the formatting or group
	// that hclog provides. Here we override that global error handler once so logs are
	// in the standard format and include "hcp" in the group name like:
	// 2024-02-06T22:35:19.072Z [ERROR] agent.hcp: failed to export metrics: failed to export metrics: code 404: 404 page not found
	otel.SetErrorHandler(&otelErrorHandler{logger: logger})

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

type otelErrorHandler struct {
	logger hclog.Logger
}

func (o *otelErrorHandler) Handle(err error) {
	o.logger.Error(err.Error())
}

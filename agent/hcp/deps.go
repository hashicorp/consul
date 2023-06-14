// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcp

import (
	"context"
	"fmt"
	"net/url"
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
	logger = logger.Named("sink")
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

	sink := sink(ctx, client, metricsClient, cfg)

	return Deps{
		Client:   client,
		Provider: provider,
		Sink:     sink,
	}, nil
}

// sink provides initializes an OTELSink which forwards Consul metrics to HCP.
// The sink is only initialized if the server is registered with the management plane (CCM).
// This step should not block server initialization, so errors are logged, but not returned.
func sink(
	ctx context.Context,
	hcpClient hcpclient.Client,
	metricsClient hcpclient.MetricsClient,
	cfg config.CloudConfig,
) metrics.MetricSink {
	logger := hclog.FromContext(ctx)
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := hcpClient.FetchTelemetryConfig(reqCtx)
	if err != nil {
		logger.Error("failed to fetch telemetry config", "error", err)
		return nil
	}

	endpoint, isEnabled := telemetryCfg.Enabled()
	if !isEnabled {
		return nil
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		logger.Error("failed to parse url endpoint", "error", err)
		return nil
	}

	sinkOpts := &telemetry.OTELSinkOpts{
		Ctx:     ctx,
		Reader:  telemetry.NewOTELReader(metricsClient, u, telemetry.DefaultExportInterval),
		Labels:  telemetryCfg.DefaultLabels(cfg),
		Filters: telemetryCfg.MetricsConfig.Filters,
	}

	sink, err := telemetry.NewOTELSink(sinkOpts)
	if err != nil {
		logger.Error("failed to init OTEL sink", "error", err)
		return nil
	}

	logger.Debug("initialized HCP metrics sink")

	return sink
}

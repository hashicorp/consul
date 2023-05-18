// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcp

import (
	"context"
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

func NewDeps(cfg config.CloudConfig, logger hclog.Logger) (d Deps, err error) {
	d.Client, err = hcpclient.NewClient(cfg)
	if err != nil {
		return
	}

	d.Provider, err = scada.New(cfg, logger.Named("hcp.scada"))
	if err != nil {
		return
	}

	d.Sink = sink(d.Client, &cfg, logger)

	return
}

// sink provides initializes an OTELSink which forwards Consul metrics to HCP.
// The sink is only initialized if the server is registered with the management plane (CCM).
// This step should not block server initialization, so errors are logged, but not returned.
func sink(hcpClient hcpclient.Client, cfg hcpclient.CloudConfig, logger hclog.Logger) *telemetry.OTELSink {
	ctx := context.Background()
	ctx = hclog.WithContext(ctx, logger)

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

	metricsClient, err := hcpclient.NewMetricsClient(cfg, ctx)
	if err != nil {
		logger.Error("failed to init metrics client", "error", err)
		return nil
	}

	sinkOpts := &telemetry.OTELSinkOpts{
		Ctx:    ctx,
		Reader: telemetry.NewOTELReader(metricsClient, u, telemetry.DefaultExportInterval),
	}

	sink, err := telemetry.NewOTELSink(sinkOpts)
	if err != nil {
		logger.Error("failed to init OTEL sink", "error", err)
		return nil
	}

	return sink
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcp

import (
	"context"
	"net/url"
	"time"

	gometrics "github.com/armon/go-metrics"
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
	Sink     gometrics.MetricSink
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

	d.Sink, err = initTelemetry(d.Client, logger, cfg)

	return
}

func initTelemetry(hcpClient hcpclient.Client, logger hclog.Logger, cfg config.CloudConfig) (gometrics.MetricSink, error) {
	// Make telemetry config request here to HCP.
	ctx := context.Background()
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	telemetryCfg, err := hcpClient.FetchTelemetryConfig(reqCtx)
	if err != nil {
		return nil, err
	}

	endpoint := telemetryCfg.Endpoint
	if override := telemetryCfg.MetricsOverride.Endpoint; override != "" {
		endpoint = override
	}

	url, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	url.Scheme = "https"

	// If the above succeeds, the server is registered with CCM, init metrics sink.
	metricsClient, err := hcpclient.NewMetricsClient(&hcpclient.TelemetryClientCfg{
		Logger:   logger,
		CloudCfg: &cfg,
	})
	if err != nil {
		return nil, err
	}

	opts := &telemetry.OTELSinkOpts{
		Reader: telemetry.NewOTELReader(metricsClient, endpoint, 10*time.Second),
		Logger: logger,
		Ctx:    ctx,
	}

	return telemetry.NewOTELSink(opts)
}

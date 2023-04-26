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

	ctx := context.Background()
	// Make telemetry config request here to verify registration with CCM.
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	config, err := d.Client.FetchTelemetryConfig(reqCtx)
	if err != nil {
		return
	}

	endpoint := config.Endpoint
	if config.MetricsOverride.Endpoint != "" {
		endpoint = config.MetricsOverride.Endpoint
	}
	url, err := url.Parse(endpoint)
	if err != nil {
		return
	}
	url.Scheme = "https"

	// If the above succeeds, Init metrics sink
	metricsClient, err := hcpclient.NewMetricsClient(&hcpclient.TelemetryClientCfg{
		Logger:   logger,
		CloudCfg: &cfg,
	})
	if err != nil {
		return
	}

	opts := &telemetry.OTELSinkOpts{
		Reader:         telemetry.NewOTELReader(metricsClient, endpoint),
		Logger:         logger,
		ExportInterval: 10 * time.Second,
		Ctx:            ctx,
	}
	d.Sink, err = telemetry.NewOTELSink(opts)

	return
}

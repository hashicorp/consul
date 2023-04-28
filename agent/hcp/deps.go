// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcp

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/coredns/coredns/plugin/pkg/log"
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

	d.Sink = sink(d.Client, &cfg, logger)

	return
}

// sink provides initializes an OTELSink which forwards Consul metrics to HCP.
// The sink is only initialized if the server is registered with the management plane (CCM).
// This step should not block server initialization, so errors are logged, but not returned.
func sink(hcpClient hcpclient.Client, cfg hcpclient.CloudConfig, logger hclog.Logger) *telemetry.OTELSink {
	ctx := context.Background()
	url, err := verifyCCMRegistration(ctx, hcpClient)

	// if endpoint is empty, no metrics endpoint configuration for this Consul server
	// (e.g. not registered with CCM or feature flag to control rollout) so do not enable the HCP metrics sink.
	if url == "" {
		return nil
	}

	metricsClientOpts := &hcpclient.TelemetryClientCfg{
		Logger:   logger,
		CloudCfg: cfg,
	}

	metricsClient, err := hcpclient.NewMetricsClient(metricsClientOpts)
	if err != nil {
		logger.Error("failed to init metrics client: %w", err)
		return nil
	}

	sinkOpts := &telemetry.OTELSinkOpts{
		Ctx:    ctx,
		Logger: logger,
		Reader: telemetry.NewOTELReader(metricsClient, url, 10*time.Second),
	}

	sink, err := telemetry.NewOTELSink(sinkOpts)
	if err != nil {
		logger.Error("failed to init OTEL sink: %w", err)
		return nil
	}

	return sink
}

// verifyCCMRegistration checks that a server is registered with the HCP management plane
// by making a HTTP request to the HCP TelemetryConfig endpoint.
// If registered, it returns the endpoint for the HCP Telemetry Gateway endpoint where metrics should be forwarded.
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

	// no error, the server simply isn't configured for metrics forwarding.
	if endpoint == "" {
		return "", nil
	}

	// The endpoint from the HCP gateway is a domain without scheme, and without the metrics path, so they must be added.
	url, err := url.Parse(fmt.Sprintf("https://%s/v1/metrics", endpoint))
	if err != nil {
		log.Error("failed to parse url: %w", err)
		return "", fmt.Errorf("failed to parse url: %w", err)
	}

	return url.String(), nil
}

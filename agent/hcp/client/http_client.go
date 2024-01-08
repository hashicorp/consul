// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/oauth2"
)

const (
	// HTTP Client config
	defaultStreamTimeout = 15 * time.Second

	// Retry config
	// TODO: Eventually, we'd like to configure these values dynamically.
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 15 * time.Second
	// defaultRetryMax is set to 0 to turn off retry functionality, until dynamic configuration is possible.
	// This is to circumvent any spikes in load that may cause or exacerbate server-side issues for now.
	defaultRetryMax = 0
)

// newHTTPClient configures the retryable HTTP client.
func newHTTPClient(cloudCfg CloudConfig, logger hclog.Logger) (*retryablehttp.Client, error) {
	hcpCfg, err := cloudCfg.HCPConfig()
	if err != nil {
		return nil, err
	}

	tlsTransport := cleanhttp.DefaultPooledTransport()
	tlsTransport.TLSClientConfig = hcpCfg.APITLSConfig()

	var transport http.RoundTripper = &oauth2.Transport{
		Base:   tlsTransport,
		Source: hcpCfg,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   defaultStreamTimeout,
	}

	retryClient := &retryablehttp.Client{
		HTTPClient:   client,
		Logger:       logger.Named("hcp_telemetry_client"),
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}

	return retryClient, nil
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"crypto/tls"
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

// NewHTTPClient configures the retryable HTTP client.
func NewHTTPClient(tlsCfg *tls.Config, source oauth2.TokenSource) *retryablehttp.Client {
	tlsTransport := cleanhttp.DefaultPooledTransport()
	tlsTransport.TLSClientConfig = tlsCfg

	var transport http.RoundTripper = &oauth2.Transport{
		Base:   tlsTransport,
		Source: source,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   defaultStreamTimeout,
	}

	retryClient := &retryablehttp.Client{
		HTTPClient: client,
		// We already log failed requests elsewhere, we pass a null logger here to avoid redundant logs.
		Logger:       hclog.NewNullLogger(),
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     defaultRetryMax,
		CheckRetry:   retryablehttp.DefaultRetryPolicy,
		Backoff:      retryablehttp.DefaultBackoff,
	}

	return retryClient
}

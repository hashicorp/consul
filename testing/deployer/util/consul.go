// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/consul-server-connection-manager/discovery"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
)

func DialExposedGRPCConn(
	ctx context.Context, logger hclog.Logger,
	exposedServerGRPCPort int, token string,
	tlsConfig *tls.Config,
) (*grpc.ClientConn, func(), error) {
	if exposedServerGRPCPort <= 0 {
		return nil, nil, fmt.Errorf("cannot dial server grpc on port %d", exposedServerGRPCPort)
	}

	cfg := discovery.Config{
		Addresses: "127.0.0.1",
		GRPCPort:  exposedServerGRPCPort,
		// Disable server watch because we only need to get server IPs once.
		ServerWatchDisabled: true,
		TLS:                 tlsConfig,

		Credentials: discovery.Credentials{
			Type: discovery.CredentialsTypeStatic,
			Static: discovery.StaticTokenCredential{
				Token: token,
			},
		},
	}
	watcher, err := discovery.NewWatcher(ctx, cfg, logger.Named("consul-server-connection-manager"))
	if err != nil {
		return nil, nil, err
	}

	go watcher.Run()

	// We recycle the GRPC connection from the discovery client because it
	// should have all the necessary dial options, including the resolver that
	// continuously updates Consul server addresses. Otherwise, a lot of code from consul-server-connection-manager
	// would need to be duplicated
	state, err := watcher.State()
	if err != nil {
		watcher.Stop()
		return nil, nil, fmt.Errorf("unable to get connection manager state: %w", err)
	}

	return state.GRPCConn, func() { watcher.Stop() }, nil
}

func ProxyNotPooledAPIClient(proxyPort int, containerIP string, containerPort int, token string) (*api.Client, error) {
	return proxyAPIClient(cleanhttp.DefaultTransport(), proxyPort, containerIP, containerPort, token)
}

func ProxyAPIClient(proxyPort int, containerIP string, containerPort int, token string) (*api.Client, error) {
	return proxyAPIClient(cleanhttp.DefaultPooledTransport(), proxyPort, containerIP, containerPort, token)
}

func proxyAPIClient(baseTransport *http.Transport, proxyPort int, containerIP string, containerPort int, token string) (*api.Client, error) {
	if proxyPort <= 0 {
		return nil, fmt.Errorf("cannot use an http proxy on port %d", proxyPort)
	}
	if containerIP == "" {
		return nil, fmt.Errorf("container IP is required")
	}
	if containerPort <= 0 {
		return nil, fmt.Errorf("cannot dial api client on port %d", containerPort)
	}

	proxyURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(proxyPort))
	if err != nil {
		return nil, err
	}

	cfg := api.DefaultConfig()
	cfg.Transport = baseTransport
	cfg.Transport.Proxy = http.ProxyURL(proxyURL)
	cfg.Address = net.JoinHostPort(containerIP, strconv.Itoa(containerPort))
	cfg.Token = token
	return api.NewClient(cfg)
}

func ProxyNotPooledHTTPTransport(proxyPort int) (*http.Transport, error) {
	return proxyHTTPTransport(cleanhttp.DefaultTransport(), proxyPort)
}

func ProxyHTTPTransport(proxyPort int) (*http.Transport, error) {
	return proxyHTTPTransport(cleanhttp.DefaultPooledTransport(), proxyPort)
}

func proxyHTTPTransport(baseTransport *http.Transport, proxyPort int) (*http.Transport, error) {
	if proxyPort <= 0 {
		return nil, fmt.Errorf("cannot use an http proxy on port %d", proxyPort)
	}
	proxyURL, err := url.Parse("http://127.0.0.1:" + strconv.Itoa(proxyPort))
	if err != nil {
		return nil, err
	}
	baseTransport.Proxy = http.ProxyURL(proxyURL)
	return baseTransport, nil
}

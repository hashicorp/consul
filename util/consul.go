package util

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-cleanhttp"
)

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
	cfg.Address = fmt.Sprintf("http://%s:%d", containerIP, containerPort)
	cfg.Token = token
	return api.NewClient(cfg)
}

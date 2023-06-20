// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"

	"github.com/hashicorp/consul/api"
)

type apiHandler struct{}

func (apiHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}

// Note: this assumes a suitable Consul ACL token with 'service:write' for
// service 'web' is set in CONSUL_HTTP_TOKEN ENV var.
func ExampleService_ServerTLSConfig_hTTP() {
	client, _ := api.NewClient(api.DefaultConfig())
	svc, _ := NewService("web", client)
	server := &http.Server{
		Addr:      ":8080",
		Handler:   apiHandler{},
		TLSConfig: svc.ServerTLSConfig(),
	}
	// Cert and key files are blank since the tls.Config will handle providing
	// those dynamically.
	log.Fatal(server.ListenAndServeTLS("", ""))
}

func acceptLoop(l net.Listener) {}

// Note: this assumes a suitable Consul ACL token with 'service:write' for
// service 'web' is set in CONSUL_HTTP_TOKEN ENV var.
func ExampleService_ServerTLSConfig_tLS() {
	client, _ := api.NewClient(api.DefaultConfig())
	svc, _ := NewService("web", client)
	l, _ := tls.Listen("tcp", ":8080", svc.ServerTLSConfig())
	acceptLoop(l)
}

func handleResponse(r *http.Response) {}

// Note: this assumes a suitable Consul ACL token with 'service:write' for
// service 'web' is set in CONSUL_HTTP_TOKEN ENV var.
func ExampleService_HTTPClient() {
	client, _ := api.NewClient(api.DefaultConfig())
	svc, _ := NewService("web", client)

	httpClient := svc.HTTPClient()
	resp, _ := httpClient.Get("https://web.service.consul/foo/bar")
	handleResponse(resp)
}

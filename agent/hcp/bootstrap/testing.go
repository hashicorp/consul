// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package bootstrap

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/hashicorp/go-uuid"
	gnmmod "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/models"
	"github.com/hashicorp/hcp-sdk-go/resource"

	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/tlsutil"
)

// TestEndpoint returns an hcp.TestEndpoint to be used in an hcp.MockHCPServer.
func TestEndpoint() hcp.TestEndpoint {
	// Memoize data so it's consistent for the life of the test server
	data := make(map[string]gnmmod.HashicorpCloudGlobalNetworkManager20220215AgentBootstrapResponse)

	return hcp.TestEndpoint{
		Methods:    []string{"GET"},
		PathSuffix: "agent/bootstrap_config",
		Handler: func(r *http.Request, cluster resource.Resource) (interface{}, error) {
			return handleBootstrap(data, cluster)
		},
	}
}

func handleBootstrap(data map[string]gnmmod.HashicorpCloudGlobalNetworkManager20220215AgentBootstrapResponse, cluster resource.Resource) (interface{}, error) {
	resp, ok := data[cluster.ID]
	if !ok {
		// Create new response
		r, err := generateClusterData(cluster)
		if err != nil {
			return nil, err
		}
		data[cluster.ID] = r
		resp = r
	}
	return resp, nil
}

const TestExistingClusterID = "133114e7-9745-41ce-b1c9-9644a20d2952"

func testLeaf(caCert, caKey string) (serverCert, serverKey string, err error) {
	signer, err := tlsutil.ParseSigner(caKey)
	if err != nil {
		return "", "", err
	}

	serverCert, serverKey, err = tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          caCert,
		Name:        "server.dc1.consul",
		Days:        30,
		DNSNames:    []string{"server.dc1.consul", "localhost"},
		IPAddresses: append([]net.IP{}, net.ParseIP("127.0.0.1")),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return "", "", err
	}
	return serverCert, serverKey, nil
}

func generateClusterData(cluster resource.Resource) (gnmmod.HashicorpCloudGlobalNetworkManager20220215AgentBootstrapResponse, error) {
	resp := gnmmod.HashicorpCloudGlobalNetworkManager20220215AgentBootstrapResponse{
		Cluster: &gnmmod.HashicorpCloudGlobalNetworkManager20220215Cluster{},
		Bootstrap: &gnmmod.HashicorpCloudGlobalNetworkManager20220215ClusterBootstrap{
			ServerTLS: &gnmmod.HashicorpCloudGlobalNetworkManager20220215ServerTLS{},
		},
	}
	if cluster.ID == TestExistingClusterID {
		token, err := uuid.GenerateUUID()
		if err != nil {
			return resp, err
		}
		resp.Bootstrap.ConsulConfig = "{}"
		resp.Bootstrap.ManagementToken = token
		return resp, nil
	}

	caCert, caKey, err := tlsutil.GenerateCA(tlsutil.CAOpts{})
	if err != nil {
		return resp, err
	}
	serverCert, serverKey, err := testLeaf(caCert, caKey)
	if err != nil {
		return resp, err
	}

	resp.Bootstrap.ServerTLS.CertificateAuthorities = append(resp.Bootstrap.ServerTLS.CertificateAuthorities, caCert)
	resp.Bootstrap.ServerTLS.Cert = serverCert
	resp.Bootstrap.ServerTLS.PrivateKey = serverKey

	// Generate Config. We don't use the read config.Config struct because it
	// doesn't have `omitempty` which makes the output gross. We only want a tiny
	// subset, so we use a map that ends up with the same structure for now.

	// Gossip key
	gossipKeyBs := make([]byte, 32)
	_, err = rand.Reader.Read(gossipKeyBs)
	if err != nil {
		return resp, err
	}

	retryJoinArgs := map[string]string{
		"provider":      "hcp",
		"resource_id":   cluster.String(),
		"client_id":     "test_id",
		"client_secret": "test_secret",
	}

	cfg := map[string]interface{}{
		"encrypt":                 base64.StdEncoding.EncodeToString(gossipKeyBs),
		"encrypt_verify_incoming": true,
		"encrypt_verify_outgoing": true,

		// TLS settings (certs will be added by client since we can't put them inline)
		"verify_incoming":        true,
		"verify_outgoing":        true,
		"verify_server_hostname": true,
		"auto_encrypt": map[string]interface{}{
			"allow_tls": true,
		},

		// Enable HTTPS port, disable HTTP
		"ports": map[string]interface{}{
			"https":    8501,
			"http":     -1,
			"grpc_tls": 8503,
		},

		// RAFT Peers
		"bootstrap_expect": 1,
		"retry_join": []string{
			mapArgsString(retryJoinArgs),
		},
	}

	// ACLs
	token, err := uuid.GenerateUUID()
	if err != nil {
		return resp, err
	}
	resp.Bootstrap.ManagementToken = token

	cfg["acl"] = map[string]interface{}{
		"tokens": map[string]interface{}{
			// Also setup the server's own agent token to be the management token so it has
			// permission to register itself.
			"agent":              token,
			"initial_management": token,
		},
		"default_policy":           "deny",
		"enabled":                  true,
		"enable_token_persistence": true,
	}

	// Encode and return a JSON string in the response
	jsonBs, err := json.Marshal(cfg)
	if err != nil {
		return resp, err
	}
	resp.Bootstrap.ConsulConfig = string(jsonBs)

	return resp, nil
}

func mapArgsString(m map[string]string) string {
	args := make([]string, len(m))
	for k, v := range m {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(args, " ")
}

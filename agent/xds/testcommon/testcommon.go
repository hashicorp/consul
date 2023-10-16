// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testcommon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func SetupTLSRootsAndLeaf(t *testing.T, snap *proxycfg.ConfigSnapshot) {
	if snap.Leaf() != nil {
		switch snap.Kind {
		case structs.ServiceKindConnectProxy:
			snap.ConnectProxy.Leaf.CertPEM = loadTestResource(t, "test-leaf-cert")
			snap.ConnectProxy.Leaf.PrivateKeyPEM = loadTestResource(t, "test-leaf-key")
		case structs.ServiceKindIngressGateway:
			snap.IngressGateway.Leaf.CertPEM = loadTestResource(t, "test-leaf-cert")
			snap.IngressGateway.Leaf.PrivateKeyPEM = loadTestResource(t, "test-leaf-key")
		case structs.ServiceKindMeshGateway:
			snap.MeshGateway.Leaf.CertPEM = loadTestResource(t, "test-leaf-cert")
			snap.MeshGateway.Leaf.PrivateKeyPEM = loadTestResource(t, "test-leaf-key")
		case structs.ServiceKindAPIGateway:
			snap.APIGateway.Leaf.CertPEM = loadTestResource(t, "test-leaf-cert")
			snap.APIGateway.Leaf.PrivateKeyPEM = loadTestResource(t, "test-leaf-key")
		}
	}
	if snap.Roots != nil {
		snap.Roots.Roots[0].RootCert = loadTestResource(t, "test-root-cert")
	}
}
func loadTestResource(t *testing.T, name string) string {
	t.Helper()

	expected, err := os.ReadFile(filepath.Join("testdata", name+".golden"))
	require.NoError(t, err)
	return string(expected)
}

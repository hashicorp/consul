package autoconf

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

func newLeaf(t *testing.T, agentName, datacenter string, root connect.CertKeyPair, idx uint64, expiration time.Duration) *structs.IssuedCert {
	t.Helper()

	pub, priv, err := connect.TestAgentLeaf(t, agentName, datacenter, root, expiration)
	require.NoError(t, err)
	cert, err := connect.ParseCert(pub)
	require.NoError(t, err)

	spiffeID, err := connect.ParseCertURI(cert.URIs[0])
	require.NoError(t, err)

	agentID, ok := spiffeID.(*connect.SpiffeIDAgent)
	require.True(t, ok, "certificate doesn't have an agent leaf cert URI")

	return &structs.IssuedCert{
		SerialNumber:   cert.SerialNumber.String(),
		CertPEM:        pub,
		PrivateKeyPEM:  priv,
		ValidAfter:     cert.NotBefore,
		ValidBefore:    cert.NotAfter,
		Agent:          agentID.Agent,
		AgentURI:       agentID.URI().String(),
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		RaftIndex: structs.RaftIndex{
			CreateIndex: idx,
			ModifyIndex: idx,
		},
	}
}

func testCerts(t *testing.T, agentName, datacenter string) (connect.TestCAResult, *structs.IndexedCARoots, *structs.IssuedCert) {
	ca := connect.TestCA(t, nil)
	cert := newLeaf(t, agentName, datacenter, ca.KeyPair(), 1, 10*time.Minute)

	root := ca.ToCARoot()
	// The mock expects a call with a non-nil slice
	root.IntermediateCerts = make([]string, 0)

	indexedRoots := structs.IndexedCARoots{
		ActiveRootID: ca.ID,
		TrustDomain:  connect.TestClusterID,
		Roots:        []*structs.CARoot{root},
		QueryMeta:    structs.QueryMeta{Index: 1},
	}

	return ca, &indexedRoots, cert
}

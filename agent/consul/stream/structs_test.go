package stream

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var result interface{}

func BenchmarkCheckServiceNodeMarshal(b *testing.B) {
	// Use a really basic CSN
	csn := CheckServiceNode{
		Node: &Node{
			Node:    "node1",
			Address: "10.10.10.10",
		},
		Service: &NodeService{
			Service: "web",
			Port:    8080,
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		bs, _ := csn.Marshal()
		result = bs // Ensure compiler can't elide the loop body by making the result visible outside loop
	}
}

func BenchmarkCheckServiceConfigNodeMarshal(b *testing.B) {
	// Use a really basic CSN
	csn := CheckServiceNode{
		Node: &Node{
			Node:    "node1",
			Address: "10.10.10.10",
		},
		Service: &NodeService{
			Service: "web",
			Port:    8080,
			Proxy:   ConnectProxyConfig{},
			Connect: ServiceConnect{},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		bs, _ := csn.Marshal()
		result = bs // Ensure compiler can't elide the loop body by making the result visible outside loop
	}
}

func TestCheckServiceConfigNodeMarshal(t *testing.T) {
	// Use a really basic CSN
	csn := CheckServiceNode{
		Node: &Node{
			Node:    "node1",
			Address: "10.10.10.10",
		},
		Service: &NodeService{
			Service: "web",
			Port:    8080,
			Proxy:   ConnectProxyConfig{},
			Connect: ServiceConnect{},
		},
	}

	bs, err := csn.Marshal()
	require.NoError(t, err)

	var got CheckServiceNode
	require.NoError(t, got.Unmarshal(bs))

	require.Equal(t, csn, got)
}

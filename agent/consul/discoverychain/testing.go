package discoverychain

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

func TestCompileConfigEntries(t testing.T,
	serviceName string,
	evaluateInNamespace string,
	evaluateInPartition string,
	evaluateInDatacenter string,
	evaluateInTrustDomain string,
	setup func(req *CompileRequest), entries ...structs.ConfigEntry) *structs.CompiledDiscoveryChain {
	set := structs.NewDiscoveryChainConfigEntries()

	set.AddEntries(entries...)

	req := CompileRequest{
		ServiceName:           serviceName,
		EvaluateInNamespace:   evaluateInNamespace,
		EvaluateInPartition:   evaluateInPartition,
		EvaluateInDatacenter:  evaluateInDatacenter,
		EvaluateInTrustDomain: evaluateInTrustDomain,
		Entries:               set,
	}
	if setup != nil {
		setup(&req)
	}

	chain, err := Compile(req)
	require.NoError(t, err)
	return chain
}

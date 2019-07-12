package discoverychain

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

func TestCompileConfigEntries(
	t testing.T,
	serviceName string,
	currentNamespace string,
	currentDatacenter string,
	entries ...structs.ConfigEntry,
) *structs.CompiledDiscoveryChain {
	set := structs.NewDiscoveryChainConfigEntries()

	set.AddEntries(entries...)

	chain, err := Compile(CompileRequest{
		ServiceName:       serviceName,
		CurrentNamespace:  currentNamespace,
		CurrentDatacenter: currentDatacenter,
		InferDefaults:     true,
		Entries:           set,
	})
	require.NoError(t, err)
	return chain
}

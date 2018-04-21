package consul

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/assert"
)

func TestCAProvider_Bootstrap(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	provider := s1.getCAProvider()

	root, err := provider.ActiveRoot()
	assert.NoError(err)

	state := s1.fsm.State()
	_, activeRoot, err := state.CARootActive(nil)
	assert.NoError(err)
	assert.Equal(root.ID, activeRoot.ID)
	assert.Equal(root.Name, activeRoot.Name)
	assert.Equal(root.RootCert, activeRoot.RootCert)
}

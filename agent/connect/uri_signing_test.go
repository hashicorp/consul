package connect

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"

	"github.com/stretchr/testify/assert"
)

// Signing ID should never authorize
func TestSpiffeIDSigningAuthorize(t *testing.T) {
	var id SpiffeIDSigning
	auth, ok := id.Authorize(nil)
	assert.False(t, auth)
	assert.True(t, ok)
}

func TestSpiffeIDSigningForCluster(t *testing.T) {
	// For now it should just append .consul to the ID.
	config := &structs.CAConfiguration{
		ClusterID: testClusterID,
	}
	id := SpiffeIDSigningForCluster(config)
	assert.Equal(t, id.URI().String(), "spiffe://"+testClusterID+".consul")
}

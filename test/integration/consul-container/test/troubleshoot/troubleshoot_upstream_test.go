package troubleshoot

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/test/observability"
)

func TestTroubleshootUpstream_Success(t *testing.T) {

	t.Parallel()
	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	_, clientService := observability.CreateServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)
	_, port := clientSidecar.GetInternalAdminAddr()

	require.Eventually(t, func() bool {
		output, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "upstreams", "-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", port)})
		require.NoError(t, err)
		return strings.Contains(output, libservice.StaticServerServiceName)
	}, 30*time.Second, 5*time.Second)
}

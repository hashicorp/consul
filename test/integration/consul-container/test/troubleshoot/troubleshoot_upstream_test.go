package troubleshoot

import (
	"context"
	"fmt"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/test/observability"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
	"time"
)

func TestTroubleshootUpstream_Success(t *testing.T) {

	cluster, _, _ := topology.NewPeeringCluster(t, 1, &libcluster.BuildOptions{
		Datacenter:           "dc1",
		InjectAutoEncryption: true,
	})

	_, clientService := observability.CreateServices(t, cluster)

	clientSidecar, ok := clientService.(*libservice.ConnectContainer)
	require.True(t, ok)
	_, port := clientSidecar.GetInternalAdminAddr()
	// wait for envoy
	time.Sleep(5 * time.Second)
	_, outputReader, err := clientSidecar.Exec(context.Background(), []string{"consul", "troubleshoot", "upstreams", "-envoy-admin-endpoint", fmt.Sprintf("localhost:%v", port)})
	buf, err := io.ReadAll(outputReader)
	require.NoError(t, err)
	require.Contains(t, string(buf), libservice.StaticServerServiceName)
}

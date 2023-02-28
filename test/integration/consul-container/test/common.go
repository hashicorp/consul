package test

import (
	"testing"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

type TestLogConsumer struct {
	Msgs []string
}

func (g *TestLogConsumer) Accept(l testcontainers.Log) {
	g.Msgs = append(g.Msgs, string(l.Content))
}

// Creates a cluster with options for basic customization. All args except t
// are optional and will use sensible defaults when not provided.
func CreateCluster(
	t *testing.T,
	cmd string,
	logConsumer *TestLogConsumer,
	buildOptions *libcluster.BuildOptions,
	applyDefaultProxySettings bool,
	ports ...int,
) *libcluster.Cluster {

	// optional
	if buildOptions == nil {
		buildOptions = &libcluster.BuildOptions{
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
		}
	}
	ctx := libcluster.NewBuildContext(t, *buildOptions)

	conf := libcluster.NewConfigBuilder(ctx).ToAgentConfig(t)

	// optional
	if logConsumer != nil {
		conf.LogConsumer = logConsumer
	}

	t.Logf("Cluster config:\n%s", conf.JSON)

	// optional custom cmd
	if cmd != "" {
		conf.Cmd = append(conf.Cmd, cmd)
	}

	cluster, err := libcluster.New(t, []libcluster.Config{*conf}, ports...)
	require.NoError(t, err)

	node := cluster.Agents[0]
	client := node.GetClient()

	libcluster.WaitForLeader(t, cluster, client)
	libcluster.WaitForMembers(t, client, 1)

	if applyDefaultProxySettings {
		ok, err := utils.ApplyDefaultProxySettings(client)
		require.NoError(t, err)
		require.True(t, ok)
	}

	return cluster
}

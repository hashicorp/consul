package xds

import (
	"testing"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoyauth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func Test_makeUpstreamCluster(t *testing.T) {
	tests := []struct {
		name     string
		snap     proxycfg.ConfigSnapshot
		upstream structs.Upstream
		want     *envoy.Cluster
	}{
		{
			name:     "timeout override",
			snap:     proxycfg.ConfigSnapshot{},
			upstream: structs.TestUpstreams(t)[0],
			want: &envoy.Cluster{
				Name: "service:db",
				Type: envoy.Cluster_EDS,
				EdsClusterConfig: &envoy.Cluster_EdsClusterConfig{
					EdsConfig: &envoycore.ConfigSource{
						ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
							Ads: &envoycore.AggregatedConfigSource{},
						},
					},
				},
				ConnectTimeout:   1 * time.Second, // TestUpstreams overrides to 1000ms
				OutlierDetection: &cluster.OutlierDetection{},
				TlsContext: &envoyauth.UpstreamTlsContext{
					CommonTlsContext: makeCommonTLSContext(&proxycfg.ConfigSnapshot{}),
				},
			},
		},
		{
			name:     "timeout default",
			snap:     proxycfg.ConfigSnapshot{},
			upstream: structs.TestUpstreams(t)[1],
			want: &envoy.Cluster{
				Name: "prepared_query:geo-cache",
				Type: envoy.Cluster_EDS,
				EdsClusterConfig: &envoy.Cluster_EdsClusterConfig{
					EdsConfig: &envoycore.ConfigSource{
						ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
							Ads: &envoycore.AggregatedConfigSource{},
						},
					},
				},
				ConnectTimeout:   5 * time.Second, // Default
				OutlierDetection: &cluster.OutlierDetection{},
				TlsContext: &envoyauth.UpstreamTlsContext{
					CommonTlsContext: makeCommonTLSContext(&proxycfg.ConfigSnapshot{}),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			got, err := makeUpstreamCluster(tt.upstream, &tt.snap)
			require.NoError(err)

			require.Equal(tt.want, got)
		})
	}
}

package xdscommon

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	duration "google.golang.org/protobuf/types/known/durationpb"
	"testing"
)

func TestCloneIndexedResources(t *testing.T) {
	exampleResources := map[string][]proto.Message{
		ListenerType: {
			&envoy_listener_v3.Listener{
				Name:                  "listener1",
				IgnoreGlobalConnLimit: true,
				ListenerFiltersTimeout: &duration.Duration{
					Seconds: 123,
				},
			},
			&envoy_listener_v3.Listener{
				Name:       "listener2",
				StatPrefix: "stats.foo",
				ListenerFiltersTimeout: &duration.Duration{
					Seconds: 456,
				},
			},
		},
		ClusterType: {
			&envoy_cluster_v3.Cluster{
				Name:          "cluster1",
				RespectDnsTtl: true,
				TransportSocketMatches: []*envoy_cluster_v3.Cluster_TransportSocketMatch{
					{
						Name: "match1",
					},
				},
			},
		},
	}

	getPointerField := func(msg proto.Message) interface{} {
		switch typedMsg := msg.(type) {
		case *envoy_cluster_v3.Cluster:
			return typedMsg.TransportSocketMatches[0]
		case *envoy_listener_v3.Listener:
			return typedMsg.ListenerFiltersTimeout
		default:
			panic("should not happen")
		}
	}
	updatePointerField := func(msg proto.Message) {
		switch typedMsg := msg.(type) {
		case *envoy_cluster_v3.Cluster:
			typedMsg.TransportSocketMatches[0] = &envoy_cluster_v3.Cluster_TransportSocketMatch{
				Name: "match1-updated",
			}
		case *envoy_listener_v3.Listener:
			typedMsg.ListenerFiltersTimeout = &duration.Duration{
				Seconds: 999,
			}
		default:
			panic("should not happen")
		}
	}

	cases := []struct {
		name         string
		input        *IndexedResources
		hasResources bool
	}{
		{
			name:         "simple compare",
			input:        IndexResources(testutil.Logger(t), exampleResources),
			hasResources: true,
		},
		{
			name:  "empty input returns empty",
			input: EmptyIndexedResources(),
		},
		{
			name:  "nil input returns nil",
			input: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clone := Clone(tc.input)

			if tc.input == nil {
				require.Nil(t, clone)
			} else {
				if diff := cmp.Diff(tc.input, clone, protocmp.Transform()); diff != "" {
					t.Errorf("unexpected difference:\n%v", diff)
				}

				require.NotSame(t, tc.input, clone)
				require.NotSame(t, tc.input.Index, clone.Index)
				require.NotSame(t, tc.input.ChildIndex, clone.ChildIndex)

				// Ensure deep clone of protos
				for typeURL, typeMap := range tc.input.Index {
					for name, msg := range typeMap {
						require.NotSame(t, msg, clone.Index[typeURL][name])
						require.NotSame(t, getPointerField(msg), getPointerField(clone.Index[typeURL][name]))
						updatePointerField(msg)
					}
				}

				// Only check post-update difference if there are resources to differ
				if tc.hasResources {
					if diff := cmp.Diff(tc.input, clone, protocmp.Transform()); diff == "" {
						t.Errorf("updated original and clone should be different:\n%v", diff)
					}
				}
			}
		})
	}
}

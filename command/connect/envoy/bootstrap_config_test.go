package envoy

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	expectedSelfAdminCluster = `{
		"name": "self_admin",
		"connect_timeout": "5s",
		"type": "STATIC",
		"http_protocol_options": {},
		"hosts": [
			{
				"socket_address": {
					"address": "127.0.0.1",
					"port_value": 19000
				}
			}
		]
	}`
	expectedPromListener = `{
		"name": "envoy_prometheus_metrics_listener",
		"address": {
			"socket_address": {
				"address": "0.0.0.0",
				"port_value": 9000
			}
		},
		"filter_chains": [
			{
				"filters": [
					{
						"name": "envoy.http_connection_manager",
						"config": {
							"stat_prefix": "envoy_prometheus_metrics",
							"codec_type": "HTTP1",
							"route_config": {
								"name": "self_admin_route",
								"virtual_hosts": [
									{
										"name": "self_admin",
										"domains": [
											"*"
										],
										"routes": [
											{
												"match": {
													"path": "/metrics"
												},
												"route": {
													"cluster": "self_admin",
													"prefix_rewrite": "/stats/prometheus"
												}
											},
											{
												"match": {
													"prefix": "/"
												},
												"direct_response": {
													"status": 404
												}
											}
										]
									}
								]
							},
							"http_filters": [
								{
									"name": "envoy.router"
								}
							]
						}
					}
				]
			}
		]
	}`
	expectedStatsListener = `{
		"name": "envoy_metrics_listener",
		"address": {
			"socket_address": {
				"address": "0.0.0.0",
				"port_value": 9000
			}
		},
		"filter_chains": [
			{
				"filters": [
					{
						"name": "envoy.http_connection_manager",
						"config": {
							"stat_prefix": "envoy_metrics",
							"codec_type": "HTTP1",
							"route_config": {
								"name": "self_admin_route",
								"virtual_hosts": [
									{
										"name": "self_admin",
										"domains": [
											"*"
										],
										"routes": [
											{
												"match": {
													"prefix": "/stats"
												},
												"route": {
													"cluster": "self_admin",
													"prefix_rewrite": "/stats"
												}
											},
											{
												"match": {
													"prefix": "/"
												},
												"direct_response": {
													"status": 404
												}
											}
										]
									}
								]
							},
							"http_filters": [
								{
									"name": "envoy.router"
								}
							]
						}
					}
				]
			}
		]
	}`
	expectedReadyListener = `{
		"name": "envoy_ready_listener",
		"address": {
			"socket_address": {
				"address": "0.0.0.0",
				"port_value": 4444
			}
		},
		"filter_chains": [
			{
				"filters": [
					{
						"name": "envoy.http_connection_manager",
						"config": {
							"stat_prefix": "envoy_ready",
							"codec_type": "HTTP1",
							"route_config": {
								"name": "self_admin_route",
								"virtual_hosts": [
									{
										"name": "self_admin",
										"domains": [
											"*"
										],
										"routes": [
											{
												"match": {
													"path": "/ready"
												},
												"route": {
													"cluster": "self_admin",
													"prefix_rewrite": "/ready"
												}
											},
											{
												"match": {
													"prefix": "/"
												},
												"direct_response": {
													"status": 404
												}
											}
										]
									}
								]
							},
							"http_filters": [
								{
									"name": "envoy.router"
								}
							]
						}
					}
				]
			}
		]
	}`
)

func TestBootstrapConfig_ConfigureArgs(t *testing.T) {
	sniTagJSON := strings.Join(sniTagJSONs, ",\n")
	defaultStatsConfigJSON := `{
					"stats_tags": [
						` + sniTagJSON + `
					],
					"use_all_default_tags": true
				}`

	tests := []struct {
		name     string
		input    BootstrapConfig
		env      []string
		baseArgs BootstrapTplArgs
		wantArgs BootstrapTplArgs
		wantErr  bool
	}{
		{
			name:  "defaults",
			input: BootstrapConfig{},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
			},
			wantErr: false,
		},
		{
			name: "extra-stats-sinks",
			input: BootstrapConfig{
				StatsSinksJSON: `{
					"name": "envoy.custom_exciting_sink",
					"config": {
						"foo": "bar"
					}
				}`,
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.custom_exciting_sink",
					"config": {
						"foo": "bar"
					}
				}]`,
			},
		},
		{
			name: "simple-statsd-sink",
			input: BootstrapConfig{
				StatsdURL: "udp://127.0.0.1:9125",
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.statsd",
					"config": {
						"address": {
							"socket_address": {
								"address": "127.0.0.1",
								"port_value": 9125
							}
						}
					}
				}]`,
			},
			wantErr: false,
		},
		{
			name: "simple-statsd-sink-plus-extra",
			input: BootstrapConfig{
				StatsdURL: "udp://127.0.0.1:9125",
				StatsSinksJSON: `{
					"name": "envoy.custom_exciting_sink",
					"config": {
						"foo": "bar"
					}
				}`,
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.statsd",
					"config": {
						"address": {
							"socket_address": {
								"address": "127.0.0.1",
								"port_value": 9125
							}
						}
					}
				},
				{
					"name": "envoy.custom_exciting_sink",
					"config": {
						"foo": "bar"
					}
				}]`,
			},
			wantErr: false,
		},
		{
			name: "simple-statsd-sink-env",
			input: BootstrapConfig{
				StatsdURL: "$MY_STATSD_URL",
			},
			env: []string{"MY_STATSD_URL=udp://127.0.0.1:9125"},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.statsd",
					"config": {
						"address": {
							"socket_address": {
								"address": "127.0.0.1",
								"port_value": 9125
							}
						}
					}
				}]`,
			},
			wantErr: false,
		},
		{
			name: "simple-dogstatsd-sink",
			input: BootstrapConfig{
				DogstatsdURL: "udp://127.0.0.1:9125",
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.dog_statsd",
					"config": {
						"address": {
							"socket_address": {
								"address": "127.0.0.1",
								"port_value": 9125
							}
						}
					}
				}]`,
			},
			wantErr: false,
		},
		{
			name: "simple-dogstatsd-unix-sink",
			input: BootstrapConfig{
				DogstatsdURL: "unix:///var/run/dogstatsd.sock",
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.dog_statsd",
					"config": {
						"address": {
							"pipe": {
								"path": "/var/run/dogstatsd.sock"
							}
						}
					}
				}]`,
			},
			wantErr: false,
		},

		{
			name: "simple-dogstatsd-sink-env",
			input: BootstrapConfig{
				DogstatsdURL: "$MY_STATSD_URL",
			},
			env: []string{"MY_STATSD_URL=udp://127.0.0.1:9125"},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.dog_statsd",
					"config": {
						"address": {
							"socket_address": {
								"address": "127.0.0.1",
								"port_value": 9125
							}
						}
					}
				}]`,
			},
			wantErr: false,
		},
		{
			name: "stats-config-override",
			input: BootstrapConfig{
				StatsConfigJSON: `{
					"use_all_default_tags": true
				}`,
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: `{
					"use_all_default_tags": true
				}`,
			},
			wantErr: false,
		},
		{
			name: "simple-tags",
			input: BootstrapConfig{
				StatsTags: []string{"canary", "foo=bar", "baz=2"},
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: `{
					"stats_tags": [
						` + sniTagJSON + `,
						{
							"tag_name": "canary",
							"fixed_value": "1"
						},
						{
							"tag_name": "foo",
							"fixed_value": "bar"
						},
						{
							"tag_name": "baz",
							"fixed_value": "2"
						}
					],
					"use_all_default_tags": true
				}`,
			},
			wantErr: false,
		},
		{
			name: "prometheus-bind-addr",
			input: BootstrapConfig{
				PrometheusBindAddr: "0.0.0.0:9000",
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON: expectedPromListener,
				StatsConfigJSON:     defaultStatsConfigJSON,
			},
			wantErr: false,
		},
		{
			name: "prometheus-bind-addr-with-overrides",
			input: BootstrapConfig{
				PrometheusBindAddr:  "0.0.0.0:9000",
				StaticClustersJSON:  `{"foo":"bar"}`,
				StaticListenersJSON: `{"baz":"qux"}`,
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: `{"foo":"bar"},` + expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON: `{"baz":"qux"},` + expectedPromListener,
				StatsConfigJSON:     defaultStatsConfigJSON,
			},
			wantErr: false,
		},
		{
			name: "stats-bind-addr",
			input: BootstrapConfig{
				StatsBindAddr: "0.0.0.0:9000",
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON: expectedStatsListener,
				StatsConfigJSON:     defaultStatsConfigJSON,
			},
			wantErr: false,
		},
		{
			name: "stats-bind-addr-with-overrides",
			input: BootstrapConfig{
				StatsBindAddr:       "0.0.0.0:9000",
				StaticClustersJSON:  `{"foo":"bar"}`,
				StaticListenersJSON: `{"baz":"qux"}`,
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: `{"foo":"bar"},` + expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON: `{"baz":"qux"},` + expectedStatsListener,
				StatsConfigJSON:     defaultStatsConfigJSON,
			},
			wantErr: false,
		},
		{
			name: "stats-flush-interval",
			input: BootstrapConfig{
				StatsFlushInterval: `10s`,
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON:    defaultStatsConfigJSON,
				StatsFlushInterval: `10s`,
			},
			wantErr: false,
		},
		{
			name: "override-tracing",
			input: BootstrapConfig{
				TracingConfigJSON: `{"foo": "bar"}`,
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON:   defaultStatsConfigJSON,
				TracingConfigJSON: `{"foo": "bar"}`,
			},
			wantErr: false,
		},
		{
			name: "err-bad-prometheus-addr",
			input: BootstrapConfig{
				PrometheusBindAddr: "asdasdsad",
			},
			wantErr: true,
		},
		{
			name: "err-bad-stats-addr",
			input: BootstrapConfig{
				StatsBindAddr: "asdasdsad",
			},
			wantErr: true,
		},
		{
			name: "err-bad-statsd-addr",
			input: BootstrapConfig{
				StatsdURL: "asdasdsad",
			},
			wantErr: true,
		},
		{
			name: "err-bad-dogstatsd-addr",
			input: BootstrapConfig{
				DogstatsdURL: "asdasdsad",
			},
			wantErr: true,
		},
		{
			name: "ready-bind-addr",
			input: BootstrapConfig{
				ReadyBindAddr: "0.0.0.0:4444",
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON: expectedReadyListener,
				StatsConfigJSON:     defaultStatsConfigJSON,
			},
			wantErr: false,
		},
		{
			name: "ready-bind-addr-with-overrides",
			input: BootstrapConfig{
				ReadyBindAddr:       "0.0.0.0:4444",
				StaticClustersJSON:  `{"foo":"bar"}`,
				StaticListenersJSON: `{"baz":"qux"}`,
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: `{"foo":"bar"},` + expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON: `{"baz":"qux"},` + expectedReadyListener,
				StatsConfigJSON:     defaultStatsConfigJSON,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.baseArgs

			defer testSetAndResetEnv(t, tt.env)()

			err := tt.input.ConfigureArgs(&args)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Want to compare JSON fields with JSONEq
				argV := reflect.ValueOf(args)
				wantV := reflect.ValueOf(tt.wantArgs)
				argT := reflect.TypeOf(args)
				for i := 0; i < argT.NumField(); i++ {
					f := argT.Field(i)
					if strings.HasSuffix(f.Name, "JSON") && wantV.Field(i).String() != "" {
						// Some of our JSON strings are comma separated objects to be
						// insertedinto an array which is not valid JSON on it's own so wrap
						// them all in an array. For simple values this is still valid JSON
						// too.
						want := "[" + wantV.Field(i).String() + "]"
						got := "[" + argV.Field(i).String() + "]"
						require.JSONEq(t, want, got, "field %s should be equivalent JSON", f.Name)
					} else {
						require.Equalf(t, wantV.Field(i).Interface(),
							argV.Field(i).Interface(), "field %s should be equal", f.Name)
					}
				}
			}
		})
	}
}

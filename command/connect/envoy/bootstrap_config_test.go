package envoy

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	expectedSelfAdminCluster = `{
  "name": "self_admin",
  "ignore_health_on_host_removal": false,
  "connect_timeout": "5s",
  "type": "STATIC",
  "http_protocol_options": {},
  "loadAssignment": {
    "clusterName": "self_admin",
    "endpoints": [
      {
        "lbEndpoints": [
          {
            "endpoint": {
              "address": {
                "socket_address": {
                  "address": "127.0.0.1",
                  "port_value": 19000
                }
              }
            }
          }
        ]
      }
    ]
  }
}`
	expectedSelfAdminClusterNonLoopbackIP = `{
  "name": "self_admin",
  "ignore_health_on_host_removal": false,
  "connect_timeout": "5s",
  "type": "STATIC",
  "http_protocol_options": {},
  "loadAssignment": {
    "clusterName": "self_admin",
    "endpoints": [
      {
        "lbEndpoints": [
          {
            "endpoint": {
              "address": {
                "socket_address": {
                  "address": "192.0.2.10",
                  "port_value": 19002
                }
              }
            }
          }
        ]
      }
    ]
  }
}`
	expectedPrometheusBackendCluster = `{
  "name": "prometheus_backend",
  "ignore_health_on_host_removal": false,
  "connect_timeout": "5s",
  "type": "STATIC",
  "http_protocol_options": {},
  "loadAssignment": {
    "clusterName": "prometheus_backend",
    "endpoints": [
      {
        "lbEndpoints": [
          {
            "endpoint": {
              "address": {
                "socket_address": {
                  "address": "127.0.0.1",
                  "port_value": 20100
                }
              }
            }
          }
        ]
      }
    ]
  }
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
			"name": "envoy.filters.network.http_connection_manager",
			"typedConfig": {
			  "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
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
				  "name": "envoy.filters.http.router"
				}
			  ]
			}
		  }
		]
	  }
	]
  }`
	expectedPromListenerCustomScrapePath = `{
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
			"name": "envoy.filters.network.http_connection_manager",
			"typedConfig": {
			  "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
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
						  "path": "/scrape-path"
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
				  "name": "envoy.filters.http.router"
				}
			  ]
			}
		  }
		]
	  }
	]
  }`
	expectedPromListenerWithPrometheusBackendCluster = `{
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
			"name": "envoy.filters.network.http_connection_manager",
			"typedConfig": {
			  "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
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
						  "cluster": "prometheus_backend",
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
				  "name": "envoy.filters.http.router"
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
          "name": "envoy.filters.network.http_connection_manager",
          "typedConfig": {
            "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
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
                "name": "envoy.filters.http.router"
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
          "name": "envoy.filters.network.http_connection_manager",
          "typedConfig": {
            "@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager",
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
                "name": "envoy.filters.http.router"
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
	defaultTags, err := generateStatsTags(&BootstrapTplArgs{}, nil, false)
	require.NoError(t, err)

	defaultTagsJSON := strings.Join(defaultTags, ",\n")
	defaultStatsConfigJSON := formatStatsTags(defaultTags)

	// The updated tags exclude the ones deprecated in Consul 1.9
	updatedTags, err := generateStatsTags(&BootstrapTplArgs{}, nil, true)
	require.NoError(t, err)

	updatedStatsConfigJSON := formatStatsTags(updatedTags)

	tests := []struct {
		name               string
		input              BootstrapConfig
		env                []string
		baseArgs           BootstrapTplArgs
		wantArgs           BootstrapTplArgs
		omitDeprecatedTags bool
		wantErr            bool
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
					"name": "envoy.stat_sinks.statsd",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.config.metrics.v3.StatsdSink",
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
					"name": "envoy.stat_sinks.statsd",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.config.metrics.v3.StatsdSink",
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
					"name": "envoy.stat_sinks.statsd",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.config.metrics.v3.StatsdSink",
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
			name: "simple-statsd-sink-inline-env-allowed",
			input: BootstrapConfig{
				StatsdURL: "udp://$HOST_IP:9125",
			},
			env: []string{"HOST_IP=127.0.0.1"},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.stat_sinks.statsd",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.config.metrics.v3.StatsdSink",
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
			name: "simple-statsd-sink-inline-env-disallowed",
			input: BootstrapConfig{
				StatsdURL: "udp://$HOST_ADDRESS:9125",
			},
			env: []string{"HOST_ADDRESS=127.0.0.1"},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
			},
			wantErr: true,
		},
		{
			name: "simple-dogstatsd-sink",
			input: BootstrapConfig{
				DogstatsdURL: "udp://127.0.0.1:9125",
			},
			wantArgs: BootstrapTplArgs{
				StatsConfigJSON: defaultStatsConfigJSON,
				StatsSinksJSON: `[{
					"name": "envoy.stat_sinks.dog_statsd",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.config.metrics.v3.DogStatsdSink",
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
					"name": "envoy.stat_sinks.dog_statsd",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.config.metrics.v3.DogStatsdSink",
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
					"name": "envoy.stat_sinks.dog_statsd",
					"typedConfig": {
						"@type": "type.googleapis.com/envoy.config.metrics.v3.DogStatsdSink",
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
						},
						` + defaultTagsJSON + `
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
				AdminBindAddress:     "127.0.0.1",
				AdminBindPort:        "19000",
				PrometheusScrapePath: "/metrics",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON:  expectedPromListener,
				StatsConfigJSON:      defaultStatsConfigJSON,
				PrometheusScrapePath: "/metrics",
			},
			wantErr: false,
		},
		{
			name: "prometheus-bind-addr-non-loopback-ip",
			input: BootstrapConfig{
				PrometheusBindAddr: "0.0.0.0:9000",
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress:     "192.0.2.10",
				AdminBindPort:        "19002",
				PrometheusScrapePath: "/metrics",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "192.0.2.10",
				AdminBindPort:    "19002",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: expectedSelfAdminClusterNonLoopbackIP,
				// Should add a static http listener too
				StaticListenersJSON:  expectedPromListener,
				StatsConfigJSON:      defaultStatsConfigJSON,
				PrometheusScrapePath: "/metrics",
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
				AdminBindAddress:     "127.0.0.1",
				AdminBindPort:        "19000",
				PrometheusScrapePath: "/scrape-path",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: `{"foo":"bar"},` + expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON:  `{"baz":"qux"},` + expectedPromListenerCustomScrapePath,
				StatsConfigJSON:      defaultStatsConfigJSON,
				PrometheusScrapePath: "/scrape-path",
			},
			wantErr: false,
		},
		{
			name: "prometheus-bind-addr-with-prometheus-backend",
			input: BootstrapConfig{
				PrometheusBindAddr: "0.0.0.0:9000",
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress:      "127.0.0.1",
				AdminBindPort:         "19000",
				PrometheusBackendPort: "20100",
				PrometheusScrapePath:  "/metrics",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should use the "prometheus_backend" cluster instead, which
				// uses the PrometheusBackendPort rather than Envoy admin port
				StaticClustersJSON:    expectedPrometheusBackendCluster,
				StaticListenersJSON:   expectedPromListenerWithPrometheusBackendCluster,
				StatsConfigJSON:       defaultStatsConfigJSON,
				PrometheusBackendPort: "20100",
				PrometheusScrapePath:  "/metrics",
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
		{
			name: "ready-bind-addr-and-prometheus-and-stats",
			input: BootstrapConfig{
				ReadyBindAddr:      "0.0.0.0:4444",
				PrometheusBindAddr: "0.0.0.0:9000",
				StatsBindAddr:      "0.0.0.0:9000",
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress:     "127.0.0.1",
				AdminBindPort:        "19000",
				PrometheusScrapePath: "/metrics",
			},
			wantArgs: BootstrapTplArgs{
				AdminBindAddress: "127.0.0.1",
				AdminBindPort:    "19000",
				// Should add a static cluster for the self-proxy to admin
				StaticClustersJSON: expectedSelfAdminCluster,
				// Should add a static http listener too
				StaticListenersJSON: strings.Join(
					[]string{expectedPromListener, expectedStatsListener, expectedReadyListener},
					", ",
				),
				StatsConfigJSON:      defaultStatsConfigJSON,
				PrometheusScrapePath: "/metrics",
			},
			wantErr: false,
		},
		{
			name: "omit-deprecated-tags",
			input: BootstrapConfig{
				ReadyBindAddr:      "0.0.0.0:4444",
				PrometheusBindAddr: "0.0.0.0:9000",
				StatsBindAddr:      "0.0.0.0:9000",
			},
			baseArgs: BootstrapTplArgs{
				AdminBindAddress:     "127.0.0.1",
				AdminBindPort:        "19000",
				PrometheusScrapePath: "/metrics",
			},
			omitDeprecatedTags: true,
			wantArgs: BootstrapTplArgs{
				AdminBindAddress:   "127.0.0.1",
				AdminBindPort:      "19000",
				StaticClustersJSON: expectedSelfAdminCluster,
				StaticListenersJSON: strings.Join(
					[]string{expectedPromListener, expectedStatsListener, expectedReadyListener},
					", ",
				),
				// Should not have default stats config JSON when deprecated tags are omitted
				StatsConfigJSON:      updatedStatsConfigJSON,
				PrometheusScrapePath: "/metrics",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := tt.baseArgs

			defer testSetAndResetEnv(t, tt.env)()

			err := tt.input.ConfigureArgs(&args, tt.omitDeprecatedTags)
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

func TestConsulTagSpecifiers(t *testing.T) {
	// Conveniently both envoy and Go use the re2 dialect of regular
	// expressions, so we can actually test the stats tag extraction regular
	// expressions right here!

	specs, err := resourceTagSpecifiers(false)
	require.NoError(t, err)

	specsNoDeprecated, err := resourceTagSpecifiers(true)
	require.NoError(t, err)

	type testPattern struct {
		name string
		r    *regexp.Regexp
	}

	parseSpecs := func(specs []string) []testPattern {
		var patterns []testPattern
		for _, spec := range specs {
			var m struct {
				TagName string `json:"tag_name"`
				Regex   string `json:"regex"`
			}
			require.NoError(t, json.Unmarshal([]byte(spec), &m))

			patterns = append(patterns, testPattern{
				name: m.TagName,
				r:    regexp.MustCompile(m.Regex),
			})
		}
		return patterns
	}

	var (
		patterns             = parseSpecs(specs)
		patternsNoDeprecated = parseSpecs(specsNoDeprecated)
	)

	type testcase struct {
		name               string
		stat               string
		expect             map[string][]string // this is the m[1:] of the match
		expectNoDeprecated map[string][]string // this is the m[1:] of the match
	}

	cases := []testcase{
		{
			name: "cluster service",
			stat: "cluster.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors",
			expect: map[string][]string{
				"consul.custom_hash":                {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.datacenter":                 {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.custom_hash":    {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.datacenter":     {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.target":         {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong.default.dc2"},
				"consul.destination.trust_domain":   {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.full_target":                {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.namespace":                  {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.routing_type":               {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.service":                    {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.service_subset":             {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.target":                     {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong.default.dc2"},
				"consul.trust_domain":               {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
			expectNoDeprecated: map[string][]string{
				"consul.destination.custom_hash":    {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.datacenter":     {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.target":         {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong.default.dc2"},
				"consul.destination.trust_domain":   {"pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
		},
		{
			name: "cluster custom service",
			stat: "cluster.f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors",
			expect: map[string][]string{
				"consul.custom_hash":                {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.datacenter":                 {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.custom_hash":    {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.destination.datacenter":     {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.target":         {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~pong.default.dc2"},
				"consul.destination.trust_domain":   {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.full_target":                {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.namespace":                  {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.routing_type":               {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.service":                    {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.service_subset":             {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.target":                     {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~pong.default.dc2"},
				"consul.trust_domain":               {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
			expectNoDeprecated: map[string][]string{
				"consul.destination.custom_hash":    {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.destination.datacenter":     {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.target":         {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~pong.default.dc2"},
				"consul.destination.trust_domain":   {"f8f8f8f8~pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
		},
		{
			name: "cluster service subset",
			stat: "cluster.v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors",
			expect: map[string][]string{
				"consul.custom_hash":                {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.datacenter":                 {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.custom_hash":    {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.datacenter":     {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.destination.target":         {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2.pong.default.dc2"},
				"consul.destination.trust_domain":   {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.full_target":                {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.namespace":                  {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.routing_type":               {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.service":                    {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.service_subset":             {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.target":                     {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2.pong.default.dc2"},
				"consul.trust_domain":               {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
			expectNoDeprecated: map[string][]string{
				"consul.destination.custom_hash":    {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.datacenter":     {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.destination.target":         {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2.pong.default.dc2"},
				"consul.destination.trust_domain":   {"v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
		},
		{
			name: "cluster custom service subset",
			stat: "cluster.f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors",
			expect: map[string][]string{
				"consul.custom_hash":                {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.datacenter":                 {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.custom_hash":    {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.destination.datacenter":     {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.destination.target":         {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.dc2"},
				"consul.destination.trust_domain":   {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.full_target":                {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.namespace":                  {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.routing_type":               {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.service":                    {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.service_subset":             {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.target":                     {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.dc2"},
				"consul.trust_domain":               {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
			expectNoDeprecated: map[string][]string{
				"consul.destination.custom_hash":    {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.destination.datacenter":     {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", ""},
				"consul.destination.routing_type":   {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal"},
				"consul.destination.service":        {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.destination.target":         {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.dc2"},
				"consul.destination.trust_domain":   {"f8f8f8f8~v2.pong.default.dc2.internal.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
		},
		{
			name: "cluster custom service subset non-default partition",
			stat: "cluster.f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.bind_errors",
			expect: map[string][]string{
				"consul.custom_hash":                {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.datacenter":                 {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.custom_hash":    {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.destination.datacenter":     {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "partA"},
				"consul.destination.routing_type":   {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal-v1"},
				"consul.destination.service":        {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.destination.target":         {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.partA.dc2"},
				"consul.destination.trust_domain":   {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.full_target":                {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.namespace":                  {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.routing_type":               {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal-v1"},
				"consul.service":                    {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.service_subset":             {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.target":                     {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.partA.dc2"},
				"consul.trust_domain":               {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
			expectNoDeprecated: map[string][]string{
				"consul.destination.custom_hash":    {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8"},
				"consul.destination.datacenter":     {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "dc2"},
				"consul.destination.full_target":    {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648"},
				"consul.destination.namespace":      {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "default"},
				"consul.destination.partition":      {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "partA"},
				"consul.destination.routing_type":   {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "internal-v1"},
				"consul.destination.service":        {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "pong"},
				"consul.destination.service_subset": {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "v2"},
				"consul.destination.target":         {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "f8f8f8f8~v2.pong.default.partA.dc2"},
				"consul.destination.trust_domain":   {"f8f8f8f8~v2.pong.default.partA.dc2.internal-v1.e5b08d03-bfc3-c870-1833-baddb116e648.consul.", "e5b08d03-bfc3-c870-1833-baddb116e648"},
			},
		},
		{
			name: "tcp listener no namespace or partition (OSS)",
			stat: "tcp.upstream.db.dc1.downstream_cx_total",
			expect: map[string][]string{
				"consul.upstream.datacenter": {"db.dc1.", "dc1"},
				"consul.upstream.namespace":  {"db.dc1.", ""},
				"consul.upstream.partition":  {"db.dc1.", ""},
				"consul.upstream.service":    {"db.dc1.", "db"},
			},
		},
		{
			name: "tcp listener with namespace and partition",
			stat: "tcp.upstream.db.frontend.west.dc1.downstream_cx_total",
			expect: map[string][]string{
				"consul.upstream.datacenter": {"db.frontend.west.dc1.", "dc1"},
				"consul.upstream.namespace":  {"db.frontend.west.dc1.", "frontend"},
				"consul.upstream.partition":  {"db.frontend.west.dc1.", "west"},
				"consul.upstream.service":    {"db.frontend.west.dc1.", "db"},
			},
		},
		{
			name: "http listener no namespace or partition (OSS)",
			stat: "http.upstream.web.dc1.downstream_cx_total",
			expect: map[string][]string{
				"consul.upstream.datacenter": {"web.dc1.", "dc1"},
				"consul.upstream.namespace":  {"web.dc1.", ""},
				"consul.upstream.partition":  {"web.dc1.", ""},
				"consul.upstream.service":    {"web.dc1.", "web"},
			},
		},
		{
			name: "http listener with namespace and partition",
			stat: "http.upstream.web.frontend.west.dc1.downstream_cx_total",
			expect: map[string][]string{
				"consul.upstream.datacenter": {"web.frontend.west.dc1.", "dc1"},
				"consul.upstream.namespace":  {"web.frontend.west.dc1.", "frontend"},
				"consul.upstream.partition":  {"web.frontend.west.dc1.", "west"},
				"consul.upstream.service":    {"web.frontend.west.dc1.", "web"},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var (
				got             = make(map[string][]string)
				gotNoDeprecated = make(map[string][]string)
			)
			for _, p := range patterns {
				m := p.r.FindStringSubmatch(tc.stat)
				if len(m) > 1 {
					m = m[1:]
					got[p.name] = m
				}
			}
			for _, p := range patternsNoDeprecated {
				m := p.r.FindStringSubmatch(tc.stat)
				if len(m) > 1 {
					m = m[1:]
					gotNoDeprecated[p.name] = m
				}
			}

			if tc.expectNoDeprecated == nil {
				tc.expectNoDeprecated = tc.expect
			}

			assert.Equal(t, tc.expect, got)
			assert.Equal(t, tc.expectNoDeprecated, gotNoDeprecated)
		})
	}
}

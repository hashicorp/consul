package structs

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/hcl"
	"github.com/stretchr/testify/require"
)

// TestDecodeConfigEntry is the 'structs' mirror image of
// command/config/write/config_write_test.go:TestParseConfigEntry
func TestDecodeConfigEntry(t *testing.T) {

	for _, tc := range []struct {
		name      string
		camel     string
		snake     string
		expect    ConfigEntry
		expectErr string
	}{
		// TODO(rb): test json?
		{
			name: "proxy-defaults: extra fields or typo",
			snake: `
				kind = "proxy-defaults"
				name = "main"
				cornfig {
				  "foo" = 19
				}
			`,
			camel: `
				Kind = "proxy-defaults"
				Name = "main"
				Cornfig {
				  "foo" = 19
				}
			`,
			expectErr: `invalid config key "cornfig"`,
		},
		{
			name: "proxy-defaults",
			snake: `
				kind = "proxy-defaults"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				config {
				  "foo" = 19
				  "bar" = "abc"
				  "moreconfig" {
					"moar" = "config"
				  }
				}
				mesh_gateway {
					mode = "remote"
				}
			`,
			camel: `
				Kind = "proxy-defaults"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Config {
				  "foo" = 19
				  "bar" = "abc"
				  "moreconfig" {
					"moar" = "config"
				  }
				}
				MeshGateway {
					Mode = "remote"
				}
			`,
			expect: &ProxyConfigEntry{
				Kind: "proxy-defaults",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Config: map[string]interface{}{
					"foo": 19,
					"bar": "abc",
					"moreconfig": map[string]interface{}{
						"moar": "config",
					},
				},
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeRemote,
				},
			},
		},
		{
			name: "service-defaults",
			snake: `
				kind = "service-defaults"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				protocol = "http"
				external_sni = "abc-123"
				mesh_gateway {
					mode = "remote"
				}
			`,
			camel: `
				Kind = "service-defaults"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Protocol = "http"
				ExternalSNI = "abc-123"
				MeshGateway {
					Mode = "remote"
				}
			`,
			expect: &ServiceConfigEntry{
				Kind: "service-defaults",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Protocol:    "http",
				ExternalSNI: "abc-123",
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeRemote,
				},
			},
		},
		{
			name: "service-router: kitchen sink",
			snake: `
				kind = "service-router"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				routes = [
					{
						match {
							http {
								path_exact = "/foo"
								header = [
									{
										name = "debug1"
										present = true
									},
									{
										name = "debug2"
										present = false
										invert = true
									},
									{
										name = "debug3"
										exact = "1"
									},
									{
										name = "debug4"
										prefix = "aaa"
									},
									{
										name = "debug5"
										suffix = "bbb"
									},
									{
										name = "debug6"
										regex = "a.*z"
									},
								]
							}
						}
						destination {
						  service               = "carrot"
						  service_subset         = "kale"
						  namespace             = "leek"
						  prefix_rewrite         = "/alternate"
						  request_timeout        = "99s"
						  num_retries            = 12345
						  retry_on_connect_failure = true
						  retry_on_status_codes    = [401, 209]
						}
					},
					{
						match {
							http {
								path_prefix = "/foo"
								methods = [ "GET", "DELETE" ]
								query_param = [
									{
										name = "hack1"
										present = true
									},
									{
										name = "hack2"
										exact = "1"
									},
									{
										name = "hack3"
										regex = "a.*z"
									},
								]
							}
						}
					},
					{
						match {
							http {
								path_regex = "/foo"
							}
						}
					},
				]
			`,
			camel: `
				Kind = "service-router"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Routes = [
					{
						Match {
							HTTP {
								PathExact = "/foo"
								Header = [
									{
										Name = "debug1"
										Present = true
									},
									{
										Name = "debug2"
										Present = false
										Invert = true
									},
									{
										Name = "debug3"
										Exact = "1"
									},
									{
										Name = "debug4"
										Prefix = "aaa"
									},
									{
										Name = "debug5"
										Suffix = "bbb"
									},
									{
										Name = "debug6"
										Regex = "a.*z"
									},
								]
							}
						}
						Destination {
						  Service               = "carrot"
						  ServiceSubset         = "kale"
						  Namespace             = "leek"
						  PrefixRewrite         = "/alternate"
						  RequestTimeout        = "99s"
						  NumRetries            = 12345
						  RetryOnConnectFailure = true
						  RetryOnStatusCodes    = [401, 209]
						}
					},
					{
						Match {
							HTTP {
								PathPrefix = "/foo"
								Methods = [ "GET", "DELETE" ]
								QueryParam = [
									{
										Name = "hack1"
										Present = true
									},
									{
										Name = "hack2"
										Exact = "1"
									},
									{
										Name = "hack3"
										Regex = "a.*z"
									},
								]
							}
						}
					},
					{
						Match {
							HTTP {
								PathRegex = "/foo"
							}
						}
					},
				]
			`,
			expect: &ServiceRouterConfigEntry{
				Kind: "service-router",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Routes: []ServiceRoute{
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathExact: "/foo",
								Header: []ServiceRouteHTTPMatchHeader{
									{
										Name:    "debug1",
										Present: true,
									},
									{
										Name:    "debug2",
										Present: false,
										Invert:  true,
									},
									{
										Name:  "debug3",
										Exact: "1",
									},
									{
										Name:   "debug4",
										Prefix: "aaa",
									},
									{
										Name:   "debug5",
										Suffix: "bbb",
									},
									{
										Name:  "debug6",
										Regex: "a.*z",
									},
								},
							},
						},
						Destination: &ServiceRouteDestination{
							Service:               "carrot",
							ServiceSubset:         "kale",
							Namespace:             "leek",
							PrefixRewrite:         "/alternate",
							RequestTimeout:        99 * time.Second,
							NumRetries:            12345,
							RetryOnConnectFailure: true,
							RetryOnStatusCodes:    []uint32{401, 209},
						},
					},
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathPrefix: "/foo",
								Methods:    []string{"GET", "DELETE"},
								QueryParam: []ServiceRouteHTTPMatchQueryParam{
									{
										Name:    "hack1",
										Present: true,
									},
									{
										Name:  "hack2",
										Exact: "1",
									},
									{
										Name:  "hack3",
										Regex: "a.*z",
									},
								},
							},
						},
					},
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathRegex: "/foo",
							},
						},
					},
				},
			},
		},
		{
			name: "service-splitter: kitchen sink",
			snake: `
				kind = "service-splitter"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				splits = [
				  {
					weight        = 99.1
					service_subset = "v1"
				  },
				  {
					weight    = 0.9
					service   = "other"
					namespace = "alt"
				  },
				]
			`,
			camel: `
				Kind = "service-splitter"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Splits = [
				  {
					Weight        = 99.1
					ServiceSubset = "v1"
				  },
				  {
					Weight    = 0.9
					Service   = "other"
					Namespace = "alt"
				  },
				]
			`,
			expect: &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Splits: []ServiceSplit{
					{
						Weight:        99.1,
						ServiceSubset: "v1",
					},
					{
						Weight:    0.9,
						Service:   "other",
						Namespace: "alt",
					},
				},
			},
		},
		{
			name: "service-resolver: subsets with failover",
			snake: `
				kind = "service-resolver"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				default_subset = "v1"
				connect_timeout = "15s"
				subsets = {
					"v1" = {
						filter = "Service.Meta.version == v1"
					},
					"v2" = {
						filter = "Service.Meta.version == v2"
						only_passing = true
					},
				}
				failover = {
					"v2" = {
						service = "failcopy"
						service_subset = "sure"
						namespace = "neighbor"
						datacenters = ["dc5", "dc14"]
					},
					"*" = {
						datacenters = ["dc7"]
					}
				}`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				DefaultSubset = "v1"
				ConnectTimeout = "15s"
				Subsets = {
					"v1" = {
						Filter = "Service.Meta.version == v1"
					},
					"v2" = {
						Filter = "Service.Meta.version == v2"
						OnlyPassing = true
					},
				}
				Failover = {
					"v2" = {
						Service = "failcopy"
						ServiceSubset = "sure"
						Namespace = "neighbor"
						Datacenters = ["dc5", "dc14"]
					},
					"*" = {
						Datacenters = ["dc7"]
					}
				}`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				DefaultSubset:  "v1",
				ConnectTimeout: 15 * time.Second,
				Subsets: map[string]ServiceResolverSubset{
					"v1": {
						Filter: "Service.Meta.version == v1",
					},
					"v2": {
						Filter:      "Service.Meta.version == v2",
						OnlyPassing: true,
					},
				},
				Failover: map[string]ServiceResolverFailover{
					"v2": {
						Service:       "failcopy",
						ServiceSubset: "sure",
						Namespace:     "neighbor",
						Datacenters:   []string{"dc5", "dc14"},
					},
					"*": {
						Datacenters: []string{"dc7"},
					},
				},
			},
		},
		{
			name: "service-resolver: redirect",
			snake: `
				kind = "service-resolver"
				name = "main"
				redirect {
					service = "other"
					service_subset = "backup"
					namespace = "alt"
					datacenter = "dc9"
				}
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				Redirect {
					Service = "other"
					ServiceSubset = "backup"
					Namespace = "alt"
					Datacenter = "dc9"
				}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				Redirect: &ServiceResolverRedirect{
					Service:       "other",
					ServiceSubset: "backup",
					Namespace:     "alt",
					Datacenter:    "dc9",
				},
			},
		},
		{
			name: "service-resolver: default",
			snake: `
				kind = "service-resolver"
				name = "main"
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
			},
		},
		{
			name: "service-resolver: envoy hash lb kitchen sink",
			snake: `
				kind = "service-resolver"
				name = "main"
				load_balancer = {
					policy = "ring_hash"
					ring_hash_config = {
						minimum_ring_size = 1
						maximum_ring_size = 2
					}
					hash_policies = [
						{
							field = "cookie"
							field_value = "good-cookie"
							cookie_config = {
								ttl = "1s"
								path = "/oven"
							}
							terminal = true
						},
						{
							field = "cookie"
							field_value = "less-good-cookie"
							cookie_config = {
								session = true
								path = "/toaster"
							}
							terminal = true
						},
						{
							field = "header"
							field_value = "x-user-id"
						},
						{
							source_ip = true
						}
					]
				}
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				LoadBalancer = {
					Policy = "ring_hash"
					RingHashConfig = {
						MinimumRingSize = 1
						MaximumRingSize = 2
					}
					HashPolicies = [
						{
							Field = "cookie"
							FieldValue = "good-cookie"
							CookieConfig = {
								TTL = "1s"
								Path = "/oven"
							}
							Terminal = true
						},
						{
							Field = "cookie"
							FieldValue = "less-good-cookie"
							CookieConfig = {
								Session = true
								Path = "/toaster"
							}
							Terminal = true
						},
						{
							Field = "header"
							FieldValue = "x-user-id"
						},
						{
							SourceIP = true
						}
					]
				}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyRingHash,
					RingHashConfig: &RingHashConfig{
						MinimumRingSize: 1,
						MaximumRingSize: 2,
					},
					HashPolicies: []HashPolicy{
						{
							Field:      HashPolicyCookie,
							FieldValue: "good-cookie",
							CookieConfig: &CookieConfig{
								TTL:  1 * time.Second,
								Path: "/oven",
							},
							Terminal: true,
						},
						{
							Field:      HashPolicyCookie,
							FieldValue: "less-good-cookie",
							CookieConfig: &CookieConfig{
								Session: true,
								Path:    "/toaster",
							},
							Terminal: true,
						},
						{
							Field:      HashPolicyHeader,
							FieldValue: "x-user-id",
						},
						{
							SourceIP: true,
						},
					},
				},
			},
		},
		{
			name: "service-resolver: envoy least request kitchen sink",
			snake: `
				kind = "service-resolver"
				name = "main"
				load_balancer = {
					policy = "least_request"
					least_request_config = {
						choice_count = 2
					}
				}
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				LoadBalancer = {
					Policy = "least_request"
					LeastRequestConfig = {
						ChoiceCount = 2
					}
				}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyLeastRequest,
					LeastRequestConfig: &LeastRequestConfig{
						ChoiceCount: 2,
					},
				},
			},
		},
		{
			name: "ingress-gateway: kitchen sink",
			snake: `
				kind = "ingress-gateway"
				name = "ingress-web"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}

				tls {
					enabled = true
				}

				listeners = [
					{
						port = 8080
						protocol = "http"
						services = [
							{
								name = "web"
								hosts = ["test.example.com", "test2.example.com"]
							},
							{
								name = "db"
							}
						]
					},
					{
						port = 9999
						protocol = "tcp"
						services = [
							{
								name = "mysql"
							}
						]
					},
					{
						port = 2234
						protocol = "tcp"
						services = [
							{
								name = "postgres"
							}
						]
					}
				]
			`,
			camel: `
				Kind = "ingress-gateway"
				Name = "ingress-web"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				TLS {
					Enabled = true
				}
				Listeners = [
					{
						Port = 8080
						Protocol = "http"
						Services = [
							{
								Name = "web"
								Hosts = ["test.example.com", "test2.example.com"]
							},
							{
								Name = "db"
							}
						]
					},
					{
						Port = 9999
						Protocol = "tcp"
						Services = [
							{
								Name = "mysql"
							}
						]
					},
					{
						Port = 2234
						Protocol = "tcp"
						Services = [
							{
								Name = "postgres"
							}
						]
					}
				]
			`,
			expect: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				TLS: GatewayTLSConfig{
					Enabled: true,
				},
				Listeners: []IngressListener{
					{
						Port:     8080,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "web",
								Hosts: []string{"test.example.com", "test2.example.com"},
							},
							{
								Name: "db",
							},
						},
					},
					{
						Port:     9999,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "mysql",
							},
						},
					},
					{
						Port:     2234,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "postgres",
							},
						},
					},
				},
			},
		},
		{
			name: "terminating-gateway: kitchen sink",
			snake: `
				kind = "terminating-gateway"
				name = "terminating-gw-west"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				services = [
					{
						name = "payments",
						ca_file = "/etc/payments/ca.pem",
						cert_file = "/etc/payments/cert.pem",
						key_file = "/etc/payments/tls.key",
						sni = "mydomain",
					},
					{
						name = "*",
						ca_file = "/etc/all/ca.pem",
						cert_file = "/etc/all/cert.pem",
						key_file = "/etc/all/tls.key",
						sni = "my-alt-domain",
					},
				]
			`,
			camel: `
				Kind = "terminating-gateway"
				Name = "terminating-gw-west"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Services = [
					{
						Name = "payments",
						CAFile = "/etc/payments/ca.pem",
						CertFile = "/etc/payments/cert.pem",
						KeyFile = "/etc/payments/tls.key",
						SNI = "mydomain",
					},
					{
						Name = "*",
						CAFile = "/etc/all/ca.pem",
						CertFile = "/etc/all/cert.pem",
						KeyFile = "/etc/all/tls.key",
						SNI = "my-alt-domain",
					},
				]
			`,
			expect: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Services: []LinkedService{
					{
						Name:     "payments",
						CAFile:   "/etc/payments/ca.pem",
						CertFile: "/etc/payments/cert.pem",
						KeyFile:  "/etc/payments/tls.key",
						SNI:      "mydomain",
					},
					{
						Name:     "*",
						CAFile:   "/etc/all/ca.pem",
						CertFile: "/etc/all/cert.pem",
						KeyFile:  "/etc/all/tls.key",
						SNI:      "my-alt-domain",
					},
				},
			},
		},
		{
			name: "service-intentions: kitchen sink",
			snake: `
				kind = "service-intentions"
				name = "web"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				sources = [
				  {
					name        = "foo"
					action      = "deny"
					type        = "consul"
					description = "foo desc"
				  },
				  {
					name        = "bar"
					action      = "allow"
					description = "bar desc"
				  },
				  {
					name = "l7"
					permissions = [
					  {
						action = "deny"
						http {
						  path_exact = "/admin"
						  header = [
							{
							  name    = "hdr-present"
							  present = true
							},
							{
							  name  = "hdr-exact"
							  exact = "exact"
							},
							{
							  name   = "hdr-prefix"
							  prefix = "prefix"
							},
							{
							  name   = "hdr-suffix"
							  suffix = "suffix"
							},
							{
							  name  = "hdr-regex"
							  regex = "regex"
							},
							{
							  name    = "hdr-absent"
							  present = true
							  invert  = true
							}
						  ]
						}
					  },
					  {
						action = "allow"
						http {
						  path_prefix = "/v3/"
						}
					  },
					  {
						action = "allow"
						http {
						  path_regex = "/v[12]/.*"
						  methods    = ["GET", "POST"]
						}
					  }
					]
				  }
				]
				sources {
				  name        = "*"
				  action      = "deny"
				  description = "wild desc"
				}
			`,
			camel: `
				Kind = "service-intentions"
				Name = "web"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Sources = [
				  {
					Name        = "foo"
					Action      = "deny"
					Type        = "consul"
					Description = "foo desc"
				  },
				  {
					Name        = "bar"
					Action      = "allow"
					Description = "bar desc"
				  },
				  {
					Name = "l7"
					Permissions = [
					  {
						Action = "deny"
						HTTP {
						  PathExact = "/admin"
						  Header = [
							{
							  Name    = "hdr-present"
							  Present = true
							},
							{
							  Name  = "hdr-exact"
							  Exact = "exact"
							},
							{
							  Name   = "hdr-prefix"
							  Prefix = "prefix"
							},
							{
							  Name   = "hdr-suffix"
							  Suffix = "suffix"
							},
							{
							  Name  = "hdr-regex"
							  Regex = "regex"
							},
							{
							  Name    = "hdr-absent"
							  Present = true
							  Invert  = true
							}
						  ]
						}
					  },
					  {
						Action = "allow"
						HTTP {
						  PathPrefix = "/v3/"
						}
					  },
					  {
						Action = "allow"
						HTTP {
						  PathRegex = "/v[12]/.*"
						  Methods   = ["GET", "POST"]
						}
					  }
					]
				  }
				]
				Sources {
				  Name        = "*"
				  Action      = "deny"
				  Description = "wild desc"
				}
			`,
			expect: &ServiceIntentionsConfigEntry{
				Kind: "service-intentions",
				Name: "web",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Sources: []*SourceIntention{
					{
						Name:        "foo",
						Action:      "deny",
						Type:        "consul",
						Description: "foo desc",
					},
					{
						Name:        "bar",
						Action:      "allow",
						Description: "bar desc",
					},
					{
						Name: "l7",
						Permissions: []*IntentionPermission{
							{
								Action: "deny",
								HTTP: &IntentionHTTPPermission{
									PathExact: "/admin",
									Header: []IntentionHTTPHeaderPermission{
										{
											Name:    "hdr-present",
											Present: true,
										},
										{
											Name:  "hdr-exact",
											Exact: "exact",
										},
										{
											Name:   "hdr-prefix",
											Prefix: "prefix",
										},
										{
											Name:   "hdr-suffix",
											Suffix: "suffix",
										},
										{
											Name:  "hdr-regex",
											Regex: "regex",
										},
										{
											Name:    "hdr-absent",
											Present: true,
											Invert:  true,
										},
									},
								},
							},
							{
								Action: "allow",
								HTTP: &IntentionHTTPPermission{
									PathPrefix: "/v3/",
								},
							},
							{
								Action: "allow",
								HTTP: &IntentionHTTPPermission{
									PathRegex: "/v[12]/.*",
									Methods:   []string{"GET", "POST"},
								},
							},
						},
					},
					{
						Name:        "*",
						Action:      "deny",
						Description: "wild desc",
					},
				},
			},
		},
		{
			name: "service-intentions: wildcard destination",
			snake: `
				kind = "service-intentions"
				name = "*"
				sources {
				  name   = "foo"
				  action = "deny"
				  # should be parsed, but we'll ignore it later
				  precedence = 6
				}
			`,
			camel: `
				Kind = "service-intentions"
				Name = "*"
				Sources {
				  Name   = "foo"
				  Action = "deny"
				  # should be parsed, but we'll ignore it later
				  Precedence = 6
				}
			`,
			expect: &ServiceIntentionsConfigEntry{
				Kind: "service-intentions",
				Name: "*",
				Sources: []*SourceIntention{
					{
						Name:       "foo",
						Action:     "deny",
						Precedence: 6,
					},
				},
			},
		},
	} {
		tc := tc

		testbody := func(t *testing.T, body string) {
			var raw map[string]interface{}
			err := hcl.Decode(&raw, body)
			require.NoError(t, err)

			got, err := DecodeConfigEntry(raw)
			if tc.expectErr != "" {
				require.Nil(t, got)
				require.Error(t, err)
				requireContainsLower(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			}
		}

		t.Run(tc.name+" (snake case)", func(t *testing.T) {
			testbody(t, tc.snake)
		})
		t.Run(tc.name+" (camel case)", func(t *testing.T) {
			testbody(t, tc.camel)
		})
	}
}

func TestServiceConfigResponse_MsgPack(t *testing.T) {
	// TODO(banks) lib.MapWalker doesn't actually fix the map[interface{}] issue
	// it claims to in docs yet. When it does uncomment those cases below.
	a := ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"string": "foo",
			// "map": map[string]interface{}{
			// 	"baz": "bar",
			// },
		},
		UpstreamConfigs: map[string]map[string]interface{}{
			"a": {
				"string": "aaaa",
				// "map": map[string]interface{}{
				// 	"baz": "aa",
				// },
			},
			"b": {
				"string": "bbbb",
				// "map": map[string]interface{}{
				// 	"baz": "bb",
				// },
			},
		},
	}

	var buf bytes.Buffer

	// Encode as msgPack using a regular handle i.e. NOT one with RawAsString
	// since our RPC codec doesn't use that.
	enc := codec.NewEncoder(&buf, MsgpackHandle)
	require.NoError(t, enc.Encode(&a))

	var b ServiceConfigResponse

	dec := codec.NewDecoder(&buf, MsgpackHandle)
	require.NoError(t, dec.Decode(&b))

	require.Equal(t, a, b)
}

func TestConfigEntryResponseMarshalling(t *testing.T) {

	cases := map[string]ConfigEntryResponse{
		"nil entry": {},
		"proxy-default entry": {
			Entry: &ProxyConfigEntry{
				Kind: ProxyDefaults,
				Name: ProxyConfigGlobal,
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		"service-default entry": {
			Entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "foo",
				Protocol: "tcp",
				// Connect:  ConnectConfiguration{SideCarProxy: true},
			},
		},
	}

	for name, tcase := range cases {
		name := name
		tcase := tcase
		t.Run(name, func(t *testing.T) {

			data, err := tcase.MarshalBinary()
			require.NoError(t, err)
			require.NotEmpty(t, data)

			var resp ConfigEntryResponse
			require.NoError(t, resp.UnmarshalBinary(data))

			require.Equal(t, tcase, resp)
		})
	}
}

func requireContainsLower(t *testing.T, haystack, needle string) {
	t.Helper()
	require.Contains(t, strings.ToLower(haystack), strings.ToLower(needle))
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package tfgen

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/secrets"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

func (g *Generator) generateAgentHCL(node *topology.Node) (string, error) {
	if !node.IsAgent() {
		return "", fmt.Errorf("not an agent")
	}

	cluster, ok := g.topology.Clusters[node.Cluster]
	if !ok {
		return "", fmt.Errorf("no such cluster: %s", node.Cluster)
	}

	var b HCLBuilder

	b.add("server", node.IsServer())
	b.add("bind_addr", "0.0.0.0")
	b.add("client_addr", "0.0.0.0")
	b.add("advertise_addr", `{{ GetInterfaceIP "eth0" }}`)
	b.add("datacenter", node.Datacenter)
	b.add("disable_update_check", true)
	b.add("log_level", "trace")
	b.add("enable_debug", true)
	b.add("use_streaming_backend", true)

	// speed up leaves
	b.addBlock("performance", func() {
		b.add("leave_drain_time", "50ms")
	})

	b.add("primary_datacenter", node.Datacenter)

	// Using retry_join here is bad because changing server membership will
	// destroy and recreate all of the servers
	// if !node.IsServer() {
	b.addSlice("retry_join", []string{"server." + node.Cluster + "-consulcluster.lan"})
	b.add("retry_interval", "1s")
	// }

	if node.IsServer() {
		b.addBlock("peering", func() {
			b.add("enabled", true)
		})
	}

	b.addBlock("ui_config", func() {
		b.add("enabled", true)
	})

	b.addBlock("telemetry", func() {
		b.add("disable_hostname", true)
		b.add("prometheus_retention_time", "168h")
	})

	b.add("encrypt", g.sec.ReadGeneric(node.Cluster, secrets.GossipKey))

	{
		var (
			root     = "/consul/config/certs"
			caFile   = root + "/consul-agent-ca.pem"
			certFile = root + "/" + node.TLSCertPrefix + ".pem"
			certKey  = root + "/" + node.TLSCertPrefix + "-key.pem"
		)

		b.addBlock("tls", func() {
			b.addBlock("internal_rpc", func() {
				b.add("ca_file", caFile)
				b.add("cert_file", certFile)
				b.add("key_file", certKey)
				b.add("verify_incoming", true)
				b.add("verify_server_hostname", true)
				b.add("verify_outgoing", true)
			})
			// if cfg.EncryptionTLSAPI {
			// 	b.addBlock("https", func() {
			// 		b.add("ca_file", caFile)
			// 		b.add("cert_file", certFile)
			// 		b.add("key_file", certKey)
			// 		// b.add("verify_incoming", true)
			// 	})
			// }
			if node.IsServer() {
				b.addBlock("grpc", func() {
					b.add("ca_file", caFile)
					b.add("cert_file", certFile)
					b.add("key_file", certKey)
					// b.add("verify_incoming", true)
				})
			}
		})
	}

	b.addBlock("ports", func() {
		if node.IsServer() {
			b.add("grpc_tls", 8503)
			b.add("grpc", -1)
		} else {
			b.add("grpc", 8502)
			b.add("grpc_tls", -1)
		}
		b.add("http", 8500)
		b.add("dns", 8600)
	})

	b.addSlice("recursors", []string{"8.8.8.8"})

	b.addBlock("acl", func() {
		b.add("enabled", true)
		b.add("default_policy", "deny")
		b.add("down_policy", "extend-cache")
		b.add("enable_token_persistence", true)
		b.addBlock("tokens", func() {
			if node.IsServer() {
				b.add("initial_management", g.sec.ReadGeneric(node.Cluster, secrets.BootstrapToken))
			}
			b.add("agent_recovery", g.sec.ReadGeneric(node.Cluster, secrets.AgentRecovery))
			b.add("agent", g.sec.ReadAgentToken(node.Cluster, node.ID()))
		})
	})

	if node.IsServer() {
		b.add("bootstrap_expect", len(cluster.ServerNodes()))
		// b.add("translate_wan_addrs", true)
		b.addBlock("rpc", func() {
			b.add("enable_streaming", true)
		})
		if node.HasPublicAddress() {
			b.add("advertise_addr_wan", `{{ GetInterfaceIP "eth1" }}`) // note: can't use 'node.PublicAddress()' b/c we don't know that yet
		}

		// Exercise config entry bootstrap
		// b.addBlock("config_entries", func() {
		// 	b.addBlock("bootstrap", func() {
		// 		b.add("kind", "service-defaults")
		// 		b.add("name", "placeholder")
		// 		b.add("protocol", "grpc")
		// 	})
		// 	b.addBlock("bootstrap", func() {
		// 		b.add("kind", "service-intentions")
		// 		b.add("name", "placeholder")
		// 		b.addBlock("sources", func() {
		// 			b.add("name", "placeholder-client")
		// 			b.add("action", "allow")
		// 		})
		// 	})
		// })

		b.addBlock("connect", func() {
			b.add("enabled", true)
		})

	} else {
		if cluster.Enterprise {
			b.add("partition", node.Partition)
		}
	}

	return b.String(), nil
}

type HCLBuilder struct {
	parts []string
}

func (b *HCLBuilder) format(s string, a ...any) {
	if len(a) == 0 {
		b.parts = append(b.parts, s)
	} else {
		b.parts = append(b.parts, fmt.Sprintf(s, a...))
	}
}

func (b *HCLBuilder) add(k string, v any) {
	switch x := v.(type) {
	case string:
		if x != "" {
			b.format("%s = %q", k, x)
		}
	case int:
		b.format("%s = %d", k, x)
	case bool:
		b.format("%s = %v", k, x)
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

func (b *HCLBuilder) addBlock(block string, fn func()) {
	b.format(block + "{")
	fn()
	b.format("}")
}

func (b *HCLBuilder) addSlice(name string, vals []string) {
	b.format(name + " = [")
	for _, v := range vals {
		b.format("%q,", v)
	}
	b.format("]")
}

func (b *HCLBuilder) String() string {
	joined := strings.Join(b.parts, "\n")
	// Ensure it looks tidy
	return string(hclwrite.Format([]byte(joined)))
}

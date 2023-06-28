[![GoDoc](https://pkg.go.dev/badge/github.com/hashicorp/consul/testingconsul)](https://pkg.go.dev/github.com/hashicorp/consul/testingconsul)

## Summary

This is a Go library used to launch one or more Consul clusters that can be
peered using the cluster peering feature. Under the covers `terraform` is used
in conjunction with the
[`kreuzwerker/docker`](https://registry.terraform.io/providers/kreuzwerker/docker/latest)
provider to manage a fleet of local docker containers and networks.

### Configuration

The complete topology of Consul clusters is defined using a topology.Config
which allows you to define a set of networks and reference those networks when
assigning nodes and services to clusters. Both Consul clients and
`consul-dataplane` instances are supported.

Here is an example configuration with two peered clusters:

```
cfg := &topology.Config{
    Networks: []*topology.Network{
        {Name: "dc1"},
        {Name: "dc2"},
        {Name: "wan", Type: "wan"},
    },
    Clusters: []*topology.Cluster{
        {
            Name: "dc1",
            Nodes: []*topology.Node{
                {
                    Kind: topology.NodeKindServer,
                    Name: "dc1-server1",
                    Addresses: []*topology.Address{
                        {Network: "dc1"},
                        {Network: "wan"},
                    },
                },
                {
                    Kind: topology.NodeKindClient,
                    Name: "dc1-client1",
                    Services: []*topology.Service{
                        {
                            ID:             topology.ServiceID{Name: "mesh-gateway"},
                            Port:           8443,
                            EnvoyAdminPort: 19000,
                            IsMeshGateway:  true,
                        },
                    },
                },
                {
                    Kind: topology.NodeKindClient,
                    Name: "dc1-client2",
                    Services: []*topology.Service{
                        {
                            ID:             topology.ServiceID{Name: "ping"},
                            Image:          "rboyer/pingpong:latest",
                            Port:           8080,
                            EnvoyAdminPort: 19000,
                            Command: []string{
                                "-bind", "0.0.0.0:8080",
                                "-dial", "127.0.0.1:9090",
                                "-pong-chaos",
                                "-dialfreq", "250ms",
                                "-name", "ping",
                            },
                            Upstreams: []*topology.Upstream{{
                                ID:        topology.ServiceID{Name: "pong"},
                                LocalPort: 9090,
                                Peer:      "peer-dc2-default",
                            }},
                        },
                    },
                },
            },
            InitialConfigEntries: []api.ConfigEntry{
                &api.ExportedServicesConfigEntry{
                    Name: "default",
                    Services: []api.ExportedService{{
                        Name: "ping",
                        Consumers: []api.ServiceConsumer{{
                            Peer: "peer-dc2-default",
                        }},
                    }},
                },
            },
        },
        {
            Name: "dc2",
            Nodes: []*topology.Node{
                {
                    Kind: topology.NodeKindServer,
                    Name: "dc2-server1",
                    Addresses: []*topology.Address{
                        {Network: "dc2"},
                        {Network: "wan"},
                    },
                },
                {
                    Kind: topology.NodeKindClient,
                    Name: "dc2-client1",
                    Services: []*topology.Service{
                        {
                            ID:             topology.ServiceID{Name: "mesh-gateway"},
                            Port:           8443,
                            EnvoyAdminPort: 19000,
                            IsMeshGateway:  true,
                        },
                    },
                },
                {
                    Kind: topology.NodeKindDataplane,
                    Name: "dc2-client2",
                    Services: []*topology.Service{
                        {
                            ID:             topology.ServiceID{Name: "pong"},
                            Image:          "rboyer/pingpong:latest",
                            Port:           8080,
                            EnvoyAdminPort: 19000,
                            Command: []string{
                                "-bind", "0.0.0.0:8080",
                                "-dial", "127.0.0.1:9090",
                                "-pong-chaos",
                                "-dialfreq", "250ms",
                                "-name", "pong",
                            },
                            Upstreams: []*topology.Upstream{{
                                ID:        topology.ServiceID{Name: "ping"},
                                LocalPort: 9090,
                                Peer:      "peer-dc1-default",
                            }},
                        },
                    },
                },
            },
            InitialConfigEntries: []api.ConfigEntry{
                &api.ExportedServicesConfigEntry{
                    Name: "default",
                    Services: []api.ExportedService{{
                        Name: "ping",
                        Consumers: []api.ServiceConsumer{{
                            Peer: "peer-dc2-default",
                        }},
                    }},
                },
            },
        },
    },
    Peerings: []*topology.Peering{{
        Dialing: topology.PeerCluster{
            Name: "dc1",
        },
        Accepting: topology.PeerCluster{
            Name: "dc2",
        },
    }},
}
```

Once you have a topology configuration, you simply call the appropriate
`Launch` function to validate and boot the cluster.

You may also modify your original configuration (in some allowed ways) and call
`Relaunch` on an existing topology which will differentially adjust the running
infrastructure. This can be useful to do things like upgrade instances in place
or subly reconfigure them.

### For Testing

It is meant to be consumed primarily by unit tests desiring a complex
reasonably realistic Consul setup. For that use case use the `sprawl/sprawltest` wrapper:

```
func TestSomething(t *testing.T) {
    cfg := &topology.Config{...}
    sp := sprawltest.Launch(t, cfg)
    // do stuff with 'sp'
}
```

### For CLI

Though this is not an immediate design goal, this library should be safely usable from a CLI
to create easy development environments. For that case the `sprawl` package should be used directly:

```
cfg := &topology.Config{...}

sp, err := sprawl.Launch(logger, workdir, cfg)
if err != nil {
    return fmt.Errorf("error during launch: %w", err)
}

defer sp.Stop()
// do stuff with 'sp'

```

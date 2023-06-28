[![GoDoc](https://pkg.go.dev/badge/github.com/hashicorp/consul/testingconsul)](https://pkg.go.dev/github.com/hashicorp/consul/testingconsul)

## Summary

This is a Go library used to launch one or more Consul clusters that can be
peered using the cluster peering feature. Under the covers `terraform` is used
in conjunction with the
[`kreuzwerker/docker`](https://registry.terraform.io/providers/kreuzwerker/docker/latest)
provider to manage a fleet of local docker containers and networks.

### Configuration

The complete topology of Consul clusters is defined using a testingconsul.Config
which allows you to define a set of networks and reference those networks when
assigning nodes and services to clusters. Both Consul clients and
`consul-dataplane` instances are supported.

Here is an example configuration with two peered clusters:

```
cfg := &testingconsul.Config{
    Networks: []*testingconsul.Network{
        {Name: "dc1"},
        {Name: "dc2"},
        {Name: "wan", Type: "wan"},
    },
    Clusters: []*testingconsul.Cluster{
        {
            Name: "dc1",
            Nodes: []*testingconsul.Node{
                {
                    Kind: testingconsul.NodeKindServer,
                    Name: "dc1-server1",
                    Addresses: []*testingconsul.Address{
                        {Network: "dc1"},
                        {Network: "wan"},
                    },
                },
                {
                    Kind: testingconsul.NodeKindClient,
                    Name: "dc1-client1",
                    Services: []*testingconsul.Service{
                        {
                            ID:             testingconsul.ServiceID{Name: "mesh-gateway"},
                            Port:           8443,
                            EnvoyAdminPort: 19000,
                            IsMeshGateway:  true,
                        },
                    },
                },
                {
                    Kind: testingconsul.NodeKindClient,
                    Name: "dc1-client2",
                    Services: []*testingconsul.Service{
                        {
                            ID:             testingconsul.ServiceID{Name: "ping"},
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
                            Upstreams: []*testingconsul.Upstream{{
                                ID:        testingconsul.ServiceID{Name: "pong"},
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
            Nodes: []*testingconsul.Node{
                {
                    Kind: testingconsul.NodeKindServer,
                    Name: "dc2-server1",
                    Addresses: []*testingconsul.Address{
                        {Network: "dc2"},
                        {Network: "wan"},
                    },
                },
                {
                    Kind: testingconsul.NodeKindClient,
                    Name: "dc2-client1",
                    Services: []*testingconsul.Service{
                        {
                            ID:             testingconsul.ServiceID{Name: "mesh-gateway"},
                            Port:           8443,
                            EnvoyAdminPort: 19000,
                            IsMeshGateway:  true,
                        },
                    },
                },
                {
                    Kind: testingconsul.NodeKindDataplane,
                    Name: "dc2-client2",
                    Services: []*testingconsul.Service{
                        {
                            ID:             testingconsul.ServiceID{Name: "pong"},
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
                            Upstreams: []*testingconsul.Upstream{{
                                ID:        testingconsul.ServiceID{Name: "ping"},
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
    Peerings: []*testingconsul.Peering{{
        Dialing: testingconsul.PeerCluster{
            Name: "dc1",
        },
        Accepting: testingconsul.PeerCluster{
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
    cfg := &testingconsul.Config{...}
    sp := sprawltest.Launch(t, cfg)
    // do stuff with 'sp'
}
```


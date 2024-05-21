# test-integ

Go integration tests for consul. `/test/integration` also holds integration tests; they need migrating.

These should use the [testing/deployer framework](../testing/deployer) to bring
up some local testing infrastructure and fixtures to run test assertions against.

Where reasonably possible, try to bring up infrastructure interesting enough to
be able to run many related sorts of tests against it, rather than waiting for
many similar clusters to be provisioned and torn down. This will help ensure
that the integration tests do not consume CPU cycles needlessly.

## Prerequisites

Before you can run these tests, a fresh dev build should be created and tagged
for automatic use by tests using `utils.TargetImages()`. This can be done by
the toplevel consul make target:

    make test-deployer-setup

You can run the entire set of deployer integration tests using:

    make test-deployer         # everything except peering_commontopo
    make test-deployer-peering # just peering_commontopo

You can also run them one by one if you like:

    go test ./connect/ -run Test_Snapshot_Restore_Agentless -v

You can have the logs stream unbuffered directly to your terminal which can
help diagnose stuck tests that would otherwise need to fully timeout before the
logs would print by also setting the `NOLOGBUFFER=1` environment variable.

## Getting started

Deployer tests have three main parts:

1. Declarative topology description.
2. Launching the infrastructure defined by that description.
3. Making test assertions about the infrastructure.

Some tests may also have an optional _mutation_ phase followed by additional
assertions. These are only needed if the test needs to observe a reaction in
the system to a change in the environment or configuration.

### Topology description

Test authors craft a declarative description of the infrastructure necessary to exist for the test.
These are also referred to as a "topology".

These are comprised of 4 main parts:

- **Images**: The set of docker images and specific versions that will be used
              by default if not overridden on each Cluster or Node.

  - Consul CE
  - Consul Enterprise
  - Consul Dataplane
  - Envoy Proxy

- **Networks**: The non-overlapping networks that should exist for use by the Clusters.

- **Clusters**: The unique Consul clusters that should exist.

  - **Nodes**: A "box with ip address(es)". This should feel a bit like a VM or
               a Kubernetes Pod as an enclosing entity.

    - **Workloads**: The list of service instances that will execute on the given node.

  - **InitialConfigEntries**: Config entries that should be created as
                              part of the fixture and that make sense to
                              include as part of the test definition, rather
                              than something created during the test assertion
                              phase.

  - **InitialResources**: Resources that should be created as part of
                          the fixture and that make sense to include as part of
                          the test definition, rather than something created
                          during the test assertion phase.

- **Peerings**: The peering relationships between Clusters to establish.

In the [topoutil](./topoutil) package there are some helpers for defining
common sets of nodes or workloads like Consul Servers, Mesh Gateways, or [fortio
servers](https://github.com/fortio/fortio)

#### Useful topology concepts

Consul has a lot of independent configurables that can greatly increase the
testing configuration space required to flush out any bugs. The topology
definition was designed to be easily "exploded" to create testing microcosms on
a variety of axes:

- agentful (clients) vs agentless (dataplane)
- tenancies (partitions, namespaces)
- locally or across a peering

Since the topology is just a declarative struct, a test author could rewrite
any one of these attributes with a single field (such as `Node.Kind`) and cause
the identical test to run against the other configuration. With the addition of
a few `if enterprise {}` blocks and `for` loops, a test author could easily
write one test of a behavior and execute it to cover agentless, agentful, and
non-default tenancy in a few extra lines of code.

#### Non-optional security settings

The test framework always enables ACLs in default deny mode and provisions
minimal-permission tokens automatically to the various containers that need
them.

TLS certificates are similarly minted and distributed to all components that
need them.

### Launching a topology

There is a [sprawltest](../testing/deployer/sprawl/sprawltest) package that has
utilities to bring up a topology in the context of a Go test. This is basically a one-liner:

    sp := sprawltest.Launch(t, config)

After this line returns you will have a handle (`sp`) to the running cluster
and can use it to get ready-made api clients, http clients, gRPC resource
client, or test sockets open to a variety of the topology components for use in
authoring test code.

This helper will rig up a `t.Cleanup` handler that will destroy all resources
created during the test. This can be opted-out of by setting the
`SPRAWL_KEEP_RUNNING=1` environment variable before running the tests.

### Upgrade test
We are migrating upgrade tests from consul-container(`/test/integration`) to
this directory using the [testing/deployer framework](../testing/deployer).

The current implementation supports two upgrade strategies: [standard upgrade](https://developer.hashicorp.com/consul/docs/upgrading/instructions/general-process)
and [autopilot upgrade](https://developer.hashicorp.com/consul/tutorials/datacenter-operations/upgrade-automation). The basic test scenario can be found in `./test-integ/upgrade/basic`.


### Test assertions

Typical service mesh tests want to ensure that use of a service from another
service behaves in a certain way. Because the entire set of components is known
declaratively, we can process it into a flat list of known source/destination
relationships:

    ships := topology.ComputeRelationships()

This works hand-in-hand with the topology concepts mentioned above to
programmatically verify independent subunits of a topology that may exist (this
is helpful for things like testing multiple tenancy configurations without
duplicating all of the assertion code).

This can also be pretty printed to the log for diagnostic purposes with:

	t.Log(topology.RenderRelationships(ships))

Which looks like this:

    $ NOLOGBUFFER=1 go test ./catalogv2/ -run TestBasicL4ExplicitDestinations -v
    ...(skipping a bunch of output)...
    2023-11-08T11:48:04.395-0600 [INFO]  TestBasicL4ExplicitDestinations: topology is ready for use: elapsed=33.510298357s
        explicit_destinations_test.go:55: DOWN   |node               |service                         |port   |UP    |service                         |
            dc1    |default/dc1-box2   |default/default/single-client   |5000   |dc1   |default/default/single-server   |
            dc1    |default/dc1-box4   |default/default/multi-client    |5000   |dc1   |default/default/multi-server    |
            dc1    |default/dc1-box4   |default/default/multi-client    |5001   |dc1   |default/default/multi-server    |
                   |                   |                                |       |      |                                |

    === RUN   TestBasicL4ExplicitDestinations/relationship:_default/default/single-client_on_default/dc1-box2_in_dc1_via_:5000_=>_default/default/single-server_in_dc1_port_http
        service.go:224: making call to http://10.238.170.5:5000
        service.go:245: ...got response code 200
    === RUN   TestBasicL4ExplicitDestinations/relationship:_default/default/multi-client_on_default/dc1-box4_in_dc1_via_:5000_=>_default/default/multi-server_in_dc1_port_http
        service.go:224: making call to http://10.238.170.7:5000
        service.go:245: ...got response code 200
    === RUN   TestBasicL4ExplicitDestinations/relationship:_default/default/multi-client_on_default/dc1-box4_in_dc1_via_:5001_=>_default/default/multi-server_in_dc1_port_http-alt
        service.go:224: making call to http://10.238.170.7:5001
        service.go:245: ...got response code 200
    2023-11-08T11:48:04.420-0600 [INFO]  TestBasicL4ExplicitDestinations.tfgen: Running 'terraform destroy'...
    --- PASS: TestBasicL4ExplicitDestinations (40.60s)
        --- PASS: TestBasicL4ExplicitDestinations/relationship:_default/default/single-client_on_default/dc1-box2_in_dc1_via_:5000_=>_default/default/single-server_in_dc1_port_http (0.01s)
        --- PASS: TestBasicL4ExplicitDestinations/relationship:_default/default/multi-client_on_default/dc1-box4_in_dc1_via_:5000_=>_default/default/multi-server_in_dc1_port_http (0.01s)
        --- PASS: TestBasicL4ExplicitDestinations/relationship:_default/default/multi-client_on_default/dc1-box4_in_dc1_via_:5001_=>_default/default/multi-server_in_dc1_port_http-alt (0.01s)
    PASS
    ok  	github.com/hashicorp/consul/test-integ/catalogv2	40.612s

There is a ready-made helper to assist with making common inquiries to Consul
and Envoy that you can create in your test:

    asserter := topoutil.NewAsserter(sp)

    asserter.UpstreamEndpointStatus(t, svc, clusterPrefix+".", "HEALTHY", 1)

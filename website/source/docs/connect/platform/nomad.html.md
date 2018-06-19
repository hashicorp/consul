---
layout: "docs"
page_title: "Connect - Nomad"
sidebar_current: "docs-connect-platform-nomad"
description: |-
  Connect can be used with [Nomad](https://www.nomadproject.io) to provide secure service-to-service communication between Nomad jobs. The ability to use the dynamic port feature of Nomad makes Connect particularly easy to use.
---

# Connect on Nomad

Connect can be used with [Nomad](https://www.nomadproject.io) to provide
secure service-to-service communication between Nomad jobs and task groups. The ability to
use the [dynamic port](https://www.nomadproject.io/docs/job-specification/network.html#dynamic-ports)
feature of Nomad makes Connect particularly easy to use.

Using Connect with Nomad today requires manually specifying the Connect
sidecar proxy and managing intentions directly via Consul (outside of Nomad).
The Consul and Nomad teams are working together towards a more automatic
and unified solution in an upcoming Nomad release.

~> **Important security note:** As of Nomad 0.8.4, Nomad doesn't yet support network namespacing
for tasks in a task group. As a result, running Connect with Nomad should
assume the same [security checklist](/docs/connect/security.html#prevent-non-connect-traffic-to-services) as running directly on a machine without namespacing.

## Requirements

To use Connect with Nomad, the following requirements must be first be
satisfied:


  * **Nomad 0.8.3 or later.** - The server and clients of the Nomad cluster
    must be running version 0.8.3 or later. This is the earliest version that
	was verified to work with Connect. It is possible to work with earlier
	versions but it is untested.

  * **Consul 1.2.0 or later.** - A Consul cluster must be setup and running with
    version 1.2.0 or later.
    Nomad must be [configured to use this Consul cluster](https://www.nomadproject.io/docs/service-discovery/index.html).

## Accepting Connect for an Existing Service

The job specification below shows a job that is configured with Connect.
The example uses `raw_exec` for now just to show how it can be used locally
but the Docker driver or any other driver could easily be used. The example
will be updated to use the official Consul Docker image following release.

The example below shows a hypothetical database being configured to listen
with Connect only. Explanations of the various sections follow the example.

```hcl
job "db" {
    datacenters = ["dc1"]

    group "db" {
        task "db" {
            driver = "raw_exec"

            config {
                command = "/usr/local/bin/my-database"
                args    = ["-bind", "127.0.0.1:${NOMAD_PORT_tcp}"]
            }

            resources {
                network {
                    port "tcp" {}
                }
            }
        }

        task "connect-proxy" {
            driver = "raw_exec"

            config {
                command = "/usr/local/bin/consul"
                args    = [
                    "connect", "proxy",
                    "-service", "db",
                    "-service-addr", "${NOMAD_ADDR_db_tcp}",
                    "-listen", ":${NOMAD_PORT_tcp}",
                    "-register",
                ]
            }

            resources {
                network {
                    port "tcp" {}
                }
            }
        }
    }
}
```

The job specification contains a single task group "db" with two tasks.
By placing the two tasks in the same group, the Connect proxy will always
be colocated directly next to the database, and has access to information
such as the dynamic port it is running on.

For the "db" task, there are a few important configurations:

  * The `-bind` address for the database is loopback only and listening on
    a dynamic port. This prevents non-Connect connections from outside of
    the node that the database is running on.

  * The `tcp` port is dynamic. This removes any static constraints on the port,
    allowing Nomad to allocate any available port for any allocation.

  * The database is _not_ registered with Consul using a `service` block.
    This isn't strictly necessary, but since we won't be connecting directly
    to this service, we also don't need to register it. We recommend registering
    the source service as well since Consul can then know the health of the
    target service, which is used in determining if the proxy should
	receive requests.

Next, the "connect-proxy" task is colocated next to the "db" task. This is
using "raw_exec" executing Consul directly. In the future this example will
be updated to use the official Consul Docker image.

The important configuration for this proxy:

  * The `-service` and `-service-addr` flag specify the name of the service
    the proxy is representing. The address is set to the interpolation
    `${NOMAD_ADDR_db_tcp}` which allows the database to listen on any
    dynamic address and the proxy can still find it.

  * The `-listen` flag sets up a public listener (TLS) to accept connections
    on behalf of the "db" service. The port this is listening on is dynamic,
    since service discovery can be used to find the service ports.

  * The `-register` flag tells the proxy to self-register with Consul. Nomad
    doesn't currently know how to register Connect proxies with the `service`
    stanza, and this causes the proxy to register itself so it is discoverable.

Following running this job specification, the DB will be started with a
Connect proxy. The only public listener from the job is the proxy. This means
that only Connect connections can access the database from an external node.

## Connecting to Upstream Dependencies

In addition to accepting Connect-based connections, services often need
to connect to upstream dependencies that are listening via Connect. For
example, a "web" application may need to connect to the "db" exposed
in the example above.

The job specification below shows an example of this scenario:

```hcl
job "web" {
    datacenters = ["dc1"]

    group "web" {
        task "web" {
            # ... typical configuration.

            env {
                DATABASE_URL = "postgresql://${NOMAD_ADDR_proxy_tcp}/db"
            }
        }

        task "proxy" {
            driver = "raw_exec"

            config {
                command = "/usr/local/bin/consul"
                args    = [
                    "connect", "proxy",
                    "-service", "web",
                    "-upstream", "db:${NOMAD_PORT_tcp}",
                ]
            }

            resources {
                network {
                    port "tcp" {}
                }
            }
        }
    }
}
```

Starting with the "proxy" task, the primary difference to accepting
connections is that the service address, `-listen`, and `-register` flag
are not specified. This prevents the proxy from registering itself as
a valid listener for the given service.

The `-upstream` flag is specified to configure a private listener to
connect to the service "db" as "web". The port is dynamic. The listener
will bind to a loopback address only.

Finally, the "web" task is configured to use the localhost address to
connect to the database. This will establish a connection to the remote
DB using Connect. Interpolation is used to retrieve the address dynamically
since the port is dynamic.

-> **Both -listen and -upstream can be specified** for services that both
accept Connect connections as well as have upstream dependencies. Additionally,
multiple `-upstream` flags can be specified for multiple upstream dependencies. This
can be done on a single proxy instance rather than having multiple.

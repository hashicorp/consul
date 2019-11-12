---
layout: "docs"
page_title: "[Enterprise] Connect Services Within a Namespace"
sidebar_current: "connect-namespaces"
description: |-
  In this guide you will register connect enabled services in two different
  namespaces, create an intention within a namespace, and test isolation between
  namespaces.
---

!> **Warning:** This guide is a draft and has not been fully tested.

!> **Warning:** Consul 1.7 is currently a beta release.

Namespaces make it easier for multiple teams to share the same Consul
datacenters by allowing operators to delegate ACL management, and allowing teams
to give services the same names without causing conflicts. These benefits extend
to Consul Connect.

With namespaces, operators can delegate intentions management to users in a
given namespace, along with permission to create configuration entries and
register services. In this guide, you will register Connect-enabled services and
create intentions within a namespace.

## Prerequisites

If you are following this track sequentially you will need to join one
additional Consul node to your existing infrastructure, and install the binaries
for the [counting and dashboard  
services](https://github.com/hashicorp/demo-consul-101/releases) to follow this
guide.

If you haven’t completed the previous guides in the track, you will need the
following.

A two-node, Consul Enterprise, version >1.7 datacenter with:

- Connect enabled
- ACLs enabled
- The web UI enabled
- An ACL token with `operator=write` and `acl=write`. We will call this the
  super-operator token.
- Two namespaces created: `app-team` and `db-team`
- Two operator tokens, one for each namespace, with  `acl=write`permission
  within its namespace.
- The binaries for the [counting and dashboard
  services](https://github.com/hashicorp/demo-consul-101/releases) installed.

In this guide you will use multiple tokens to demonstrate how namespaces enable
permissions delegation. We assume that you will open multiple shell sessions and
set the tokens as environment variables in each session (which is recommended
practice for production), but in most cases you could also pass them as command
line flags using `-token`.

## Create developer tokens

We recommend that you restrict Consul access to the minimum that users need. In
this example you will give developers the ability to register their own
services, and allow or deny communication between services in their team’s
namespace.

~> **Note:** Depending on your company’s security model you may want to delegate
intentions management to a different set of users than service registration.

### Create a policy

First make a file called `developer-policy.hcl` and paste in the following,
which allows writing services and intentions for those services.

```hcl
service "" {
  policy = "write"
}
service_prefix "" {
  intention = "write"
}
```

Next create the policy using this file.

```shell
$ consul acl policy create \
                           -name "developer-policy" \
                           -description "Write services and intentions" \
                           -rules @developer-policy.hcl
```

### Create an app developer token

Open a new terminal and set the app team operator’s token as an environment
variable.

```shell
$ CONSUL_HTTP_TOKEN=<App operator token here>
```

Using the developer policy defined in the previous step, create a token for the
developer in the app-team namespace.

```shell
$ consul acl token create \     
                          -description "App developer token" \
                          -policy-name developer-policy \
```

### Create a DB developer token

Open a new terminal and set the DB team operator’s token as an environment
variable.

```shell
$ CONSUL_HTTP_TOKEN=<DB operator token here>
```

Using the database operator’s token repeat the process to create a database
developer token in the db-team namespace.

```shell
$ consul acl token create \     
                          -description "DB developer token" \
                          -policy-name developer-policy \
```

Now that you have a developer token for each namespace you're ready to register
services as if you were a developer, first in the app namespace and then in the
DB namespace.

## Node 1: Register the app counting service

Open a new terminal window set the app developer token as an environment
variable.

```shell
$ CONSUL_HTTP_TOKEN=<App developer token here>
```

Now you’re ready to register the counting service along with its sidecar proxy
on node one.

Create a registration file called `counting.hcl`. Paste the following
configuration into the file. It names the application `counting`, tells Consul
what port to find the application on, and specifies that the application should
have a sidecar proxy associated with it that Consul will configure using its
reasonable defaults.

```hcl
service {
  name = "counting"
  port = 9003
  connect {
    sidecar_service {}
  }
}
```

Register the counting service using the Consul CLI, by providing the location of
the registration file and the node to register the service on (substitute node
one's IP address).

```shell
$ consul services register counting.hcl -http-addr=<Node 1 address>:8500
Registered service: counting
```

Log onto node one and start the counting service in the background, and using an
environment variable configure it to run on port `9003`.

```shell
$ PORT=9003 ./counting-service &
```

## Node 2: Register the app dashboard service

Open a new terminal window and set the same app developer token as an
environment variable.

```sh
$ CONSUL_HTTP_TOKEN=<App developer token here>
```

Now you can register the dashboard service on the second node, which will get
data from the counting service. Create a file called `dashboard.hcl` and paste
the following configuration into it.

```hcl
service {
  name = "dashboard"
  port = 9002
  connect {
    sidecar_service {
      proxy {
        upstreams = [
          {
            destination_name = "counting"
            local_bind_port  = 5000
         }
        ]
      }
    }
 }
```

The configuration names the service `dashboard`, tells Consul which port to find
the service on, and specifies that the service should have an associated sidecar
proxy. The proxy’s configuration relies mostly on Consul’s reasonable defaults,
but specifies an upstream service called ‘counting’ and tells the proxy where
the dashboard service will look for data from counting, in this case on port
`5000`.

Now register the dashboard service using the Consul CLI, by providing the
location of the registration file and the node to register the service on
(substitute node two's IP address).

```shell
$ consul services register dashboard.hcl -http-addr=<Node 2 address>:8500
Registered service: dashboard
```

Log onto node two and start the dashboard service, using environment variables
to expose it on port `9002`, and configure it to look for input from the
counting service on port `5000` (the port you defined in the sidecar’s upstream
configuration).

```shell
$ PORT=9002 COUNTING_SERVICE_URL="http://localhost:5000" ./dashboard-service
<example output here>
```

Let the dashboard service run in the foreground. The dashboard service should
output counts of `-1` because with ACLs enabled with a deny-all policy,
intentions also deny all communications that are not specifically allowed.

## Create an intention

In the terminal window on the first node (where you started the counting
service) enable communication between the dashboard and counting services by
creating an allow intention from the dashboard to the counting service. This
window is using the app developer token.

```shell
$ consul intention create dashboard counting
```

Your dashboard service should now print increasing numbers, indicating that it
is connected to the counting service. You can also see this service’s output in
a browser window by navigating to the IP address of your second node, and
visiting port `9002`. For example `http://localhost:9002`.

## Node 2: Register the DB counting service

Now imagine that another team, the database team, wants to register a different
service, also called called “counting”, in the same Consul datacenter.
Namespaces allow the app and db teams to register services with the same name.

Open a new terminal window set the database developer token as an environment
variable.

```sh
$ CONSUL_HTTP_TOKEN=<DB developer token here>
```

Register the counting service using the Consul CLI, by providing the location of
the same registration file you used for the app team’s counting service. The
only differences between the app and DB counting services is their namespace,
and the fact that they’re registered on different nodes.

```shell
$ consul services register counting.hcl -http-addr=<node2 address>:8500
Registered service: counting
```

Log into node two and start the counting service in the background, and using an
environment variable configure it to run on port `9003`.

```shell
$ PORT=9003 ./counting-service &
```

### Check isolation

In a browser window, navigate to the dashboard service page at node two's IP
address on port `9002`, for example `http://localhost:9002`. Below the large
number that is counting up, the dashboard displays a small bit of white text
indicating the location of the counting service it is connected to.

When you refresh the dashboard service in your browser this location should
always indicate that the counting service is on node one, despite the fact that
there is also a counting service registered with Consul, and running locally
with the dashboard on node two. The dashboard never connects to this service,
because it is in a different namespace.

As the app developer, you can manage intentions that specify destination within
your namespace. You can not create an intention between the app dashboard
service and DB counting service, because the DB counting service is outside of
your namespace.

If a super operator were to create a cross namespace intention between your
dashboard service and the DB counting service, you would be able to see this
intention, but not edit it. However, because users can manage any intentions
that target services in their namespaces, the DB developer would be able to edit
that intention.

## Extend your knowledge

In this guide you created developer tokens in two different namespaces and saw
how intentions created within a namespace only apply to services within that
namespace.

In production an operator may want to configure connect proxies to collect
metrics or tracing data about your network traffic. Because this data gives a
standardized picture of vital metrics across all services, the proxy defaults
configuration entry type is not namespaced. Proxy defaults configuration entries
will apply to the proxies in all the namespaces of the datacenter.

You can find out more about namespace configuration by reading the [reference
documentation](LINK), or move on to the next guide to explore how namespaces
work across datacenters.

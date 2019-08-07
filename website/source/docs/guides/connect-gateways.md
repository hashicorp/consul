---
layout: "docs"
page_title: "Connecting Services Across Datacenters"
sidebar_current: "docs-guides-connect-gateways"
description: |-
  Connect services and secure inter-service communication across datacenters
  using Consul Connect and mesh gateways.
---
## Introduction

Consul Connect is Consul’s service mesh offering, which allows users to observe
and secure service-to-service communication. Because Connect implements mutual
TLS between services, it also enabled us to build mesh gateways, which provide
users with a way to help services in different datacenters communicate with each
other. Mesh gateways take advantage of Server Name Indication (SNI), which is an
extension to TLS that allows them to see the destination of inter-datacenter
traffic without decrypting the message payload.

Using mesh gateways for inter-datacenter communication can prevent each Connect
proxy from needing an accessible IP address, and frees operators from worrying
about IP address overlap between datacenters.

In this guide, you will configure Consul Connect across multiple Consul
datacenters and use mesh gateways to enable inter-service traffic between them.

Specifically, you will:

1. Enable Connect in both datacenters
1. Deploy the two mesh gateways
1. Register services and Connect sidecar proxies
1. Configure intentions
1. Test that your services can communicate with each other

For the remainder of this guide we will refer to mesh gateways as "gateways".
Anywhere in this guide where you see the word gateway, assume it is specifically
a mesh gateway (as opposed to an API or other type of gateway).

## Prerequisites

To complete this guide you will need two wide area network (WAN) joined Consul
datacenters with access control list (ACL) replication enabled. If you are
starting from scratch, follow these guides to set up your datacenters, or use
them to check that you have the proper configuration:

- [Deployment Guide](/consul/datacenter-deploy/deployment-guide)
- [Securing Consul with ACLs](/consul/security-networking/production-acls)
- [Basic Federation with WAN Gossip](/consul/security-networking/datacenters)

You will also need to enable ACL replication, which you can do by following the
[ACL Replication for Multiple
Datacenters](/consul/day-2-operations/acl-replication) guide with the following
modification.

When creating the [replication token for ACL
management](/consul/day-2-operations/acl-replication#create-the-replication-token-for-acl-management),
it will need the following policy:

```json
{
  "acl": "write",
  "operator": "write",
  "service_prefix": {
    "" : {
      "policy": "read"
    }
  }
}
```

The replication token needs different permissions depending on what you want to
accomplish. The above policy allows for ACL policy, role, and token replication
with `acl:write`, CA replication with `operator:write` and intention and
configuration entry replication with `service:*:read`.

You will also need to install [Envoy](https://www.envoyproxy.io/) alongside your
Consul clients. Both the gateway and sidecar proxies will need to get
configuration and updates from a local Consul client.

Lastly you should set [`enable_central_service_config =
true`](https://www.consul.io/docs/agent/options.html#enable_central_service_config)
on your Consul clients, which will allow them to centrally configrure the
sidecar and mesh gateway proxies.

## Enable Connect in Both Datacenters

Once you have your datacenters set up and ACL replication configured, it’s time
to enable Connect in each of them sequentially. Connect’s certificate authority
(which is distinct from the Consul certificate authority that you manage using
the CLI) will automatically bootstrap as soon as a server with Connect enabled
becomes the server cluster’s leader. You can also use [Vault as a Connect
CA](https://www.consul.io/docs/connect/ca/vault.html).

!> **Warning:** If you are using this guide as a production playbook, we
strongly recommend that you enable Connect in each of your datacenters by
following the [Connect in Production
guide](/consul/developer-segmentation/connect-production),
which includes production security recommendations.

### Enable Connect in the primary datacenter

Enable Connect in the primary data center and bootstrap the Connect CA by adding
the following snippet to the server configuration for each of your servers.

```json
connect {
  "enabled": true
}
```

Load the new configuration by restarting each server one at a time, making sure
to maintain quorum. This will be a similar process to performing a [rolling
restart during
upgrades](https://www.consul.io/docs/upgrading.html#standard-upgrades).

Stop the first server by running the following [leave
command](https://www.consul.io/docs/commands/leave.html).

```text
$ consul leave
```

Once the server shuts down restart it and make sure that it is healthy and
rejoins the other servers. Repeat this process until you've restarted all the
servers with Connect enabled.

### Enable Connect in the secondary datacenter

Once Connect is enabled in the primary datacenter, follow the same process to
enable Connect in the secondary datacenter. Add the following configuration to
the configuration for your servers, and restart them one at a time, making sure
to maintain quorum.

```json
connect {
  "enabled": true
}
```

The `primary_datacenter` setting that was required in order to enable ACL
replication between datacenters also specifies which datacenter will write
intentions and act as the [root CA for Connect](https://www.consul.io/docs/connect/connect-internals.html#connections-across-datacenters).
Intentions, which allow or deny inter-service communication, are automatically
replicated to the secondary datacenter.

## Deploy Gateways

Connect mesh gateways proxy requests from services in one datacenter to services
in another, so you will need to deploy your gateways on nodes that can reach
each other over the network. As we mentioned in the prerequisites,
you will need to make sure that both Envoy and Consul are installed on the
gateway nodes. You won’t want to run any services on these nodes other than
Consul and Envoy because they necessarily will have access to the WAN.

### Generate Tokens for the Gateways

You’ll need to [generate a
token](/consul/security-networking/production-acls#apply-individual-tokens-to-the-services)
for each gateway that gives it read access to the entire catalog.

Create a file named `mesh-gateway-policy.json` containing the following content.

```json
{
  "node_prefix": {
    "": {
      "policy": "read"
    }
  }
}  
{
  "service_prefix": {
    "": {
      "policy": "read"
    }
  }
}  
{
  "service": {
      "mesh-gateway": {
        "policy": "write"
      }
    }
}
```

Next, create and name a new ACL policy using the file you just made.

```text
$ consul acl policy create \
  -name mesh-gateway \
  -rules @mesh-gateway-policy.json
```

Generate a token for each gateway from the new policy.

```text
$ consul acl token create -description "mesh-gateway primary datacenter token" \
  -policy-name mesh-gateway
```

```text
$ consul acl token create \
  -description "mesh-gateway secondary datacenter token" \
  -policy-name mesh-gateway
```

You’ll apply those tokens when you deploy the gateways.

### Deploy the Gateway for your primary datacenter

Register and start the gateway in your primary datacenter with the following
command.

```text
$ consul connect envoy -mesh-gateway -register \
                     -service-name "gateway-primary"
                     -address "<your private address>" \
                     -wan-address "<your externally accessible address>"\
                     -token=<token for the primary dc gateway>
```

### Deploy the Gateway for your Secondary Datacenter

Register and start the gateway in your secondary datacenter with the following
command.

```text
$ consul connect envoy -mesh-gateway -register \
                     -service-name "gateway-secondary"
                     -address "<your private address>" \
                     -wan-address "<your externally accessible address>"\
                     -token=<token for the secondary dc gateway>
```

### Configure Sidecar Proxies to use Gateways

Next, create a [centralized
configuration](https://www.consul.io/docs/agent/config_entries/proxy-defaults.html)
file for all the sidecar proxies in both datacenters called
`proxy-defaults.json`. This file will instruct the sidecar proxies to send all
their inter-datacenter traffic through the gateways. It should contain the
following:

```json
{
  "Kind": "proxy-defaults",
  "Name":  "global",
  "MeshGateway": "local"
}
```

Write the centralized configuration you just created with the following command.

```text
$ consul config write proxy-defaults.json
```

Once this step is complete, you will have set up Consul Connect with gateways
across multiple datacenters. Now you are ready to register the services that
will use Connect.

## Register a Service in Each Datacenter to Use Connect

You can register a service to use a sidecar proxy by including a sidecar proxy
stanza in its registration file. For this guide, you can use socat to act as a
backend service and register a dummy service called web to represent the client
service. Those names are used in our examples. If you have services that you
would like to connect, feel free to use those instead.

~> **Caution:** Connect takes its default intention policy from Consul’s default
ACL policy. If you have set your default ACL policy to deny (as is recommended
for secure operation) and are adding Connect to already registered services,
those services may lose connection to each other until you set an intention
between them to allow communication.

### Register a back end service in one datacenter

In one datacenter register a backend service and add an Envoy sidecar proxy
registration. To do this you will either create a new registration file or edit
an existing one to include a sidecar proxy stanza. If you are using socat as
your backend service, you will create a new file called `socat.json` that will
contain the below snippet. Since you have ACLs enabled, you will have to [create
a token for the
service](/consul/security-networking/production-acls#apply-individual-tokens-to-the-services).

```json
{
  "service": {
    "name": "socat",
    "port": 8181,
    "token": "<token here>",
    "connect": {"sidecar_service": {} }
  }
}
```

Note the Connect stanza of the registration with the `sidecar_service` and
`token` options. This is what you would add to an existing service registration
if you are not using socat as an example.

Reload the client with the new or modified registration.

```text
$ consul reload
```

Then start Envoy specifying which service it will proxy.

```text
$ consul connect envoy -sidecar-for socat
```

If you are using socat as your example, start it now on the port you specified
in your registration by running the following command.

```text
$ socat -v tcp-l:8181,fork exec:"/bin/cat"
```

Check that the socat service is running by accessing it using netcat on the same
node. It will echo back anything you type.

```text
$ nc 127.0.0.1 8181
hello
hello
echo
echo
```

Stop the running netcat service by typing `ctrl + c`.

### Register a front end service in the other datacenter

Now in your other datacenter, you will register a service (with a sidecar proxy)
that calls your backend service. Your registration will need to list the backend
service as your upstream. Like the backend service, you can use an example
service, which will be called web, or append the connect stanza to an existing
registration with some customization.

To use web as your front end service, create a registration file called
`web.json` that contains the following snippet.

```json
{
  "service": {
    "name": "web",
    "port": 8080,
    "token": "<token here>",
    "connect": {
      "sidecar_service": {
        "proxy": {
          "upstreams": [{
            "destination_name": "socat",
            "datacenter": "primary",
            "local_bind_port": 8181
          }]
        }
      }
    }  
  }
}
```

Note the Connect part of the registration, which specifies socat as an
upstream. If you are using another service as a back end, replace `socat` with
its name and the `8181` with its port.

Reload the client with the new or modified registration.

```text
$ consul reload
```

Then start Envoy and specify which service it will proxy.

```text
$ consul connect envoy -sidecar-for web
```

## Configure Intentions to Allow Communication Between Services

Now that your services both use Connect, you will need to configure intentions
in order for them to communicate with each other. Add an intention to allow the
front end service to access the back end service. For web and socat the command
would look like this.

```text
$ consul intention create web socat
```

Consul will automatically forward intentions initiated in the in the secondary
datacenter to the primary datacenter, where the servers will write them. The
servers in the primary datacenter will then automatically replicate the written
intentions back to the secondary datacenter.

## Test the connection

Now that you have services using Connect, verify that they can contact each
other. If you have been using the example web and socat services, from the node
and datacenter where you registered the web service, start netcat and type
something for it to echo.

```text
$ nc 127.0.0.1 8181
hello
hello
echo
echo
```

## Summary

In this guide you configured two WAN-joined datacenters to use Consul Connect,
deployed gateways in each datacenter, and connected two services to each other
across datacenters.

Gateways know where to route traffic because of Server Name Indication (SNI)
where the client service sends the destination as part of the TLS handshake.
Because gateways rely on TLS to discover the traffic’s destination, they require
Consul Connect to route traffic.


### Next Steps

Now that you’ve seen how to deploy gateways to proxy inter-datacenter traffic,
you can deploy multiple gateways for redundancy or availability. The gateways
and proxies will automatically round-robin load balance traffic between the
gateways.

If you are using Kubernetes you can configure Connect and deploy gateways for
your Kubernetes cluster using the Helm chart. Learn more in the [Consul’s
Kubernetes documentation](https://www.consul.io/docs/platform/k8s/helm.html)

Visit the Consul documentation for a full list of configurations for [Consul
Connect](https://www.consul.io/docs/connect/index.html), including [mesh gateway
configuration options](https://www.consul.io/docs/connect/mesh_gateway.html).

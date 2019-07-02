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
TLS between services, it also provides users with a way to help services in
different datacenters communicate with each other through mesh gateways. Using
gateways for inter-datacenter communication can prevent each Connect proxy from
needing an accessible address, and frees operators from worrying about IP
overlap between datacenters.

In this guide, you will configure Consul Connect in multiple, joined Consul
datacenters and use gateways to facilitate inter-service traffic between them.
Specifically, you will:

1. Enable Connect in both datacenters
1. Deploy the two gateways
1. Register services and Connect sidecar proxies
1. Configure intentions
1. Test that your services can communicate with each other

## Prerequisites

To complete this guide you will need two wide area network (WAN) joined Consul
datacenters with access control list (ACL) replication enabled. If you are
starting from scratch, follow these guides to set up your datacenters, or use
them to check that you have the proper configuration:

- [Deployment Guide](https://learn.hashicorp.com/consul/datacenter-deploy/deployment-guide)
- [Securing Consul with ACLs](https://learn.hashicorp.com/consul/security-networking/production-acls)
- [Basic Federation with WAN Gossip](https://learn.hashicorp.com/consul/security-networking/datacenters)
- [ACL Replication for Multiple Datacenters](https://learn.hashicorp.com/consul/day-2-operations/acl-replication)

You will also need to install [Envoy](https://www.envoyproxy.io/) alongside your
Consul clients. Both the gateway and sidecar proxies will need to get
configuration and updates from a local Consul client.

## Enable Connect in Both Datacenters

Once you have your datacenters set up and ACL replication configured, it’s time
to enable Connect in each of them sequentially. Connect’s certificate authority
(which is distinct from the Consul certificate authority that you manage using
the CLI) will automatically bootstrap as soon as a server with Connect enabled
becomes the server cluster’s leader. You can also use [Vault as a Connect
CA](https://www.consul.io/docs/connect/ca/vault.html).

!> If you are using this guide as a production playbook, we strongly recommend
that you enable Connect in each of your datacenters by following the [Connect in
Production
guide](https://learn.hashicorp.com/consul/developer-segmentation/connect-production),
which includes production security recommendations.

### Enable Connect in the primary datacenter

Enable Connect in the primary data center and bootstrap the Connect CA by adding
the following snippet to the server configuration for each of your servers.

```
connect {
  enabled = true
}
```

Load the new configuration by restarting each server one at a time, making sure
to maintain quorum. Stop the first server by running the following [leave
command](https://www.consul.io/docs/commands/leave.html).

```
$ consul leave
```

Once the server shuts down, restart it with the Consul agent command.

```
$ consul agent -server
```

Once the server is healthy and has rejoined the other servers you can restart
the next one.

### Enable Connect in the secondary datacenter

Once Connect is enabled in the primary datacenter, follow the same process to
enable Connect in the secondary datacenter. Add the following configuration to
the configuration for your servers, and restart them one at a time, making sure
to maintain quorum.

```
connect {
  enabled = true
}
```

The `primary_datacenter` setting that was required in order to enable ACL
replication between datacenters also specifies which datacenter will act as the
[root CA for Connect](LINK TO MULTI-DC CONNECT DOCS) and write intentions.
Intentions are automatically replicated to the secondary datacenter.

## Deploy Gateways

Connect mesh gateways proxy requests from services in one datacenter to services
in another, so you will need to deploy your gateways on nodes that can reach
each other over the network. You’ll also need to [generate a
token](https://learn.hashicorp.com/consul/security-networking/production-acls#apply-individual-tokens-to-the-services)
for each gateway that gives it read access to the entire catalog. You’ll apply
those tokens when you start the gateways. As we mentioned in the prerequisites,
you will need to make sure that both Envoy and Consul are installed on the
gateway nodes. You won’t want to run any services on these nodes other than
Consul and Envoy because they need to be more accessible than the other nodes in
your cluster, and will have access to the WAN.

### Deploy the Gateway for your primary datacenter

Register and start the gateway in your primary datacenter with the following
command.

```
consul connect envoy -mesh-gateway -register \
                     -address "<your private address>" \
                     -wan-address "<your externally accessible address>"\
                     -token=<token for the primary dc gateway>
```

Next, create a [centralized configuration](LINK) file for all the sidecar
proxies in your primary datacenter called `proxy-defaults.hcl`. This file will
instruct the sidecar proxies to send all their inter-datacenter through the
gateway. It should contain the following:

```
Kind = "proxy-defaults"
Name =  "global"
MeshGateway = "local"
```

Write the centralized configuration you just created with the following command.

```
consul config write proxy-defaults.hcl
```

### Repeat the Process to Deploy the Gateway for your Secondary Datacenter

Repeat the above process for the secondary datacenter. Register and start the
gateway in your secondary datacenter with the following command.

```
consul connect envoy -mesh-gateway -register \
                     -address "<your private address>" \
                     -wan-address "<your externally accessible address>"\
                     -token=<token for the secondary dc gateway>
```

Next, create a centralized configuration file for all the sidecar proxies in
your secondary datacenter called `proxy-defaults.hcl`. This file will instruct
the sidecar proxies to send all their inter-datacenter through the gateway. It
should contain the following:

```
Kind = "proxy-defaults"
Name =  "global"
MeshGateway = "local"
```

Write the centralized configuration you just created with the following command.

```
consul config write proxy-defaults.hcl
```

Once this step is complete, you will have set up Consul Connect with gateways
across multiple datacenters. Now you are ready to register the services that
will use Connect.

## Register a Service in Each Datacenter to Use Connect

You can register a service to use a sidecar proxy by including a sidecar proxy
stanza in its registration file. For this guide, you can use netcat to act as a
backend service and register a dummy service called web to represent the client
service. Those names are used in our examples. If you have services that you
would like to connect, feel free to use those instead.

~> **Caution:** Connect takes its default intention policy from Consul’s default
ACL policy. If you have set your default ACL policy to deny (as is recommended
for secure operation)  and are adding Connect to already registered services,
those services will lose connection to each other until you set an intention
between them to allow communication.

### Register a back end service in one datacenter

In one datacenter register a backend service and add an Envoy sidecar proxy
registration. To do this you will either create a new registration file or edit
an existing one to include a sidecar proxy stanza. If you are using netcat as
your backend service, you will create a new file called `netcat.json` that would
contain the below snippet. Since you have ACLs enabled, you will have to create
a token for the service.

```
{
  "service": {
    "name": "netcat",
    "port": 8181,
    "token": <token here>,
    "connect": {"sidecar_service": {} }

}
```

Note the Connect part of the registration with the sidecar service and token
options. This is what you would add to an existing service registration if you
are not using netcat as an example.

Reload the client with the new or modified registration.

```
consul reload
```

Then start Envoy specifying which service it will proxy.

```
consul connect envoy -sidecar-for netcat
```

If you are using netcat as your example, start it now by running the following
command. netcat will run in your terminal window and echo back anything you type
on the command line.

```
$ nc 127.0.0.1 8181
hello
hello
echo
echo
```

Stop the running service by typing `ctrl + c`.

### Register a front end service in the other datacenter

Now in your other datacenter, you will register a service that calls your
backend service, with a sidecar proxy Your registration will need to list the
backend service as your upstream. Like the backend service, you can use our
example service, which will be called web, or append the connect stanza to an
existing registration with some customization.

To use web as your front end service, create a registration file called
`web.json` that contains the following snippet.

```
{
  "service": {
    "name": "web",
    "port": 8080,
    "token": <token here>,
    "connect": {
      "sidecar_service": {
        "proxy": {
          "upstreams": [{
             "destination_name": "netcat",
             "local_bind_port": 8181
          }]




}
```

Note the Connect part of the registration, which specifies netcat as an
upstream. If you are using another service as a back end, replace `netcat` with
its name and the `8181` with its port.

Reload the client with the new or modified registration.

```
consul reload
```

Then start Envoy and specify which service it will proxy.

```
consul connect envoy -sidecar-for web
```

## Configure Intentions to Allow Communication Between Services

Now that your services both use Connect, you will need to configure intentions
in order for them to communicate with each other. Add an intention to allow the
front end service to access the back end service. For web and netcat the command
would look like this.

```
consul intention create web netcat
```

Consul will automatically forward intentions initiated in the in the secondary
datacenter to the primary datacenter, where the servers will write them. The
servers in the primary datacenter will then automatically replicate the written
intentions back to the secondary datacenter.

## Test the connection

Now that you have services using Connect, verify that they can contact each
other. If you have been using the example web and netcat services, from the node
and datacenter where you registered the web service, start the netcat service
and type something for it to echo.

```
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
Kubernetes documentation](LINK TO DOCS)

Visit the Consul documentation for a full list of configurations for [Consul
Connect](https://www.consul.io/docs/connect/index.html), including [Gateway
configuration options](LINK).

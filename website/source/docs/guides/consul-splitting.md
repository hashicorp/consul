---
name: Traffic Splitting for Service Deployments
content_length: 15
id: connect-splitting
products_used:
  - Consul
description: |-
  In this guide you will split layer-7 traffic, using Envoy proxies configured by
  Consul, to roll out a new version of a service. You can use this method for
  zero-downtime, blue-green, and canary deployments.
level: Implementation
---

-> **Note:** This guide requires Consul 1.6.0 or newer.

When you deploy a new version of a service, you need a way to start using the
new version without causing downtime for your end users. You can't just take the
old version down and deploy the new one, because for a brief period you would
cause downtime. This method runs the additional risk of being hard to roll back
if there are unexpected problems with the new version of the service.

You can solve this problem by deploying the new service, making sure it works in
your production environment with a small amount of traffic at first, then slowly
shifting traffic over as you gain confidence (from monitoring) that it is
performing as expected. Depending on the rate at which you shift the traffic and
the level of monitoring you have in place, a deployment like this might be
called a zero-downtime, blue-green, canary deployment, or something else.

In this guide you will deploy a new version of a service and shift HTTP
traffic slowly to the new version.

## Prerequisites

The steps in this guide use Consul’s service mesh feature, Consul Connect. If
you aren’t already familiar with Connect you can learn more by following [this
guide](https://learn.hashicorp.com/consul/getting-started/connect).

We created a demo environment for the steps we describe here. The environment
relies on Docker and Docker Compose. If you do not already have Docker and
Docker Compose, you can install them from [Docker’s install
page](https://docs.docker.com/install/).

## Environment

This guide uses a two-tiered application made of of three services: a
public web service, two versions of the API service, and Consul. The Web service
accepts incoming traffic and makes an upstream call to the API service. At the
start of this scenario version 1 of the API service is already running in
production and handling traffic. Version 2 contains some changes you will ship
in a canary deployment.

![Architecture diagram of the splitting demo. A web service directly connects to two different versions of the API service through proxies. Consul configures those proxies.](/static/img/consul-splitting-architecture.png)

## Start the Environment

First clone the repo containing the source and examples for this guide.

```shell
$ git clone git@github.com:hashicorp/consul-demo-traffic-splitting.git
```

Change directories into the cloned folder, and start the demo environment with
`docker-compose up`. This command will run in the foreground, so you’ll need to
open a new terminal window after you run it.

```shell
$ docker-compose up

Creating consul-demo-traffic-splitting_api_v1_1    ... done
Creating consul-demo-traffic-splitting_consul_1 ... done
Creating consul-demo-traffic-splitting_web_1    ... done
Creating consul-demo-traffic-splitting_web_envoy_1    ... done
Creating consul-demo-traffic-splitting_api_proxy_v1_1 ... done
Attaching to consul-demo-traffic-splitting_consul_1, consul-demo-traffic-splitting_web_1, consul-demo-traffic-splitting_api_v1_1, consul-demo-traffic-splitting_web_envoy_1, consul-demo-traffic-splitting_api_proxy_v1_1
```

Consul is preconfigured to run as a single server, with all the
configurations for splitting enabled.

- Connect is enabled - Traffic shaping requires that you use Consul Connect.

- gRPC is enabled - splitting also requires the you use Envoy as a sidecar
proxy, and Envoy gets its configuration from Consul via gRPC.

- Central service configuration is enabled - you will use configuration entries
to specify the API service protocol, and define your splitting ratios.

These settings are defined in the Consul configuration file at
`consul_config/consul.hcl`, which contains the follwoing.

```hcl
data_dir = "/tmp/"
log_level = "DEBUG"
server = true

bootstrap_expect = 1
ui = true

bind_addr = "0.0.0.0"
client_addr = "0.0.0.0"

connect {
  enabled = true
}

ports {
  grpc = 8502
}

enable_central_service_config = true
```

You can find the service definitions for this demo in the `service_config`
folder. Note the metadata stanzas in the registrations for versions 1 and 2 of
the API service. Consul will use the metadata you define here to split traffic
between the two services. The metadata stanza contains the following.

```json
"meta": {
  "version": "1"
},
```

Once everything is up and running, you can view the health of the registered
services by checking the Consul UI at
[http://localhost:8500](http://localhost:8500). The docker compose file has
started and registered Consul, the web service, a sidecar for the web service,
version 1 of the API service, and a sidecar for the API service.

![List of services in the Consul UI including Consul, and the web and API services with their proxies](/static/img/consul-splitting-services.png)

Curl the Web endpoint to make sure that the whole application is running. The
Web service will get a response from version 1 of the API service.

```hcl
$ curl localhost:9090
Hello World
###Upstream Data: localhost:9091###
  Service V1
```

Initially, you will want to deploy version 2 of the API service to production
without sending any traffic to it, to make sure that it performs well in a new
environment. Prevent traffic from flowing to version 2 when you register it, you
will preemptively set up a traffic split to send 100% of your traffic to
version 1 of the API service, and 0% to the not-yet-deployed version 2.

## Configure Traffic Splitting

Traffic splitting makes use of configuration entries to centrally configure
services and Envoy proxies. There are three configuration entries you need to
create to enable traffic splitting:

- Service defaults for the API service to set the protocol to HTTP.
- Service splitter which defines the traffic split between the service subsets.
- Service resolver which defines which service instances are version 1 and 2.

### Configuring Service Defaults

Traffic splitting requires that the upstream application uses HTTP, because
splitting happens on layer 7 (on a request-by-request basis). You will tell
Consul that your upstream service uses HTTP by setting the protocol in a
service-defaults configuration entry for the API service. This configuration
is already in your demo environment at `l7_config/api_service_defaults.json`. It
contains the following.

```json
{
  "kind": "service-defaults",
  "name": "api",
  "protocol": "http"
}
```

To apply the configuration, you can either use the Consul CLI or the API. In
this example we’ll use the CLI to write the configuration, providing the file location.

```shell
$ consul config write l7_config/api_service_defaults.json
```

Find more information on `service-defaults` configuration entries in the
[documentation](https://www.consul.io/docs/agent/config-entries/service-defaults.html).

-> **Automation Tip:** To automate interactions with configuration entries, use
the HTTP API endpoint [`http://localhost:8500/v1/config`](https://www.consul.io/api/config.html).

### Configuring the Service Resolver

The next configuration entry you need to add is the service resolver, which
allows you to define how Consul’s service discovery selects service instances
for a given service name.

Service resolvers allow you to filter for subsets of services based on
information in the service registration. In this example, we are going to define
the subsets “v1” and “v2” for the API service, based on their registered
metadata. API service version 1 in the demo is already registered with the
service metadata `version:1`, and an optional tag, `v1`, to make the version
number appear in the UI. When you register version 2 you will give it the
metadata `version:2`, which Consul will use to find the right service, and
optional tag `v2`. The `name` field is set to the name of the service in the
Consul service catalog.

The service resolver is already in your demo environment at
`l7_config/api_service_resolver.json` and it contains the following
configuration.

```json
{
  "kind": "service-resolver",
  "name": "api",

  "subsets": {
    "v1": {
      "filter": "Service.Meta.version == 1"
    },
    "v2": {
      "filter": "Service.Meta.version == 2"
    }
  }
}
```

Write the service resolver configuration entry using the CLI and providing the
location, just like in the previous example.

```shell
$ consul config write l7_config/api_service_resolver.json
```

Find more information about service resolvers in the
[documentation](https://www.consul.io/docs/agent/config-entries/service-resolver.html).

### Configure Service Splitting - 100% of traffic to Version 1

Next, you’ll create a configuration entry that will split percentages of traffic
to the subsets of your upstream service that you just defined. Initially, you
want the splitter to send all traffic to v1 of your upstream service, which
prevents any traffic from being sent to v2 when you register it. In a production
scenario, this would give you time to make sure that v2 of your service is up
and running as expected before sending it any real traffic.

The configuration entry for service splitting has the `kind` of
`service-splitter`. Its `name` specifies which service that the splitter will
act on. The `splits` field takes an array which defines the different splits; in
this example, there are only two splits; however, it is [possible to configure
multiple sequential
splits](https://www.consul.io/docs/connect/l7-traffic-management.html#splitting).

Each split has a `weight` which defines the percentage of traffic to distribute
to each service subset. The total weights for all splits must equal 100. For
your initial split, configure all traffic to be directed to the service subset
v1.

The service splitter already exists in your demo environment at
`l7_config/api_service_splitter_100_0.json` and contains the following
configuration.

```json
{
  "kind": "service-splitter",
  "name": "api",
  "splits": [
    {
      "weight": 100,
      "service_subset": "v1"
    },
    {
      "weight": 0,
      "service_subset": "v2"
    }
  ]
}
```

Write this configuration entry using the CLI as well.

```shell
$ consul config write l7_config/api_service_splitter_100_0.json
```

This concludes the set up of the first stage in your deployment; you can now
launch the new version of the API service without it immediately being used.

### Start and Register API Service Version 2

Next you’ll start version 2 of the API service, and register it with the
settings that you used in the configuration entries for resolution and
splitting. Start the service, register it, and start its connect sidecar with
the following command. This command will run in the foreground, so you’ll need
to open a new terminal window after you run it.

```shell
$ docker-compose -f docker-compose-v2.yml up
```

Check that the service and its proxy have registered by checking for new `v2`
tags next to the API service and API sidecar proxies in the Consul UI.

### Configure Service Splitting - 50% Version 1, 50% Version 2

Now that version 2 is running and registered, the next step is to gradually
increase traffic to it by changing the weight of the v2 service subset in the
service splitter configuration. In this example you will increase the percent of
traffic destined for the the v2 service to 50%. In a production roll out you
would typically set the initial percent to be much lower. You can specify
percentages as low as 0.01%.

Remember; total service percent must equal 100, so in this example you will
reduce the percent of the v1 subset to 50. The configuration file is already in
your demo environment at `l7_config/api_service_splitter_50_50.json` and it
contains the following.

```json
{
  "kind": "service-splitter",
  "name": "api",
  "splits": [
    {
      "weight": 50,
      "service_subset": "v1"
    },
    {
      "weight": 50,
      "service_subset": "v2"
    }
  ]
}
```

Write the new configuration using the CLI.

```shell
$ consul config write l7_config/api_service_splitter_50_50.json
```

Now that you’ve increased the percentage of traffic to v2, curl the web service
again. Consul will equally distribute traffic across both of the service
subsets.

```hcl
$ curl localhost:9090
Hello World
###Upstream Data: localhost:9091###
  Service V1
$ curl localhost:9090
Hello World
###Upstream Data: localhost:9091###
  Service V2
$ curl localhost:9090
Hello World
###Upstream Data: localhost:9091###
  Service V1
```

### Configure Service Splitting - 100% Version 2

Once you are confident that the new version of the service is operating
correctly, you can send 100% of traffic to the version 2 subset. The
configuration for a 100% split to version 2 contains the following.

```json
{
  "kind": "service-splitter",
  "name": "api",
  "splits": [
    {
      "weight": 0,
      "service_subset": "v1"
    },
    {
      "weight": 100,
      "service_subset": "v2"
    }
  ]
}
```

Apply it with the CLI, providing the path to the configuration entry.

```shell
$ consul config write l7_config/api_service_splitter_0_100.json
```

Now when you curl the web service again. 100% of traffic goes to the version
2 subset.

```hcl
$ curl localhost:9090
Hello World
###Upstream Data: localhost:9091###
  Service V2
$ curl localhost:9090
Hello World
###Upstream Data: localhost:9091###
  Service V2
$ curl localhost:9090
Hello World
###Upstream Data: localhost:9091###
  Service V2
```

Typically in a production environment, you would now remove the version 1
service to release capacity in your cluster. Once you remove version 1's
registration from Consul you can either remove the splitter and resolver
entirely, or leave them in place, removing the stanza that sends traffic to
version 1, so that you can eventually deploy version 3 without it receiving any
initial traffic.

Congratulations, you’ve now completed the deployment of version 2 of your
service.

## Demo Cleanup

To stop and remove the containers and networks that you created you will run
`docker-compose down` twice: once for each of the docker compose commands you
ran. Because containers you created in the second compose command are running on
the network you created in the first command, you will need to bring down the
environments in the opposite order that you created them in.

First you’ll stop and remove the containers created for v2 of the API service.

```shell
$ docker-compose -f docker-compose-v2.yml down
Stopping consul-demo-traffic-splitting_api_proxy_v2_1 ... done
Stopping consul-demo-traffic-splitting_api_v2_1       ... done
WARNING: Found orphan containers (consul-demo-traffic-splitting_api_proxy_v1_1, consul-demo-traffic-splitting_web_envoy_1, consul-demo-traffic-splitting_consul_1, consul-demo-traffic-splitting_web_1, consul-demo-traffic-splitting_api_v1_1) for this project. If you removed or renamed this service in your compose file, you can run this command with the --remove-orphans flag to clean it up.
Removing consul-demo-traffic-splitting_api_proxy_v2_1 ... done
Removing consul-demo-traffic-splitting_api_v2_1       ... done
Network consul-demo-traffic-splitting_vpcbr is external, skipping
```

Then, you’ll stop and remove the containers and the network that you created in
the first docker compose command.

```shell
$ docker-compose down
Stopping consul-demo-traffic-splitting_api_proxy_v1_1 ... done
Stopping consul-demo-traffic-splitting_web_envoy_1    ... done
Stopping consul-demo-traffic-splitting_consul_1       ... done
Stopping consul-demo-traffic-splitting_web_1          ... done
Stopping consul-demo-traffic-splitting_api_v1_1       ... done
Removing consul-demo-traffic-splitting_api_proxy_v1_1 ... done
Removing consul-demo-traffic-splitting_web_envoy_1    ... done
Removing consul-demo-traffic-splitting_consul_1       ... done
Removing consul-demo-traffic-splitting_web_1          ... done
Removing consul-demo-traffic-splitting_api_v1_1       ... done
Removing network consul-demo-traffic-splitting_vpcbr
```

## Summary

In this guide, we walked you through the steps required to perform Canary
deployments using traffic splitting and resolution.

Find out more about L7 traffic management settings in the
[documentation](https://www.consul.io/docs/connect/l7-traffic-management.html).

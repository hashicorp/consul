---
layout: "intro"
page_title: "Web UI"
sidebar_current: "gettingstarted-ui"
description: |-
  Consul comes with support for beautiful, functional web UIs out of the box. UIs can be used for viewing all services and nodes, for viewing all health checks and their current status, and for reading and setting key/value data. The UIs automatically supports multi-datacenter.
---

# Consul Web UI

Consul comes with support for beautiful, functional web UIs out of the
box. UIs can be used for viewing all services and nodes, for viewing
all health checks and their current status, and for reading and setting
key/value data. The UIs automatically support multi-datacenter.

There are two options for running a web UI for Consul: using
[Atlas by HashiCorp](https://atlas.hashicorp.com) to host the
dashboard for you or self-hosting the
[open-source UI](/downloads.html).

## Atlas-hosted Dashboard

<div class="center">
![Atlas Web UI](atlas_web_ui.png)
</div>

To set up the Atlas UI for Consul, you must add two fields to your
configuration: the
[name of your Atlas infrastructure](/docs/agent/options.html#_atlas)
and [your Atlas token](/docs/agent/options.html#_atlas_token). Below is
an example command-line invocation of the Consul agent providing these
settings:

```text
$ consul agent -atlas=ATLAS_USERNAME/demo -atlas-token="ATLAS_TOKEN"
```
To get an Atlas username and token,
[create an account](https://atlas.hashicorp.com/account/new?utm_source=oss&utm_medium=getting-started-ui&utm_campaign=consul)
and replace the respective values in your Consul configuration with
your credentials.

You can view a live demo
[here](https://atlas.hashicorp.com/hashicorp/environments/consul-demo).

## Self-hosted Dashboard

<div class="center">
![Consul Web UI](consul_web_ui.png)
</div>

To set up the self-hosted UI, start the Consul agent with the
[`-ui` parameter](/docs/agent/options.html#_ui):

```text
$ consul agent -ui
...
```

The UI is available at the `/ui` path on the same port as the HTTP API.
By default this is `http://localhost:8500/ui`.

You can view a live demo of the Consul Web UI
[here](http://demo.consul.io).

While the live demo is able to access data from all datacenters,
we've also setup demo endpoints in the specific datacenters:
[AMS2](http://ams2.demo.consul.io) (Amsterdam),
[SFO1](http://sfo1.demo.consul.io) (San Francisco),
and [NYC3](http://nyc3.demo.consul.io) (New York).

## Next Steps

This concludes our Getting Started guide.  See the
[next steps](next-steps.html) page to learn more about how to continue
your journey with Consul!

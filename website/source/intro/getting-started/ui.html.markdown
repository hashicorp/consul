---
layout: "intro"
page_title: "Web UI"
sidebar_current: "gettingstarted-ui"
description: |-
  Consul comes with support for a beautiful, functional web UI out of the box. This UI can be used for viewing all services and nodes, viewing all health checks and their current status, and for reading and setting key/value data. The UI automatically supports multi-datacenter.
---

# Consul Web UI

Consul comes with support for a beautiful, functional web UI.
The UI can be used for viewing all services and nodes, viewing all
health checks and their current status, and for reading and setting
key/value data. The UI automatically supports multi-datacenter.

There are two options for running a web UI for Consul. The first option is self-hosting and using the [open-source UI](/downloads.html), the second option is using [Atlas by HashiCorp](https://atlas.hashicorp.com) to host the dashboard for you. 

## Atlas-hosted Dashboard
To setup the Atlas UI for Consul, you must add two fields to your configuration — the name of your Atlas infrastructure and your Atlas token. Below is an example configuration:

```text
$ consul agent -atlas=ATLAS_USERNAME/demo -atlas-token="ATLAS_TOKEN"
```

To get an Atlas username and token, [create an account here](https://atlas.hashicorp.com/account/new?utm_source=oss&utm_medium=getting-started-ui&utm_campaign=consul) and replace the respective values in your Consul configuration with your credentials.

You can view a live demo of the Atlas UI [here](https://atlas.hashicorp.com/hashicorp/infrastructures/consul-demo). 

A screenshot of one page of the demo is shown below so you can get an
idea of what the web UI is like.

<div class="center">
![Atlas Web UI](atlas_web_ui.png)
</div>

## Self-hosted Dashboard
To set up the self-hosted UI,
[download the web UI package](/downloads.html)
and unzip it to a directory somewhere on the server where the Consul agent
is also being run. Then append the `-ui-dir` to the `consul agent`
command pointing to the directory where you unzipped the UI (the
directory with the `index.html` file):

```text
$ consul agent -ui-dir /path/to/ui
...
```

The UI is available at the `/ui` path on the same port as the HTTP API.
By default this is `http://localhost:8500/ui`.
-datacenter.

You can view a live demo of the Consul Web UI
[here](http://demo.consul.io).

While the live demo is able to access data from all datacenters,
we've also setup demo endpoints in the specific datacenters:
[AMS2](http://ams2.demo.consul.io) (Amsterdam),
[SFO1](http://sfo1.demo.consul.io) (San Francisco),
and [NYC3](http://nyc3.demo.consul.io) (New York).

A screenshot of one page of the demo is shown below so you can get an
idea of what the web UI is like. Click the screenshot for the full size.

<div class="center">
![Consul Web UI](consul_web_ui.png)
</div>
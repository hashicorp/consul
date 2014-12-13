---
layout: "intro"
page_title: "Web UI"
sidebar_current: "gettingstarted-ui"
description: |-
  Consul comes with support for a beautiful, functional web UI out of the box. This UI can be used for viewing all services and nodes, viewing all health checks and their current status, and for reading and setting key/value data. The UI automatically supports multi-datacenter.
---

# Consul Web UI

Consul comes with support for a
[beautiful, functional web UI](http://demo.consul.io) out of the box.
This UI can be used for viewing all services and nodes, viewing all
health checks and their current status, and for reading and setting
key/value data. The UI automatically supports multi-datacenter.

For ease of deployment, the UI is
[distributed](/downloads.html)
as static HTML and JavaScript.
You don't need a separate web server to run the web UI. The Consul
agent itself can be configured to serve the UI.

## Screenshot and Demo

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

## Set Up

To set up the web UI,
[download the web UI package](/downloads.html)
and unzip it to a directory somewhere on the server where the Consul agent
is also being run. Then, just append the `-ui-dir` to the `consul agent`
command pointing to the directory where you unzipped the UI (the
directory with the `index.html` file):

```text
$ consul agent -ui-dir /path/to/ui
...
```

The UI is available at the `/ui` path on the same port as the HTTP API.
By default this is `http://localhost:8500/ui`.

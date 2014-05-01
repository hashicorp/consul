---
layout: "intro"
page_title: "Web UI"
sidebar_current: "gettingstarted-ui"
---

# Consul Web UI

Consul comes with support for a
[beautiful, functional web UI](http://demo.consul.io) out of the box.
This UI can be used for viewing all services and nodes, viewing all
health checks and their current status, and for reading and setting
key/value data. The UI automatically supports multi-datacenter.

For ease of deployment, the UI is
[distributed](/downloads_web_ui.html)
as static HTML and JavaScript.
You don't need a separate web server to run the web UI. The Consul
agent itself can be configured to serve the UI.

## Screenshot and Demo

You can view a live demo of the Consul Web UI
[here](http://demo.consul.io).

A screenshot of one page of the demo is shown below so you can get an
idea of what the web UI is like. Click the screenshot for the full size.

<div class="center">
<a href="/images/consul_web_ui.png">
<img src="/images/consul_web_ui.png">
</a>
</div>

## Set Up

To set up the web UI,
[download the web UI package](/downloads_web_ui.html)
and unzip it to a directory somewhere on the server where the Consul agent
is also being run. Then, just append the `-ui-dir` to the `consul agent`
command pointing to the directory where you unzipped the UI (the
directory with the `index.html` file):

```
$ consul agent -ui-dir /path/to/ui
...
```

The UI is available at the `/ui` path on the same port as the HTTP API.
By default this is `http://localhost:8500/ui`.

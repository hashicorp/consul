---
name: 'Web UI'
content_length: 4
layout: content_layout
description: |-
  Consul comes with support for a user-friendly and functional web UI out of the box. In this guide we will explore the web UI.
id: ui
products_used:
  - Consul
level: Beginner
wistia_video_id: t0gc0r51me
---

Consul comes with support for a user-friendly and functional web UI out of the
box. UIs can be used for viewing all services and nodes, for viewing
all health checks and their current status, and for reading and setting
key/value data. The UIs automatically support multi-datacenter.

To set up the self-hosted UI, start the local Consul agent with the
[`-ui` parameter](https://consul.io/docs/agent/options.html#_ui):

```text
$ consul agent -dev -ui
...
```

The UI is available at the `/ui` path on the same port as the HTTP API.
By default this is `http://localhost:8500/ui`.

Access to the UI can be secured with [ACLs](/consul/advanced/day-1-operations/acl-guide#create-tokens-for-ui-use-optional-). Once the ACLs have been [bootstrapped](/consul/advanced/day-1-operations/acl-guide), you can limit read, write, and updated permissions for the various pages in the UI. 

You can view a live demo of the Consul Web UI
[here](http://demo.consul.io).

## How to Use the Legacy UI

As of Consul version 1.2.0 the original Consul UI is deprecated. You can
still enable it by setting the environment variable `CONSUL_UI_LEGACY` to `true`.
Without this environment variable, the web UI will default to the latest version.
To use the latest UI version, either set `CONSUL_UI_LEGACY` to false or don't
include that environment variable at all.

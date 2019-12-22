---
layout: "docs"
page_title: "Connect - Nomad"
sidebar_current: "docs-connect-platform-nomad"
description: |-
  Connect can be used with [Nomad](https://www.nomadproject.io) to provide secure service-to-service communication between Nomad jobs. The ability to use the dynamic port feature of Nomad makes Connect particularly easy to use.
---

# Connect on Nomad

Consul Connect can be used with [Nomad](https://www.nomadproject.io) to provide
secure service-to-service communication between Nomad jobs and task groups. The ability to
use the [dynamic port](https://www.nomadproject.io/docs/job-specification/network.html#dynamic-ports)
feature of Nomad makes Connect reduces operational complexity. 

For more information
about using Consul Connect with Nomad, select one of the following resources.

For a step-by-step guide on using Consul Connect with Nomad:

- [Nomad Consul Connect Guide](https://www.nomadproject.io/guides/integrations/consul-connect/index.html)

For reference information about configuring Nomad jobs to use Consul Connect:

- [Nomad Job Specification - `connect`](https://www.nomadproject.io/docs/job-specification/connect.html)
- [Nomad Job Specification - `sidecar_service`](https://www.nomadproject.io/docs/job-specification/sidecar_service.html)
- [Nomad Job Specification - `sidecar_task`](https://www.nomadproject.io/docs/job-specification/sidecar_task.html)
- [Nomad Job Specification - `proxy`](https://www.nomadproject.io/docs/job-specification/proxy.html)
- [Nomad Job Specification - `upstreams`](https://www.nomadproject.io/docs/job-specification/upstreams.html)

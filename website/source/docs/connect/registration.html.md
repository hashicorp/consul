---
layout: "docs"
page_title: "Connect - Proxy Registration"
sidebar_current: "docs-connect-registration"
description: |-
  To make connect aware of proxies you will need to register them as Consul services. This section describes the process and options for proxy registration.
---

# Proxy Registration

To make connect aware of proxies you will need to register them in a [service
definition](/docs/agent/services.html), just like you would register any other service with Consul. This section outlines your options for registering Connect proxies, either using independent registrations, or in nested sidecar registrations.

## Proxy Service Registration

To register proxies with independent proxy service registrations, you can define them in either in config files or via the API just like any other service. Learn more about all of the options you can define when registering your proxy service in the [proxy registration documentation](/docs/connect/registration/service-registration.html).

## Sidecar Service Registration

To reduce the amount of boilerplate needed for a sidecar proxy,
application service definitions may define inline, which are an opinionated
shorthand for a separate full proxy registration as described here. For a
description of the opinionated defaults, see the [sidecar service registrations
documentation](/docs/connect/registration/sidecar-service.html).

---
layout: "intro"
page_title: "Consul Connect"
sidebar_current: "gettingstarted-connect"
description: |-
  Connect is a feature of Consul that provides service-to-service connection authorization and encryption using mutual TLS. This ensures that all service communication in your datacenter is encrypted and that the rules of what services can communicate is centrally managed with Consul.
---

# Connect

We've now registered our first service with Consul and we've shown how you
can use the HTTP API or DNS interface to query the address and directly connect
to that service. Consul also provides a feature called **Connect** for
automatically connecting via an encrypted TLS connection and authorizing
which services are allowed to connect to each other.

Applications do not need to be modified at all to use Connect.
[Sidecar proxies](/docs/connect/proxies.html) can be used
to automatically establish TLS connections for inbound and outbound connections
without being aware of Connect at all. Applications may also
[natively integrate with Connect](/docs/connect/native.html)
for optimal performance and security.

-> **Security note:** The getting started guide will show Connect features and
focus on ease of use with a dev-mode agent. We will _not setup_ Connect in a
production-recommended secure way. Please read the [Connect production
guide](/docs/guides/connect-production.html) to understand the tradeoffs.

## Starting a Connect-unaware Service

Let's begin by starting a service that is unaware of Connect all. To
keep it simple, let's just use `socat` to start a basic echo service. This
service will accept TCP connections and echo back any data sent to it. If
`socat` isn't installed on your machine, it should be easily available via
a package manager.

```sh
$ socat -v tcp-l:8181,fork exec:"/bin/cat"
```

You can verify it is working by using `nc` to connect directly to it. Once
connected, type some text and press enter. The text you typed should be
echoed back:

```
$ nc 127.0.0.1 8181
hello
hello
echo
echo
```

`socat` is a decades-old Unix utility and our process is configured to
only accept a basic TCP connection. It has no concept of encryption, the
TLS protocol, etc. This can be representative of an existing service in
your datacenter such as a database, backend web service, etc.

## Registering the Service with Consul and Connect

Next, let's register the service with Consul. We'll do this by writing
a new service definition. This is the same as the previous step in the
getting started guide, except this time we'll also configure Connect.

```sh
$ cat <<EOF | sudo tee /etc/consul.d/socat.json
{
  "service": {
    "name": "socat",
    "port": 8181,
    "connect": { "proxy": {} }
  }
}
EOF
```

After saving this, run `consul reload` or send a `SIGHUP` signal to Consul
so it reads the new configuration.

Notice the only difference is the line starting with `"connect"`. The
existence of this empty configuration notifies Consul to manage a
proxy process for this process.
The proxy process represents that specific service. It accepts inbound
connections on a dynamically allocated port, verifies and authorizes the TLS
connection, and proxies back a standard TCP connection to the process.

## Connecting to the Service

Next, let's connect to the service. We'll first do this by using the
`consul connect proxy` command directly. This command manually runs a local
proxy that can represent a service. This is a useful tool for development
since it'll let you masquerade as any service (that you have permissions for)
and establish connections to other services via Connect.

The command below starts a proxy representing a service "web". We request
an upstream dependency of "socat" (the service we previously registered)
on port 9191. With this configuration, all TCP connections to 9191 will
perform service discovery for a Connect-capable "socat" endpoint and establish
a mutual TLS connection identifying as the service "web".

```sh
$ consul connect proxy -service web -upstream socat:9191
==> Consul Connect proxy starting...
    Configuration mode: Flags
               Service: web
              Upstream: socat => :9191
       Public listener: Disabled

...
```

With that running, we can verify it works by establishing a connection:

```
$ nc 127.0.0.1 9191
hello
hello
```

**The connection between proxies is now encrypted and authorized.**
We're now communicating to the "socat" service via a TLS connection.
The local connections to/from the proxy are unencrypted, but in production
these will be loopback-only connections. Any traffic in and out of the
machine is always encrypted.

## Registering a Dependent Service

We previously established a connection by directly running
`consul connect proxy`. Realistically, services need to establish connections
to dependencies over Connect. Let's register a service "web" that registers
"socat" as an upstream dependency:

```sh
$ cat <<EOF | sudo tee /etc/consul.d/web.json
{
  "service": {
    "name": "web",
    "port": 8080,
    "connect": {
      "proxy": {
        "config": {
          "upstreams": [{
             "destination_name": "socat",
             "local_bind_port": 9191
          }]
        }
      }
    }
  }
}
EOF
```

This configures a Consul-managed proxy for the service "web" that
is listening on port 9191 to establish connections to "socat" as "web". The
"web" service should then use that local port to talk to socat rather than
directly attempting to connect.

-> **Security note:** The Connect security model requires trusting
loopback connections when proxies are in use. To further secure this,
tools like network namespacing may be used.

## Controlling Access with Intentions

Intentions are used to define which services may communicate. Our connections
above succeeded because in a development mode agent, the ACL system is "allow
all" by default.

Let's insert a rule to deny access from web to socat:

```sh
$ consul intention create -deny web socat
Created: web => socat (deny)
```

With the proxy processes running that we setup previously, connection
attempts now fail:

```sh
$ nc 127.0.0.1 9191
$
```

Try deleting the intention (or updating it to allow) and attempting the
connection again. Intentions allow services to be segmented via a centralized
control plane (Consul). To learn more, read the reference documentation on
[intentions](/docs/connect/intentions.html).

Note that in the current release of Consul, changing intentions will not
affect existing connections. Therefore, you must establish a new connection
to see the effects of a changed intention. This will be addressed in the near
term in a future version of Consul.

## Next Steps

We've now configured a service on a single agent and used Connect for
automatic connection authorization and encryption. This is a great feature
highlight but let's explore the full value of Consul by [setting up our
first cluster](/intro/getting-started/join.html)!

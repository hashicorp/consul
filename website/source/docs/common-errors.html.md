---
layout: "docs"
page_title: "Common Error Messages"
sidebar_current: "docs-common-errors"
---

# Common Error Messages

When installing and running Consul, there are some common messages you might see. Usually they indicate an issue in your network or in your server's configuration. Some of the more common errors and their solutions are listed below.

If you are getting an error message you don't see listed on this page, please consider following our general [Troubleshooting Guide][troubleshooting].

## Configuration file errors

### Multiple network interfaces

```
Multiple private IPv4 addresses found. Please configure one with 'bind' and/or 'advertise'.
```
Your server has multiple active network interfaces. Consul needs to know which interface to use for local LAN communications. Add the [`bind`][bind] option to your configuration.

-> **Tip**: If your server does not have a static IP address, you can use a [go-sockaddr template][go-sockaddr] as the argument to the `bind` option, e.g. `"bind_addr": "{{GetInterfaceIP \"eth0\"}}"`.

### Configuration syntax errors

```
Error parsing config.hcl: At 1:12: illegal char
```
```
Error parsing config.hcl: At 1:32: key 'foo' expected start of object ('{') or assignment ('=')
```
```
Error parsing server.json: invalid character '`' looking for beginning of value
```
There is a syntax error in your configuration file. If the error message doesn't identify the exact location in the file where the problem is, try using [jq] to find it, for example:

```
$ consul agent -server -config-file server.json
==> Error parsing server.json: invalid character '`' looking for beginning of value
$ cat server.json | jq .
parse error: Invalid numeric literal at line 3, column 29
```

## Invalid host name

```
Node name "consul_client.internal" will not be discoverable via DNS due to invalid characters.
```
Add the [`node name`][node_name] option to your agent configuration and provide a valid DNS name.

## I/O timeouts

```
Failed to join 10.0.0.99: dial tcp 10.0.0.99:8301: i/o timeout
```
```
Failed to sync remote state: No cluster leader
```
If the Consul client and server are on the same LAN, then most likely, a firewall is blocking connections to the Consul server.

If they are not on the same LAN, check the [`retry_join`][retry_join] settings in the Consul client configuration. The client should be configured to join a cluster inside its local network.

## Deadline exceeded

```
Error getting server health from "XXX": context deadline exceeded
```
These error messages indicate a general performance problem on the Consul server. Make sure you are monitoring Consul telemetry and system metrics according to our [monitoring guide][monitoring]. Increase the CPU or memory allocation to the server if needed. Check the performance of the network between Consul nodes.

## Too many open files

```
Error accepting TCP connection: accept tcp [::]:8301: too many open files in system
```
```
Get http://localhost:8500/: dial tcp 127.0.0.1:31643: socket: too many open files
```
On a busy cluster, the operating system may not provide enough file descriptors to the Consul process. You will need to increase the limit for the Consul user, and maybe the system-wide limit as well. A good guide for Linux can be found [here][files].

Or, if you are starting Consul from `systemd`, you could add `LimitNOFILE=65536` to the unit file for Consul. You can see our example unit file [here][systemd].

## Snapshot close error

Our RPC protocol requires support for a TCP half-close in order to signal the other side that they are done reading the stream, since we don't know the size in advance. This saves us from having to buffer just to calculate the size.

If a host does not properly implement half-close you may see an error message `[ERR] consul: Failed to close snapshot: write tcp <source>-><destination>: write: broken pipe` when saving snapshots. This should not affect saving and restoring snapshots.

This has been a [known issue](https://github.com/docker/libnetwork/issues/1204) in Docker, but may manifest in other environments as well.

## ACL Not Found

```
RPC error making call: rpc error making call: ACL not found
```
This indicates that you have ACL enabled in your cluster, but you aren't passing a valid token. Make sure that when creating your tokens that they have the correct permissions set. In addition, you would want to make sure that an agent token is provided on each call.

## TLS and Certificates

### Incorrect certificate or certificate name

```
Remote error: tls: bad certificate
```
```
X509: certificate signed by unknown authority
```
Make sure that your Consul clients and servers are using the correct certificates, and that they've been signed by the same CA. The easiest way to do this is to follow [our guide][certificates].

If you generate your own certificates, make sure the server certificates include the special name `server.dc1.consul` in the Subject Alternative Name (SAN) field. (If you change the values of `datacenter` or `domain` in your configuration, update the SAN accordingly.)

### HTTP instead of HTTPS

```
Error querying agent: malformed HTTP response
```
```
Net/http: HTTP/1.x transport connection broken: malformed HTTP response "\x15\x03\x01\x00\x02\x02"
```

You are attempting to connect to a Consul agent with HTTP on a port that has been configured for HTTPS.

If you are using the Consul CLI, make sure you are specifying "https" in the `-http-addr` flag or the `CONSUL_HTTP_ADDR` environment variable.

If you are interacting with the API, change the URI scheme to "https".

## License warnings

```
License: expiration time: YYYY-MM-DD HH:MM:SS -0500 EST, time left: 29m0s
```
You have installed an Enterprise version of Consul. If you are an Enterprise customer, [provide a license key][license] to Consul before it shuts down. Otherwise, install the open-source Consul binary instead.

-> **Note:** Enterprise binaries can be identified on our [download site][releases] by the `+ent` suffix.


[troubleshooting]: https://learn.hashicorp.com/consul/day-2-operations/advanced-operations/troubleshooting
[node_name]: https://www.consul.io/docs/agent/options.html#node_name
[retry_join]: https://www.consul.io/docs/agent/options.html#retry-join
[license]: https://www.consul.io/docs/commands/license.html
[releases]: https://releases.hashicorp.com/consul/
[files]: https://easyengine.io/tutorials/linux/increase-open-files-limit
[certificates]: https://learn.hashicorp.com/consul/advanced/day-1-operations/certificates
[systemd]: https://learn.hashicorp.com/consul/advanced/day-1-operations/deployment-guide#configure-systemd
[monitoring]: https://learn.hashicorp.com/consul/advanced/day-1-operations/monitoring
[bind]: https://www.consul.io/docs/agent/options.html#_bind
[jq]: https://stedolan.github.io/jq/
[go-sockaddr]: https://godoc.org/github.com/hashicorp/go-sockaddr/template

---
layout: "docs"
page_title: "Environment"
sidebar_current: "docs-commands-environment"
description: |-
  Consul's behavior can be modified by certain environment variables.
---

# Environment variables

The Consul CLI will read the following environment variables to set
behavioral defaults. These can be overridden in all cases using
command-line arguments; see the 
[Consul Commands documentation](https://www.consul.io/docs/commands/index.html)
for details.

The following table describes these variables:

<table>
  <tr>
    <th>Variable name</th>
    <th>Value</th>
  </tr>
  <tr>
    <td><tt>CONSUL_HTTP_ADDR</tt></td>
    <td>The HTTP API address as a host:port pair</td>
  </tr>
  <tr>
    <td><tt>CONSUL_HTTP_TOKEN</tt></td>
    <td>The API access token required when access control lists (ACLs) are enabled</td>
  </tr>
  <tr>
    <td><tt>CONSUL_HTTP_AUTH</tt></td>
    <td>The HTTP Basic access credentials as a username:password pair</td>
  </tr>
  <tr>
    <td><tt>CONSUL_HTTP_SSL</tt></td>
    <td>Boolean (default False) to specify HTTPS connections</td>
  </tr>
  <tr>
    <td><tt>CONSUL_HTTP_SSL_VERIFY</tt></td>
    <td>Boolean (default True) to specify SSL certificate verification</td>
  </tr>
  <tr>
    <td><tt>CONSUL_RPC_ADDR</tt></td>
    <td>The RPC interface address as a host:port pair</td>
  </tr>
</table>

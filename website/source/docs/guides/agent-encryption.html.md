---
layout: "docs"
page_title: "Agent Communication Encryption"
sidebar_current: "docs-guides-agent-encryption"
description: |-
  This guide covers how to encrypt both gossip and RPC communication.
---

# Agent Communication Encryption

There are two different systems that need to be configured separately to encrypt communication within the cluster: gossip encryption and TLS. TLS is used to secure the RPC calls between agents. Gossip encryption is secured with a symmetric key, since gossip between nodes is done over UDP. In this guide we will configure both.

To complete the RPC encryption section, you must have [configured agent certificates](https://www.consul.io/docs/guides/creating-certificates.html).

## Gossip Encryption

To enable gossip encryption, you need to use an encryption key when starting the Consul agent. The key can be simple set with the `encrypt` parameter in the agent configuration file. Alternatively, the encryption key can be placed in a seperate configuration file with only the `encrypt` field, since the agent can merge multiple configuration files. The key must be 32-bytes, Base64 encoded. 

You can use the Consul CLI command, [`consul keygen`](/docs/commands/keygen.html), to generate a cryptographically suitable key.

```sh
$ consul keygen
pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=
```

### Enable Gossip Encryption: New Cluster

To enable gossip on a new cluster, we will add the encryption key parameter to the
agent configuration file and then pass the file at startup with the [`-config-dir`](https://www.consul.io/docs/agent/options.html#_config_dir) flag.

```javascript
{
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "bulldog",
  "server": true,
  "encrypt": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s="
}
```

```sh
$ consul agent -config-dir=/etc/consul.d/
==> Starting Consul agent...
==> Starting Consul agent RPC...
==> Consul agent running!
         Node name: 'Armons-MacBook-Air.local'
        Datacenter: 'dc1'
            Server: false (bootstrap: false)
       Client Addr: 127.0.0.1 (HTTP: 8500, HTTPS: -1, DNS: 8600, RPC: 8400)
      Cluster Addr: 10.1.10.12 (LAN: 8301, WAN: 8302)
    Gossip encrypt: true, RPC-TLS: false, TLS-Incoming: false
...
```

"Encrypt: true" will be included in the output, if encryption is properly configured.

Note: all nodes within a cluster must share the same encryption key in order to send and receive cluster information, including clients and servers. Additionally, if you're using multiple WAN joined datacenters, be sure to use _the same encryption key_ in all datacenters.

### Enable Gossip Encryption: Existing Cluster

Gossip encryption can also be enabled on an existing cluster, but requires several extra steps. The additional configuration of the agent configuration parameters, [`encrypt_verify_incoming`](/docs/agent/options.html#encrypt_verify_incoming) and [`encrypt_verify_outgoing`](/docs/agent/options.html#encrypt_verify_outgoing) is necessary. 

**Step 1**: Generate an encryption key using `consul keygen`.

```sh
$ consul keygen
pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=
```

**Step 2**: Set the [`encrypt`](/docs/agent/options.html#_encrypt) key, and set `encrypt_verify_incoming` and `encrypt_verify_outgoing` to `false` in the agent configuration file. Then initiate a rolling update of the cluster with these new values. After this step, the agents will be able to decrypt gossip but will not yet be sending encrypted traffic.

```javascript
{
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "bulldog",
  "server": true,
  "encrypt": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=",
  "encrypt_verify_incoming": false,
  "encrypt_verify_outgoing": false
}
```

A rolling update can be made by restarting the Consul agents (clients and servers) in turn. `consul reload` or `kill -HUP <process_id>` is _not_ sufficient to change the gossip configuration.

**Step 3**: Update the `encrypt_verify_outgoing` setting to `true` and perform another rolling update of the cluster by restarting Consul on each agent. The agents will now be sending encrypted gossip but will still allow incoming unencrypted traffic.

```javascript
{
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "bulldog",
  "server": true,
  "encrypt": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=",
  "encrypt_verify_incoming": false,
  "encrypt_verify_outgoing": true
}
``` 

**Step 4**: The previous step, enabling verify outgoing, must be completed on all agents before continuing. Update the `encrypt_verify_incoming` setting to `true` and perform a final rolling update of the cluster. 

```javascript
{
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "bulldog",
  "server": true,
  "encrypt": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=",
  "encrypt_verify_incoming": true,
  "encrypt_verify_outgoing": true
}
```

All the agents will now be strictly enforcing encrypted gossip. Note, the default
behavior of both `encrypt_verify_incoming` and `encrypt_verify_outgoing` is `true`.
We have set them in the configuration file as an explicit example. 

## TLS Encryption for RPC

Consul supports using TLS to verify the authenticity of servers and clients. To enable TLS,
Consul requires that all servers have certificates that are signed by a single
Certificate Authority. Clients may optionally authenticate with a client certificate generated by the same CA. Please see
[this tutorial on creating a CA and signing certificates](/docs/guides/creating-certificates.html).

TLS can be used to verify the authenticity of the servers with [`verify_outgoing`](/docs/agent/options.html#verify_outgoing) and [`verify_server_hostname`](/docs/agent/options.html#verify_server_hostname). It can also optionally verify client certificates when using [`verify_incoming`](/docs/agent/options.html#verify_incoming) 

Review the [docs for specifics](https://www.consul.io/docs/agent/encryption.html).

In Consul version 0.8.4 and newer, migrating to TLS encrypted traffic on a running cluster
is supported. 

### Enable TLS: New Cluster

After TLS has been configured on all the agents, you can start the agents and RPC communication will be encrypted.

```javascript
{
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "bulldog",
  "server": true,
  "encrypt": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=",
  "verify_incoming": true,
  "verify_outgoing": true,
  "verify_server_hostname": true,
  "ca_file": "consul-agent-ca.pem",
  "cert_file": "consul-server-dc1-0.pem",
  "key_file": "consul-server-dc1-0-key.pem"
}
```

Note, for clients, the default `cert_file` and `key_file` will be named according to their cluster for. For example, `consul-client-dc1-0.pem`.

The `verify_outgoing` parameter enables agents to verify the authenticity of Consul servers for outgoing connections. The `verify_server_hostname` parameter requires outgoing connections to perform hostname verification and is critically important to prevent compromised client agents from becoming servers and revealing all state to the attacker. Finally, the `verify_incoming` parameter enables the servers to verify the authenticity of all incoming connections. 

### Enable TLS: Existing Cluster

Enabling TLS on an existing cluster is supported. This process assumes a starting point of a running cluster with no TLS settings configured, and involves an intermediate step in order to get to full TLS encryption.

**Step 1**: [Generate the necessary keys and certificates](/docs/guides/creating-certificates.html), then set the `ca_file` or `ca_path`, `cert_file`, and `key_file` settings in the configuration for each agent. Make sure the `verify_outgoing` and `verify_incoming` options are set to `false`. HTTPS for the API can be enabled at this point by setting the [`https`](/docs/agent/options.html#http_port) port.

```javascript
{
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "bulldog",
  "server": true,
  "encrypt": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=",
  "verify_incoming": false,
  "verify_outgoing": false,
  "ca_file": "consul-agent-ca.pem",
  "cert_file": "consul-server-dc1-0.pem",
  "key_file": "consul-server-dc1-0-key.pem"
}
```

Next, perform a rolling restart of each agent in the cluster. After this step, TLS should be enabled everywhere but the agents will not yet be enforcing TLS. Again, `consul reload` or `kill -HUP <process_id>` is _not_ sufficient to update the configuration.


**Step 2**: (Optional, Enterprise-only) If applicable, set the `Use TLS` setting in any network areas to `true`. This can be done either through the [`consul operator area update`](/docs/commands/operator/area.html) command or the [Operator API](/api/operator/area.html).

**Step 3**: Change the `verify_incoming`, `verify_outgoing`, and `verify_server_hostname` to `true` then perform another rolling restart of each agent in the cluster.

```javascript
{
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "bulldog",
  "server": true,
  "encrypt": "pUqJrVyVRj5jsiYEkM/tFQYfWyJIv4s3XkvDwy7Cu5s=",
  "verify_incoming": true,
  "verify_outgoing": true,
  "verify_server_hostname": true,
  "ca_file": "consul-agent-ca.pem",
  "cert_file": "consul-server-dc1-0.pem",
  "key_file": "consul-server-dc1-0-key.pem"
}

```

At this point, full TLS encryption for RPC communication is enabled. To disable `HTTP`
connections, which may still be in use by clients for API and CLI communications, update
the [agent configuration](https://www.consul.io/docs/agent/options.html#ports).

## Summary

In this guide we configured both gossip encryption and TLS for RPC. Securing agent communication is a recommended set in setting up a production ready cluster. 


---
layout: "docs"
page_title: "Security Model"
sidebar_current: "docs-internals-security"
description: |-
  Consul relies on both a lightweight gossip mechanism and an RPC system to provide various features. Both of the systems have different security mechanisms that stem from their designs. However, the security mechanisms of Consul have a common goal: to provide confidentiality, integrity, and authentication.
---

# Security Model

Consul relies on both a lightweight gossip mechanism and an RPC system
to provide various features. Both of the systems have different security
mechanisms that stem from their designs. However, the security mechanisms
of Consul have a common goal: to provide
[confidentiality, integrity, and authentication](https://en.wikipedia.org/wiki/Information_security).

The [gossip protocol](/docs/internals/gossip.html) is powered by [Serf](https://www.serf.io/),
which uses a symmetric key, or shared secret, cryptosystem. There are more
details on the security of [Serf here](https://www.serf.io/docs/internals/security.html).
For details on how to enable Serf's gossip encryption in Consul, see the
[encryption doc here](/docs/agent/encryption.html).

The RPC system supports using end-to-end TLS with optional client authentication.
[TLS](https://en.wikipedia.org/wiki/Transport_Layer_Security) is a widely deployed asymmetric
cryptosystem and is the foundation of security on the Web.

This means Consul communication is protected against eavesdropping, tampering,
and spoofing. This makes it possible to run Consul over untrusted networks such
as EC2 and other shared hosting providers.

~> **Advanced Topic!** This page covers the technical details of
the security model of Consul. You don't need to know these details to
operate and use Consul. These details are documented here for those who wish
to learn about them without having to go spelunking through the source code.

## Secure Configuration

The Consul threat model is only applicable if Consul is running in a secure configuration. Consul does not operate in a secure-by-default configuration. If any of the settings below are not enabled, then parts of this threat model are going to be invalid. Additional security precautions must also be taken for items outside of Consul's threat model as noted in sections below.

* **ACLs enabled with default deny.** Consul must be configured to use ACLs with a whitelist (default deny) approach. This forces all requests to have explicit anonymous access or provide an ACL token.

* **Encryption enabled.** TCP and UDP encryption must be enabled and configured to prevent plaintext communication between Consul agents. At a minimum, verify_outgoing should be enabled to verify server authenticity with each server having a unique TLS certificate. verify_incoming provides additional agent verification, but shouldn't directly affect the threat model since requests must also contain a valid ACL token.

## Threat Model

The following are parts of the Consul threat model:

* **Consul agent-to-agent communication.** Communication between Consul agents should be secure from eavesdropping. This requires transport encryption to be enabled on the cluster and covers both TCP and UDP traffic.

* **Consul agent-to-CA communication.** Communication between the Consul server and the configured certificate authority provider for Connect is always encrypted.

* **Tampering of data in transit.** Any tampering should be detectable and cause Consul to avoid processing the request.

* **Access to data without authentication or authorization.** All requests must be authenticated and authorized. This requires that ACLs are enabled on the cluster with a default deny mode.

* **State modification or corruption due to malicious messages.** Ill-formatted messages are discarded and well-formatted messages require authentication and authorization.

* **Non-server members accessing raw data.**  All servers must join the cluster (with proper authentication and authorization) to begin participating in Raft. Raft data is transmitted over TLS.

* **Denial of Service against a node.** DoS attacks against a node should not compromise the security stance of the software.

* **Connect-based Service-to-Service communication.** Communications between two Connect-enabled services (natively or by proxy) should be secure from eavesdropping and provide authentication. This is achieved via mutual TLS.

The following are _not_ part of the Consul threat model for Consul server agents:

* **Access (read or write) to the Consul data directory.** All Consul servers, including non-leaders, persist the full set of Consul state to this directory. The data includes all KV, service registrations, ACL tokens, Connect CA configuration, and more. Any read or write to this directory allows an attacker to access and tamper with that data.

* **Access (read or write) to the Consul configuration directory.** Consul configuration can enable or disable the ACL system, modify data directory paths, and more. Any read or write of this directory allows an attacker to reconfigure many aspects of Consul. By disabling the ACL system, this may give an attacker access to all Consul data.

* **Memory access to a running Consul server agent.** If an attacker is able to inspect the memory state of a running Consul server agent the confidentiality of almost all Consul data may be compromised. If you're using an external Connect CA, the root private key material is never available to the Consul process and can be considered safe. Service Connect TLS certificates should be considered compromised; they are never persisted by server agents but do exist in-memory during at least the duration of a Sign request.

The following are _not_ part of the Consul threat model for Consul client agents:

* **Access (read or write) to the Consul data directory.** Consul clients will use the data directory to cache local state. This includes local services, associated ACL tokens, Connect TLS certificates, and more. Read or write access to this directory will allow an attacker to access this data. This data is typically a smaller subset of the full data of the cluster.

* **Access (read or write) to the Consul configuration directory.** Consul client configuration files contain the address and port information of services, default ACL tokens for the agent, and more. Access to Consul configuration could enable an attacker to change the port of a service to a malicious port, register new services, and more. Further, some service definitions have ACL tokens attached that could be used cluster-wide to impersonate that service. An attacker cannot change cluster-wide configurations such as disabling the ACL system.

* **Memory access to a running Consul client agent.** The blast radius of this is much smaller than a server agent but the confidentiality of a subset of data can still be compromised. Particularly, any data requested against the agent's API including services, KV, and Connect information may be compromised. If a particular set of data on the server was never requested by the agent, it never enters the agent's memory since replication only exists between servers. An attacker could also potentially extract ACL tokens used for service registration on this agent, since the tokens must be stored in-memory alongside the registered service.

* **Network access to a local Connect proxy or service.** Communications between a service and a Connect-aware proxy are generally unencrypted and must happen over a trusted network. This is typically a loopback device. This requires that other processes on the same machine are trusted, or more complex isolation mechanisms are used such as network namespaces. This also requires that external processes cannot communicate to the Connect service or proxy (except on the inbound port). Therefore, non-native Connect applications should only bind to non-public addresses.

* **Improperly Implemented Connect proxy or service.** A Connect proxy or natively integrated service must correctly serve a valid leaf certificate, verify the inbound TLS client certificate, and call the Consul agent-local authorize endpoint. If any of this isn't performed correctly, the proxy or service may allow unauthenticated or unauthorized connections.

## External Threat Overview

There are four components that affect the Consul threat model: the server agent, the client agent, the Connect CA, and Consul API clients (including proxies for Connect).

The server agent participates in leader election and data replication via Raft. All communications with other agents is encrypted. Data is stored at rest unencrypted in the configured data directory. The stored data includes ACL tokens and TLS certificates. If the built-in CA is used with Connect, root certificate private keys are also stored on disk. External CA providers do not store data in this directory. This data directory must be carefully protected to prevent an attacker from impersonating a server or specific ACL user. We plan to introduce further mitigations (including at least partial data encryption) to the data directory over time, but the data directory should always be considered secret.

For a client agent to join a cluster, it must provide a valid ACL token with node:write capabilities. The join request and all other API requests between the client and server agents communicate via TLS. Clients serve the Consul API and forward all requests to a server over a shared TLS connection. Each request contains an ACL token which is used for both authentication and authorization. Requests that do not provide an ACL token inherit the agent-configurable default ACL token.

The Connect CA provider is responsible for storing the private key of the root (or intermediate) certificate used to sign and verify connections established via Connect. Consul server agents communicate with the CA provider via an encrypted method. This method is dependent on the CA provider in use. Consul provides a built-in CA which performs all operations locally on the server agent. Consul itself does not store any private key material except for the built-in CA.

Consul API clients (the agent itself, the built-in UI, external software) must communicate to a Consul agent over TLS and must provide an ACL token per request for authentication and authorization.

## Network Ports

For configuring network rules to support Consul, please see [Ports Used](/docs/agent/options.html#ports)
for a listing of network ports used by Consul and details about which features
they are used for.

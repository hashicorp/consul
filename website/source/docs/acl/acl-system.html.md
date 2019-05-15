---
layout: "docs"
page_title: "ACL System"
sidebar_current: "docs-acl-system"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL system is a Capability-based system that relies on tokens which can have fine grained rules applied to them. It is very similar to AWS IAM in many ways.
---

-> **1.4.0 and later:** This guide only applies in Consul versions 1.4.0 and later. The documentation for the legacy ACL system is [here](/docs/acl/acl-legacy.html)

# ACL System

Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs.
The ACL is [Capability-based](https://en.wikipedia.org/wiki/Capability-based_security), relying on tokens which
are associated with policies to determine which fine grained rules can be applied. Consul's capability based
ACL system is very similar to the design of [AWS IAM](https://aws.amazon.com/iam/).

## ACL System Overview

The ACL system is designed to be easy to use and fast to enforce while providing administrative insight.
At the highest level, there are two major components to the ACL system:

 * **ACL Policies** - Policies allow the grouping of a set of rules into a logical unit that can be reused and linked with
 many tokens.

 * **ACL Tokens** - Requests to Consul are authorized by using bearer token. Each ACL token has a public
 Accessor ID which is used to name a token, and a Secret ID which is used as the bearer token used to
 make requests to Consul.

For many scenarios policies and tokens are sufficient, but more advanced setups
may benefit from additional components in the ACL system:

 * **ACL Roles** - Roles allow for the grouping of a set of policies and service
   identities into a reusable higher-level entity that can be applied to many
   tokens. (Added in Consul 1.5.0)

 * **ACL Service Identities** - Service identities are a policy template for
   expressing a link to a policy suitable for use in [Consul
   Connect](/docs/connect/index.html). At authorization time this acts like an
   additional policy was attached, the contents of which are described further
   below. These are directly attached to tokens and roles and are not
   independently configured. (Added in Consul 1.5.0)

 * **ACL Auth Methods and Binding Rules** - To learn more about these topics,
   see the [dedicated auth methods documentation page](/docs/acl/acl-auth-methods.html).

ACL tokens, policies, roles, auth methods, and binding rules are managed by 
Consul operators via Consul's [ACL API](/api/acl/acl.html), 
[ACL CLI](/docs/commands/acl.html), or systems like 
[HashiCorp's Vault](https://www.vaultproject.io/docs/secrets/consul/index.html).

### ACL Policies

An ACL policy is a named set of rules and is composed of the following elements:

* **ID** - The policy's auto-generated public identifier.
* **Name** - A unique meaningful name for the policy.
* **Description** - A human readable description of the policy. (Optional)
* **Rules** - Set of rules granting or denying permissions. See the [Rule Specification](/docs/acl/acl-rules.html#rule-specification) documentation for more details.
* **Datacenters** - A list of datacenters the policy is valid within.

#### Builtin Policies

* **Global Management** - Grants unrestricted privileges to any token that uses it. When created it will be named `global-management`
and will be assigned the reserved ID of `00000000-0000-0000-0000-000000000001`. This policy can be renamed but modification
of anything else including the rule set and datacenter scoping will be prevented by Consul.

### ACL Service Identities

-> Added in Consul 1.5.0

An ACL service identity is an [ACL policy](/docs/acl/acl-system.html#acl-policies) template for expressing a link to a policy
suitable for use in [Consul Connect](/docs/connect/index.html). They are usable
on both tokens and roles and are composed of the following elements:

* **Service Name** - The name of the service.
* **Datacenters** - A list of datacenters the effective policy is valid within. (Optional)

Services participating in the service mesh will need privileges to both _be
discovered_ and to _discover other healthy service instances_. Suitable
policies tend to all look nearly identical so a service identity is a policy
template to aid in avoiding boilerplate policy creation.

During the authorization process, the configured service identity is automatically
applied as a policy with the following preconfigured [ACL
rules](/docs/acl/acl-system.html#acl-rules-and-scope):

```hcl
// Allow the service and its sidecar proxy to register into the catalog.
service "<Service Name>" {
	policy = "write"
}
service "<Service Name>-sidecar-proxy" {
	policy = "write"
}

// Allow for any potential upstreams to be resolved.
service_prefix "" {
	policy = "read"
}
node_prefix "" {
	policy = "read"
}
```

The [API documentation for roles](/api/acl/roles.html#sample-payload) has some
examples of using a service identity.

### ACL Roles

-> Added in Consul 1.5.0

An ACL role is a named set of policies and service identities and is composed
of the following elements:

* **ID** - The role's auto-generated public identifier.
* **Name** - A unique meaningful name for the role.
* **Description** - A human readable description of the role. (Optional)
* **Policy Set** - The list of policies that are applicable for the role.
* **Service Identity Set** - The list of service identities that are applicable for the role.

### ACL Tokens

ACL tokens are used to determine if the caller is authorized to perform an action. An ACL token is composed of the following
elements:

* **Accessor ID** - The token's public identifier.
* **Secret ID** -The bearer token used when making requests to Consul.
* **Description** - A human readable description of the token. (Optional)
* **Policy Set** - The list of policies that are applicable for the token.
* **Role Set** - The list of roles that are applicable for the token. (Added in Consul 1.5.0)
* **Service Identity Set** - The list of service identities that are applicable for the token. (Added in Consul 1.5.0)
* **Locality** - Indicates whether the token should be local to the datacenter it was created within or created in
the primary datacenter and globally replicated.
* **Expiration Time** - The time at which this token is revoked. (Optional; Added in Consul 1.5.0)

#### Builtin Tokens

During cluster bootstrapping when ACLs are enabled both the special `anonymous` and the `master` token will be
injected.

* **Anonymous Token** - The anonymous token is used when a request is made to Consul without specifying a bearer token.
The anonymous token's description and policies may be updated but Consul will prevent this token's deletion. When created,
it will be assigned `00000000-0000-0000-0000-000000000002` for its Accessor ID and `anonymous` for its Secret ID.

* **Master Token** - When a master token is present within the Consul configuration, it is created and will be linked
With the builtin Global Management policy giving it unrestricted privileges. The master token is created with the Secret ID
set to the value of the configuration entry.

#### Authorization

The token Secret ID is passed along with each RPC request to the servers. Consul's
[HTTP endpoints](/api/index.html) can accept tokens via the `token`
query string parameter, the `X-Consul-Token` request header, or an 
[RFC6750](https://tools.ietf.org/html/rfc6750) authorization bearer token. Consul's
[CLI commands](/docs/commands/index.html) can accept tokens via the
`token` argument, or the `CONSUL_HTTP_TOKEN` environment variable. The CLI
commands can also accept token values stored in files with the `token-file`
argument, or the `CONSUL_HTTP_TOKEN_FILE` environment variable.

If no token is provided for an HTTP request then Consul will use the default ACL token
if it has been configured. If no default ACL token was configured then the anonymous
token will be used.

#### ACL Rules and Scope

The rules from all policies, roles, and service identities linked with a token are combined to form that token's
effective rule set. Policy rules can be defined in either a whitelist or blacklist
mode depending on the configuration of [`acl_default_policy`](/docs/agent/options.html#acl_default_policy).
If the default policy is to "deny" access to all resources, then policy rules can be set to
whitelist access to specific resources. Conversely, if the default policy is “allow” then policy rules can
be used to explicitly deny access to resources.

The following table summarizes the ACL resources that are available for constructing
rules:

| Resource                   | Scope |
| ------------------------ | ----- |
| [`acl`](#acl-rules)              | Operations for managing the ACL system [ACL API](/api/acl/acl.html) |
| [`agent`](#agent-rules)          | Utility operations in the [Agent API](/api/agent.html), other than service and check registration |
| [`event`](#event-rules)          | Listing and firing events in the [Event API](/api/event.html) |
| [`key`](#key-value-rules)        | Key/value store operations in the [KV Store API](/api/kv.html) |
| [`keyring`](#keyring-rules)      | Keyring operations in the [Keyring API](/api/operator/keyring.html) |
| [`node`](#node-rules)            | Node-level catalog operations in the [Catalog API](/api/catalog.html), [Health API](/api/health.html), [Prepared Query API](/api/query.html), [Network Coordinate API](/api/coordinate.html), and [Agent API](/api/agent.html) |
| [`operator`](#operator-rules)    | Cluster-level operations in the [Operator API](/api/operator.html), other than the [Keyring API](/api/operator/keyring.html) |
| [`query`](#prepared-query-rules) | Prepared query operations in the [Prepared Query API](/api/query.html)
| [`service`](#service-rules)      | Service-level catalog operations in the [Catalog API](/api/catalog.html), [Health API](/api/health.html), [Prepared Query API](/api/query.html), and [Agent API](/api/agent.html) |
| [`session`](#session-rules)      | Session operations in the [Session API](/api/session.html) |

Since Consul snapshots actually contain ACL tokens, the [Snapshot API](/api/snapshot.html)
requires a token with "write" privileges for the ACL system.

The following resources are not covered by ACL policies:

1. The [Status API](/api/status.html) is used by servers when bootstrapping and exposes
basic IP and port information about the servers, and does not allow modification
of any state.

2. The datacenter listing operation of the
[Catalog API](/api/catalog.html#list-datacenters) similarly exposes the names of known
Consul datacenters, and does not allow modification of any state.

3. The [connect CA roots endpoint](/api/connect/ca.html#list-ca-root-certificates) exposes just the public TLS certificate which other systems can use to verify the TLS connection with Consul.

Constructing rules from these policies is covered in detail on the
[ACL Rules](/docs/acl/acl-rules.html) page.

## Configuring ACLs

ACLs are configured using several different configuration options. These are marked
as to whether they are set on servers, clients, or both.

| Configuration Option | Servers | Clients | Purpose |
| -------------------- | ------- | ------- | ------- |
| [`acl.enabled`](/docs/agent/options.html#acl_enabled) | `REQUIRED` | `REQUIRED` | Controls whether ACLs are enabled |
| [`acl.default_policy`](/docs/agent/options.html#acl_default_policy) | `OPTIONAL` | `N/A` | Determines whitelist or blacklist mode |
| [`acl.down_policy`](/docs/agent/options.html#acl_down_policy) | `OPTIONAL` | `OPTIONAL` | Determines what to do when the remote token or policy resolution fails |
| [`acl.role_ttl`](/docs/agent/options.html#acl_role_ttl) | `OPTIONAL` | `OPTIONAL` | Determines time-to-live for cached ACL Roles |
| [`acl.policy_ttl`](/docs/agent/options.html#acl_policy_ttl) | `OPTIONAL` | `OPTIONAL` | Determines time-to-live for cached ACL Policies |
| [`acl.token_ttl`](/docs/agent/options.html#acl_token_ttl) | `OPTIONAL` | `OPTIONAL` | Determines time-to-live for cached ACL Tokens |

A number of special tokens can also be configured which allow for bootstrapping the ACL
system, or accessing Consul in special situations:

| Special Token | Servers | Clients | Purpose |
| ------------- | ------- | ------- | ------- |
| [`acl.tokens.agent_master`](/docs/agent/options.html#acl_tokens_agent_master) | `OPTIONAL` | `OPTIONAL` | Special token that can be used to access [Agent API](/api/agent.html) when remote bearer token resolution fails; used for setting up the cluster such as doing initial join operations, see the [ACL Agent Master Token](#acl-agent-master-token) section for more details |
| [`acl.tokens.agent`](/docs/agent/options.html#acl_tokens_agent) | `OPTIONAL` | `OPTIONAL` | Special token that is used for an agent's internal operations, see the [ACL Agent Token](#acl-agent-token) section for more details |
| [`acl.tokens.master`](/docs/agent/options.html#acl_tokens_master) | `OPTIONAL` | `N/A` | Special token used to bootstrap the ACL system, see the [Bootstrapping ACLs](https://learn.hashicorp.com/consul/advanced/day-1-operations/acl-guide) guide for more details |
| [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default) | `OPTIONAL` | `OPTIONAL` | Default token to use for client requests where no token is supplied; this is often configured with read-only access to services to enable DNS service discovery on agents |

All of these tokens except the `master` token can all be introduced or updated via the [/v1/agent/token API](/api/agent.html#update-acl-tokens).

#### ACL Agent Master Token

Since the [`acl.tokens.agent_master`](/docs/agent/options.html#acl_tokens_agent_master) is designed to be used when the Consul servers are not available, its policy is managed locally on the agent and does not need to have a token defined on the Consul servers via the ACL API. Once set, it implicitly has the following policy associated with it

```hcl
agent "<node name of agent>" {
  policy = "write"
}
node_prefix "" {
  policy = "read"
}
```

#### ACL Agent Token

The [`acl.tokens.agent`](/docs/agent/options.html#acl_tokens_agent) is a special token that is used for an agent's internal operations. It isn't used directly for any user-initiated operations like the [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default), though if the `acl.tokens.agent_token` isn't configured the `acl.tokens.default` will be used. The ACL agent token is used for the following operations by the agent:

1. Updating the agent's node entry using the [Catalog API](/api/catalog.html), including updating its node metadata, tagged addresses, and network coordinates
2. Performing [anti-entropy](/docs/internals/anti-entropy.html) syncing, in particular reading the node metadata and services registered with the catalog
3. Reading and writing the special `_rexec` section of the KV store when executing [`consul exec`](/docs/commands/exec.html) commands

Here's an example policy sufficient to accomplish the above for a node called `mynode`:

```hcl
node "mynode" {
  policy = "write"
}
service_prefix "" {
  policy = "read"
}
key_prefix "_rexec" {
  policy = "write"
}
```

The `service_prefix` policy needs read access for any services that can be registered on the agent. If [remote exec is disabled](/docs/agent/options.html#disable_remote_exec), the default, then the `key_prefix` policy can be omitted.

## Next Steps

Setup ACLs with the [Bootstrapping the ACL System guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/acl-guide) or continue reading about
[ACL rules](/docs/acl/acl-rules.html).

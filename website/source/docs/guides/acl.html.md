---
layout: "docs"
page_title: "ACL System"
sidebar_current: "docs-guides-acl"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL system is a Capability-based system that relies on tokens which can have fine grained rules applied to them. It is very similar to AWS IAM in many ways.
---

-> **1.4.0 and later:** This guide only applies in Consul versions 1.4.0 and later. The documentation for the legacy ACL system is [here](/docs/guides/acl-legacy.html)

# ACL System

Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs.
The ACL is [Capability-based](https://en.wikipedia.org/wiki/Capability-based_security), relying on tokens which
are associated with policies to determine which fine grained rules can be applied. Consul's capability based
ACL system is very similar to the design of [AWS IAM](https://aws.amazon.com/iam/).

## ACL System Overview

The ACL system is designed to be easy to use and fast to enforce while providing administrative insight.
At the highest level, there are two major components to the ACL system:

 * **ACL Policies** - Policies allow grouping of a set of rules into a logical unit that can be reused and linked with
 many tokens.

 * **ACL Tokens** - Requests to Consul are authorized by using bearer token. Each ACL token has a public
 Accessor ID which is used to name a token, and a Secret ID which is used as the bearer token used to
 make requests to Consul.

 ACL Tokens and Policies are managed by Consul operators via Consul's
[ACL API](/api/acl.html), ACL CLI or systems like
[HashiCorp's Vault](https://www.vaultproject.io/docs/secrets/consul/index.html).

### ACL Policies

An ACL policy is a named set of rules and is composed of the following elements:

* **ID** - The policies auto-generated public identifier.
* **Name** - A unique meaningful name for the policy.
* **Rules** - Set of rules granting or denying permissions. See the [Rule Specification](#rule-specification) section for more details.
* **Datacenters** - A list of datacenters the policy is valid within.

#### Builtin Policies

* **Global Management** - Grants unrestricted privileges to any token that uses it. When created it will be named `global-management`
and will be assigned the reserved ID of `00000000-0000-0000-0000-000000000001`. This policy can be renamed but modification
of anything else including the rule set and datacenter scoping will be prevented by Consul.

### ACL Tokens

ACL tokens are used to determine if the caller is authorized to perform an action. An ACL Token is composed of the following
elements:

* **Accessor ID** - The token's public identifier.
* **Secret ID** -The bearer token used when making requests to Consul.
* **Description** - A human readable description of the token. (Optional)
* **Policy Set** - The list of policies that are applicable for the token.
* **Locality** - Indicates whether the token should be local to the datacenter it was created within or created in
the primary datacenter and globally replicated.

#### Builtin Tokens

During cluster bootstrapping when ACLs are enabled both the special `anonymous` and the `master` token will be
injected.

* **Anonymous Token** - The anonymous token is used when a request is made to Consul without specifying a bearer token.
The anonymous token's description and policies may be updated but Consul will prevent this tokens deletion. When created,
it will be assigned `00000000-0000-0000-0000-000000000002` for its Accessor ID and `anonymous` for its Secret ID.

* **Master Token** - When a master token is present within the Consul configuration, it is created and will be linked
With the builtin Global Management policy giving it unrestricted privileges. The master token is created with the Secret ID
set to the value of the configuration entry.

#### Authorization

The token Secret ID is passed along with each RPC request to the servers. Consul's
[HTTP endpoints](/api/index.html) can accept tokens via the `token`
query string parameter, the `X-Consul-Token` request header, or Authorization Bearer
token [RFC6750](https://tools.ietf.org/html/rfc6750). Consul's
[CLI commands](/docs/commands/index.html) can accept tokens via the
`token` argument, or the `CONSUL_HTTP_TOKEN` environment variable.

If no token is provided for an HTTP request then Consul will use the default ACL token
if it has been configured. If no default ACL token was configured then the anonymous
token will be used.

#### ACL Rules and Scope

The rules from all policies linked with a token are combined to form that token's
effective rule set. Policy rules can be defined in either a whitelist or blacklist
mode depending on the configuration of [`acl_default_policy`](/docs/agent/options.html#acl_default_policy).
If the default policy is to "deny" access to all resources, then policy rules can be set to
whitelist access to specific resources. Conversely, if the default policy is “allow” then policy rules can
be used to explicitly deny access to resources.

The following table summarizes the ACL resources that are available for constructing
rules:

| Resource                   | Scope |
| ------------------------ | ----- |
| [`acl`](#acl-rules)              | Operations for managing the ACL system [ACL API](/api/acl.html) |
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

3. The [connect CA roots endpoint](/api/connect/ca.html#list-ca-root-certificates) exposes just the public TLS certificate which other systems can used to verify the TLS connection with Consul.

Constructing rules from these policies is covered in detail in the
[Rule Specification](#rule-specification) section below.

## Configuring ACLs

ACLs are configured using several different configuration options. These are marked
as to whether they are set on servers, clients, or both.

| Configuration Option | Servers | Clients | Purpose |
| -------------------- | ------- | ------- | ------- |
| [`acl.enabled`](/docs/agent/options.html#acl_enabled) | `REQUIRED` | `REQUIRED` | Controls whether ACLs are enabled |
| [`acl.default_policy`](/docs/agent/options.html#acl_default_policy) | `OPTIONAL` | `N/A` | Determines whitelist or blacklist mode |
| [`acl.down_policy`](/docs/agent/options.html#acl_down_policy) | `OPTIONAL` | `OPTIONAL` | Determines what to do when the remote token or policy resolution fails |
| [`acl.policy_ttl`](/docs/agent/options.html#acl_policy_ttl) | `OPTIONAL` | `OPTIONAL` | Determines time-to-live for cached ACL Policies |
| [`acl.token_ttl`](/docs/agent/options.html#acl_token_ttl) | `OPTIONAL` | `OPTIONAL` | Determines time-to-live for cached ACL Tokens |

A number of special tokens can also be configured which allow for bootstrapping the ACL
system, or accessing Consul in special situations:

| Special Token | Servers | Clients | Purpose |
| ------------- | ------- | ------- | ------- |
| [`acl.tokens.agent_master`](/docs/agent/options.html#acl_tokens_agent_master) | `OPTIONAL` | `OPTIONAL` | Special token that can be used to access [Agent API](/api/agent.html) when remote bearer token resolution fails; used for setting up the cluster such as doing initial join operations, see the [ACL Agent Master Token](#acl-agent-master-token) section for more details |
| [`acl.tokens.agent`](/docs/agent/options.html#acl_tokens_agent) | `OPTIONAL` | `OPTIONAL` | Special token that is used for an agent's internal operations, see the [ACL Agent Token](#acl-agent-token) section for more details |
| [`acl.tokens.master`](/docs/agent/options.html#acl_tokens_master) | `OPTIONAL` | `N/A` | Special token used to bootstrap the ACL system, see the [Bootstrapping ACLs](#bootstrapping-acls) section for more details |
| [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default) | `OPTIONAL` | `OPTIONAL` | Default token to use for client requests where no token is supplied; this is often configured with read-only access to services to enable DNS service discovery on agents |

All of these tokens except the `master` token can all be introduced or updated via the [/v1/agent/token API](/api/agent.html#update-acl-tokens).

#### ACL Agent Master Token

Since the [`acl.tokens.agent_master`](/docs/agent/options.html#acl_tokens_agent_master) is designed to be used when the Consul servers are not available, its policy is managed locally on the agent and does not need to have a token defined on the Consul servers via the ACL API. Once set, it implicitly has the following policy associated with it

```text
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

```text
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

The `service_prefix` policy needs `read` access for any services that can be registered on the agent. If [remote exec is disabled](/docs/agent/options.html#disable_remote_exec), the default, then the `key_prefix` policy can be omitted.

## Bootstrapping ACLs

Bootstrapping ACLs on a new cluster requires a few steps, outlined in the examples in this
section.

#### Enable ACLs on the Consul Servers

The first step for bootstrapping ACLs is to enable ACLs on the Consul servers in the primary
datacenter. In this example, we are configuring the following:

1. A primary datacenter of "dc1", which is where these servers are.
2. An ACL master token of "b1gs33cr3t"; see below for an alternative using the [/v1/acl/bootstrap API](/api/acl.html#bootstrap-acls)
3. A default policy of "deny" which means we are in whitelist mode
4. A down policy of "extend-cache" which means that we will ignore token TTLs during an
   outage

Here's the corresponding JSON configuration file:

```json
{
  "primary_datacenter": "dc1",
  "acl" : {
    "enabled": true,
    "default_policy": "deny",
    "down_policy": "extend-cache",
    "tokens" : {
      "master" : "b1gs33cr3t"
    }
  }
}
```

The servers will need to be restarted to load the new configuration. Please take care
to start the servers one at a time, and ensure each server has joined and is operating
correctly before starting another.

The [`acl.tokens.master`](/docs/agent/options.html#acl_tokens_master) will be created
and assigned the `global-management` policy.
[`acl.tokens.master`](/docs/agent/options.html#acl_tokens_master) is only installed when
a server acquires cluster leadership. If you would like to install or change the
[`acl.tokens.master`](/docs/agent/options.html#acl_tokens_master), set the new value for
[`acl.tokens.master`](/docs/agent/options.html#acl_tokens_master) in the configuration
for all servers. Once this is done, restart the current leader to force a leader election.

In Consul 0.9.1 and later, you can use the [/v1/acl/bootstrap API](/api/acl.html#bootstrap-acls)
to make the initial master token, so a token never needs to be placed into a configuration
file. To use this approach, omit `acl.tokens.master` from the above config and then call the API:

```bash
$ consul acl bootstrap
AccessorID:   1ee820ce-e149-829f-caba-77ec37be3c98
SecretID:     be13b885-ddd4-830a-857c-d5fec72bbe8b
Description:  Bootstrap Token (Global Management)
Local:        false
Create Time:  2018-10-19 11:48:25.614214 -0400 EDT
Policies:
   00000000-0000-0000-0000-000000000001 - global-management
```

It's only possible to bootstrap one time, and bootstrapping will be disabled if a master
token was configured and created.

Once the ACL system is bootstrapped, ACL tokens can be managed through the
[ACL API](/api/acl.html).

#### Create an Agent Token

After the servers are restarted above, you will see new errors in the logs of the Consul
servers related to permission denied errors:

```
2017/07/08 23:38:24 [WARN] agent: Node info update blocked by ACLs
2017/07/08 23:38:44 [WARN] agent: Coordinate update blocked by ACLs
```

These errors are because the agent doesn't yet have a properly configured
[`acl.tokens.agent`](/docs/agent/options.html#acl_tokens_agent) that it can use for its
own internal operations like updating its node information in the catalog and performing
[anti-entropy](/docs/internals/anti-entropy.html) syncing. We can create a token using the
ACL API, and the ACL master token we set in the previous step:

The first step is to create a policy for your agent tokens.

```bash
# Assumes agent-policy.hcl contains the following:
# node_prefix "" {
#    policy = "write"
# }
# service_prefix "" {
#    policy = "read"
# }
#
$ consul acl policy create  -name "agent-token" -description "Agent Token Policy" -rules @agent-policy.hcl

ID:           5102b76c-6058-9fe7-82a4-315c353eb7f7
Name:         agent-policy
Description:  Agent Token Policy
Datacenters:
Rules:
node_prefix "" {
   policy = "write"
}

service_prefix "" {
   policy = "read"
}
```

The returned value is the newly-created policy. We can now create tokens and assign it this policy.

```bash
$ consul acl token create -description "Agent Token" -policy-name "agent-token"

AccessorID:   499ab022-27f2-acb8-4e05-5a01fff3b1d1
SecretID:     da666809-98ca-0e94-a99c-893c4bf5f9eb
Description:  Agent Token
Local:        false
Create Time:  2018-10-19 14:23:40.816899 -0400 EDT
Policies:
   fcd68580-c566-2bd2-891f-336eadc02357 - agent-token

```

We can now assign the SecretID of this token in our Consul server
configuration and restart the servers once more to apply it:

```json
{
  "primary_datacenter": "dc1",
  "acl" : {
    "enabled" : true,
    "default_policy" : "deny",
    "down_policy" : "extend-cache",
    "tokens" : {
      "master" : "b1gs33cr3t",
      "agent" : "da666809-98ca-0e94-a99c-893c4bf5f9eb"
    }
  }
}
```

In Consul 0.9.1 and later you can also introduce the agent token using an API,
so it doesn't need to be set in the configuration file:

```bash
$ consul acl set-agent-token agent da666809-98ca-0e94-a99c-893c4bf5f9eb

ACL token "agent" set successfully
```

With that ACL agent token set, the servers will be able to sync themselves with the
catalog:

```
2017/07/08 23:42:59 [INFO] agent: Synced node info
```

See the [ACL Agent Token](#acl-agent-token) section for more details.

#### Enable ACLs on the Consul Clients

Since ACL enforcement also occurs on the Consul clients, we need to also restart them
with a configuration file that enables ACLs:

```json
{
  "primary_datacenter": "dc1",
  "acl" : {
    "enabled" : true,
    "default_policy" : "deny",
    "down_policy" : "extend-cache",
    "tokens" : {
      "agent" : "fcd68580-c566-2bd2-891f-336eadc02357"
    }
  }
}
```

Similar to the previous example, in Consul 0.9.1 and later you can also introduce the
agent token using an API, so it doesn't need to be set in the configuration file:

```bash
$ consul acl set-agent-token agent "fcd68580-c566-2bd2-891f-336eadc02357"

ACL token "agent" set successfully
```

We used the same ACL agent token that we created for the servers, which will work since
it was not specific to any node or set of service prefixes. In a more locked-down
environment it is recommended that each client get an ACL agent token with `node` write
privileges for just its own node name, and `service` read privileges for just the
service prefixes expected to be registered on that client.

[Anti-entropy](/docs/internals/anti-entropy.html) syncing requires the ACL agent token
to have `service` read privileges for all services that may be registered with the agent,
so generally an empty `service` prefix can be used, as shown in the example.

Clients will report similar permission denied errors until they are restarted with an ACL
agent token.

#### Configure the Anonymous Token (Optional)

At this point ACLs are bootstrapped with ACL agent tokens configured, but there are no
other policies set up. Even basic operations like `consul members` will be restricted
by the ACL default policy of "deny":

```
$ consul members
```

We don't get an error since the ACL has filtered what we see, and we aren't allowed to
see any nodes by default.

If we supply the token we created above we will be able to see a listing of nodes because
it has write privileges to an empty `node` prefix, meaning it has access to all nodes:

```bash
$ CONSUL_HTTP_TOKEN=fcd68580-c566-2bd2-891f-336eadc02357 consul members
Node    Address         Status  Type    Build     Protocol  DC
node-1  127.0.0.1:8301  alive   server  0.9.0dev  2         dc1
node-2  127.0.0.2:8301  alive   client  0.9.0dev  2         dc1
```

It's pretty common in many environments to allow listing of all nodes, even without a
token. The policies associated with the special anonymous token can be updated to
configure Consul's behavior when no token is supplied. The anonymous token is managed
like any other ACL token, except that `anonymous` is used for the ID. In this example
we will give the anonymous token read privileges for all nodes:

```bash
$ consul acl policy create -name 'list-all-nodes' -rules 'node_prefix "" { policy = "read" }'

ID:           e96d0a33-28b4-d0dd-9b3f-08301700ac72
Name:         list-all-nodes
Description:
Datacenters:
Rules:
node_prefix "" { policy = "read" }

$ consul acl token update -id 00000000-0000-0000-0000-000000000002 -policy-name list-all-nodes -description "Anonymous Token - Can List Nodes"

Token updated successfully.
AccessorID:   00000000-0000-0000-0000-000000000002
SecretID:     anonymous
Description:  Anonymous Token - Can List Nodes
Local:        false
Create Time:  0001-01-01 00:00:00 +0000 UTC
Hash:         ee4638968d9061647ac8c3c99e9d37bfdd2af4d1eaa07a7b5f80af0389460948
Create Index: 5
Modify Index: 38
Policies:
   e96d0a33-28b4-d0dd-9b3f-08301700ac72 - list-all-nodes

```

The anonymous token is implicitly used if no token is supplied, so now we can run
`consul members` without supplying a token and we will be able to see the nodes:

```bash
$ consul members
Node    Address         Status  Type    Build     Protocol  DC
node-1  127.0.0.1:8301  alive   server  0.9.0dev  2         dc1
node-2  127.0.0.2:8301  alive   client  0.9.0dev  2         dc1
```

The anonymous token is also used for DNS lookups since there's no way to pass a
token as part of a DNS request. Here's an example lookup for the "consul" service:

```
$ dig @127.0.0.1 -p 8600 consul.service.consul

; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 consul.service.consul
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 9648
;; flags: qr aa rd; QUERY: 1, ANSWER: 0, AUTHORITY: 1, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;consul.service.consul.         IN      A

;; AUTHORITY SECTION:
consul.                 0       IN      SOA     ns.consul. postmaster.consul. 1499584110 3600 600 86400 0

;; Query time: 2 msec
;; SERVER: 127.0.0.1#8600(127.0.0.1)
;; WHEN: Sun Jul  9 00:08:30 2017
;; MSG SIZE  rcvd: 89
```

Now we get an `NXDOMAIN` error because the anonymous token doesn't have access to the
"consul" service. Let's add that to the anonymous token's policy:

```bash
$ consul acl policy create -name 'service-consul-read' -rules 'service "consul" { policy = "read" }'
ID:           3c93f536-5748-2163-bb66-088d517273ba
Name:         service-consul-read
Description:
Datacenters:
Rules:
service "consul" { policy = "read" }

$ consul acl token update -id 00000000-0000-0000-0000-000000000002 --merge-policies -description "Anonymous Token - Can List Nodes" -policy-name service-consul-read
Token updated successfully.
AccessorID:   00000000-0000-0000-0000-000000000002
SecretID:     anonymous
Description:  Anonymous Token - Can List Nodes
Local:        false
Create Time:  0001-01-01 00:00:00 +0000 UTC
Hash:         2c641c4f73158ef6d62f6467c68d751fccd4db9df99b235373e25934f9bbd939
Create Index: 5
Modify Index: 43
Policies:
   e96d0a33-28b4-d0dd-9b3f-08301700ac72 - list-all-nodes
   3c93f536-5748-2163-bb66-088d517273ba - service-consul-read
```

With that new policy in place, the DNS lookup will succeed:

```
$ dig @127.0.0.1 -p 8600 consul.service.consul

; <<>> DiG 9.8.3-P1 <<>> @127.0.0.1 -p 8600 consul.service.consul
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 46006
;; flags: qr aa rd; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0
;; WARNING: recursion requested but not available

;; QUESTION SECTION:
;consul.service.consul.         IN      A

;; ANSWER SECTION:
consul.service.consul.  0       IN      A       127.0.0.1

;; Query time: 0 msec
;; SERVER: 127.0.0.1#8600(127.0.0.1)
;; WHEN: Sun Jul  9 00:11:14 2017
;; MSG SIZE  rcvd: 55
```

The next section shows an alternative to the anonymous token.

#### Set Agent-Specific Default Tokens (Optional)

An alternative to the anonymous token is the [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default)
configuration item. When a request is made to a particular Consul agent and no token is
supplied, the [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default) will be used for the token,
instead of being left empty which would normally invoke the anonymous token.

In Consul 0.9.1 and later, the agent ACL tokens can be introduced or updated via the
[/v1/agent/token API](/api/agent.html#update-acl-tokens).

This behaves very similarly to the anonymous token, but can be configured differently on each
agent, if desired. For example, this allows more fine grained control of what DNS requests a
given agent can service, or can give the agent read access to some key-value store prefixes by
default.

If using [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default), then it's likely the anonymous
token will have a more restrictive policy than shown in the examples here.

#### Create Tokens for UI Use (Optional)

If you utilize the Consul UI with a restrictive ACL policy, as above, the UI will
not function fully using the anonymous ACL token. It is recommended
that a UI-specific ACL token is used, which can be set in the UI during the
web browser session to authenticate the interface.

```bash
$ consul acl policy create -name "ui-policy" \
                           -description "Necessary permissions for UI functionality" \
                           -rules 'key "" { policy = "write" } node "" { policy = "read" } service "" { policy = "read" }'

ID:           9cb99b2b-3c20-81d4-a7c0-9ffdc2fbf08a
Name:         ui-policy
Description:  Necessary permissions for UI functionality
Datacenters:
Rules:
key "" { policy = "write" } node "" { policy = "read" } service "" { policy = "read" }

$ consul acl token create -description "UI Token" -policy-name "ui-policy"

AccessorID:   56e605cf-a6f9-5f9d-5c08-a0e1323cf016
SecretID:     117842b6-6208-446a-0d1e-daf93854857d
Description:  UI Token
Local:        false
Create Time:  2018-10-19 14:55:44.254063 -0400 EDT
Policies:
   9cb99b2b-3c20-81d4-a7c0-9ffdc2fbf08a - ui-policy

```

The token can then be set on the "settings" page of the UI.

#### Next Steps

The examples above configure a basic ACL environment with the ability to see all nodes
by default, and limited access to just the "consul" service. The [ACL API](/api/acl.html)
can be used to create tokens for applications specific to their intended use, and to create
more specific ACL agent tokens for each agent's expected role.

## Rule Specification

A core part of the ACL system is the rule language which is used to describe the policy
that must be enforced. There are two types of rules: prefix based rules and exact matching
rules. The rules is composed of a resource, a segment (for some resource areas) and a policy
disposition. The general structure of a rule is:

```text
<resource> "<segment>" {
  policy = "<policy disposition>"
}
```

Segmented resource areas allow operators to more finely control access to those resources.
Note that not all resource areas are segmented such as the `keyring`, `operator` and `acl` resources. For those rules they would look like:

```text
<resource> = "<policy disposition>"
```

Policies can have several dispositions:

* `read`: allow the resource to be read but not modified
* `write`: allow the resource to be read and modified
* `deny`: do not allow the resource to be read or modified

When using prefix-based rules, the most specific prefix match determines the action. This
allows for flexible rules like an empty prefix to allow read-only access to all
resources, along with some specific prefixes that allow write access or that are
denied all access. Exact matching rules will only apply to the exact resource specified.

We make use of the
[HashiCorp Configuration Language (HCL)](https://github.com/hashicorp/hcl/) to specify
rules. This language is human readable and interoperable with JSON making it easy to
machine-generate. Rules can make use of one or more policies.

Specification in the HCL format looks like:

```text
# These control access to the key/value store.
key_prefix "" {
  policy = "read"
}
key_prefix "foo/" {
  policy = "write"
}
key_prefix "foo/private/" {
  policy = "deny"
}

key "foo/bar/secret" {
  policy = "deny"
}

# This controls access to cluster-wide Consul operator information.
operator = "read"
```

This is equivalent to the following JSON input:

```javascript
{
  "key_prefix": {
    "": {
      "policy": "read"
    },
    "foo/": {
      "policy": "write"
    },
    "foo/private/": {
      "policy": "deny"
    }
  },
  "key" : {
    "foo/bar/secret" : {
      "policy" : "deny"
    }
  }
  "operator": "read"
}
```

The [ACL API](/api/acl.html) allows either HCL or JSON to be used to define the content
of the rules section of a policy.

Here's a sample request using the HCL form:

```text
$ curl \
    --request PUT \
    --data \
'{
  "Name": "my-app-policy",
  "Rules": "key \"\" { policy = \"read\" } key \"foo/\" { policy = \"write\" } key \"foo/private/\" { policy = \"deny\" } operator = \"read\""
}' http://127.0.0.1:8500/v1/acl/policy?token=<token with ACL "write">
```

Here's an equivalent request using the JSON form:

```text
$ curl \
    --request PUT \
    --data \
'{
  "Name": "my-app-policy",
  "Rules": "{\"key\":{\"\":{\"policy\":\"read\"},\"foo/\":{\"policy\":\"write\"},\"foo/private\":{\"policy\":\"deny\"}},\"operator\":\"read\"}"
}' http://127.0.0.1:8500/v1/acl/policy?token=<management token>
```

On success, the Policy is returned:

```json
{
    "CreateIndex": 7,
    "Hash": "UMG6QEbV40Gs7Cgi6l/ZjYWUwRS0pIxxusFKyKOt8qI=",
    "ID": "5f423562-aca1-53c3-e121-cb0eb2ea1cd3",
    "ModifyIndex": 7,
    "Name": "my-app-policy",
    "Rules": "key \"\" { policy = \"read\" } key \"foo/\" { policy = \"write\" } key \"foo/private/\" { policy = \"deny\" } operator = \"read\""
}
```

This token ID can then be passed into Consul's HTTP APIs via the `token`
query string parameter, or the `X-Consul-Token` request header, or Authorization
Bearer token header, or Consul's CLI commands via the `token` argument,
or the `CONSUL_HTTP_TOKEN` environment variable.

#### ACL Rules

The `acl` resource controls access to ACL operations in the
[ACL API](/api/acl.html).

ACL rules look like this:

```text
acl = "write"
```

There is only one acl rule allowed per policy and its value is set to one of the policy dispositions. In the example
above ACLs may be read or written including discovering any token's secret ID. Snapshotting also requires `acl = "write"`
permissions due to the fact that all the token secrets are contained within the snapshot.

#### Agent Rules

The `agent` and `agent_prefix` resources control access to the utility operations in the [Agent API](/api/agent.html),
such as join and leave. All of the catalog-related operations are covered by the [`node` or `node_prefix`](#node-rules)
and [`service` or `service_prefix`](#service-rules) policies instead.

Agent rules look like this:

```text
agent "" {
  policy = "read"
}
agent "foo" {
  policy = "write"
}
agent "bar" {
  policy = "deny"
}

agent_prefix"" {
  policy = "read"
}
```

Agent rules are keyed by the node name they apply to, using the longest prefix match rule. In
the example above, the rules allow read-only access to any node name with the empty prefix, allow
read-write access to any node name that starts with "foo", and deny all access to any node name that
starts with "bar".

Since [Agent API](/api/agent.html) utility operations may be reqired before an agent is joined to
a cluster, or during an outage of the Consul servers or ACL datacenter, a special token may be
configured with [`acl_agent_master_token`](/docs/agent/options.html#acl_agent_master_token) to allow
write access to these operations even if no ACL resolution capability is available.

#### Event Rules

The `event` and `event_prefix` resources control access to event operations in the [Event API](/api/event.html), such as
firing events and listing events.

Event rules look like this:

```text
event_prefix "" {
  policy = "read"
}
event "deploy" {
  policy = "write"
}
```

Event rules are segmented by the event name they apply to. In the example above, the rules allow
read-only access to any event, and firing of the "deploy" event.

The [`consul exec`](/docs/commands/exec.html) command uses events with the "_rexec" prefix during
operation, so to enable this feature in a Consul environment with ACLs enabled, you will need to
give agents a token with access to this event prefix, in addition to configuring
[`disable_remote_exec`](/docs/agent/options.html#disable_remote_exec) to `false`.

#### Key/Value Rules

The `key` and `key_prefix` resources control access to key/value store operations in the [KV API](/api/kv.html). Key
rules look like this:

```text
key_prefix "" {
  policy = "read"
}
key "foo" {
  policy = "write"
}
key "bar" {
  policy = "deny"
}
```

Key rules are segmented by the key name they apply to. In the example above, the rules allow read-only access
to any key name with the empty prefix rule, allow read-write access to the "foo" key, and deny access to the "bar" key.

#### List Policy for Keys

Consul 1.0 introduces a new `list` policy for keys that is only enforced when opted in via the boolean config param "acl.enable_key_list_policy".
`list` controls access to recursively list entries and keys, and enables more fine grained policies. With "acl.enable_key_list_policy",
recursive reads via [the KV API](/api/kv.html#recurse) with an invalid token result in a 403. Example:

```text
key_prefix "" {
 policy = "deny"
}

key_prefix "bar" {
 policy = "list"
}

key_prefix "baz" {
 policy = "read"
}
```

In the example above, the rules allow reading the key "baz", and only allow recursive reads on the prefix "bar".

A token with `write` access on a prefix also has `list` access. A token with `list` access on a prefix also has `read` access on all its suffixes.

#### Sentinel Integration

Consul Enterprise supports additional optional fields for key write policies for
[Sentinel](https://docs.hashicorp.com/sentinel/app/consul/) integration. An example key rule with a
Sentinel code policy looks like this:

```text
key "foo" {
  policy = "write"
  sentinel {
      code = <<EOF
        import "strings\
        main = rule { strings.has_suffix(value, "bar") }
EOF
      enforcementlevel = "hard-mandatory"
  }
}
```

For more detailed documentation, see the [Consul Sentinel Guide](/docs/guides/sentinel.html).

#### Keyring Rules

The `keyring` resource controls access to keyring operations in the
[Keyring API](/api/operator/keyring.html).

Keyring rules look like this:

```text
keyring = "write"
```

There's only one keyring policy allowed per rule set, and its value is set to one of the policy
dispositions. In the example above, the keyring may be read and updated.

#### Node Rules

The `node` and `node_prefix` resources controls node-level registration and read access to the [Catalog API](/api/catalog.html),
service discovery with the [Health API](/api/health.html), and filters results in [Agent API](/api/agent.html)
operations like fetching the list of cluster members.

Node rules look like this:

```text
node_prefix "" {
  policy = "read"
}
node "app" {
  policy = "write"
}
node "admin" {
  policy = "deny"
}
```

Node rules are segmented by the node name they apply to. In the example above, the rules allow read-only access to any node name with the empty prefix, allow
read-write access to the "app" node, and deny all access to the "admin" node.

Agents need to be configured with an [`acl.tokens.agent`](/docs/agent/options.html#acl_tokens_agent)
with at least "write" privileges to their own node name in order to register their information with
the catalog, such as node metadata and tagged addresses. If this is configured incorrectly, the agent
will print an error to the console when it tries to sync its state with the catalog.

Consul's DNS interface is also affected by restrictions on node rules. If the
[`acl.token.default`](/docs/agent/options.html#acl_tokens_default) used by the agent does not have "read" access to a
given node, then the DNS interface will return no records when queried for it.

When reading from the catalog or retrieving information from the health endpoints, node rules are
used to filter the results of the query. This allows for configurations where a token has access
to a given service name, but only on an allowed subset of node names.

Node rules come into play when using the [Agent API](/api/agent.html) to register node-level
checks. The agent will check tokens locally as a check is registered, and Consul also performs
periodic [anti-entropy](/docs/internals/anti-entropy.html) syncs, which may require an
ACL token to complete. To accommodate this, Consul provides two methods of configuring ACL tokens
to use for registration events:

1. Using the [acl.tokens.default](/docs/agent/options.html#acl_tokens_default) configuration
   directive. This allows a single token to be configured globally and used
   during all check registration operations.
2. Providing an ACL token with service and check definitions at
   registration time. This allows for greater flexibility and enables the use
   of multiple tokens on the same agent. Examples of what this looks like are
   available for both [services](/docs/agent/services.html) and
   [checks](/docs/agent/checks.html). Tokens may also be passed to the
   [HTTP API](/api/index.html) for operations that require them.

In addition to ACLs, in Consul 0.9.0 and later, the agent must be configured with
[`enable_script_checks`](/docs/agent/options.html#_enable_script_checks) set to `true` in order to enable
script checks.

#### Operator Rules

The `operator` resource controls access to cluster-level operations in the
[Operator API](/api/operator.html), other than the [Keyring API](/api/operator/keyring.html).

Operator rules look like this:

```text
operator = "read"
```

There's only one operator rule allowed per rule set, and its value is set to one of the policy
dispositions. In the example above, the token could be used to query the operator endpoints for
diagnostic purposes but not make any changes.

#### Prepared Query Rules

The `query` and `query_prefix` resources control access to create, update, and delete prepared queries in the
[Prepared Query API](/api/query.html). Executing queries is subject to `node`/`node_prefix` and `service`/`service_prefix`
policies, as will be explained below.

Query rules look like this:

```text
query_prefix "" {
  policy = "read"
}
query "foo" {
  policy = "write"
}
```

Query rules are segmented by the query name they apply to. In the example above, the rules allow read-only
access to any query name with the empty prefix, and allow read-write access to the query named "foo".
This allows control of the query namespace to be delegated based on ACLs.

There are a few variations when using ACLs with prepared queries, each of which uses ACLs in one of two
ways: open, protected by unguessable IDs or closed, managed by ACL policies. These variations are covered
here, with examples:

* Static queries with no `Name` defined are not controlled by any ACL policies.
  These types of queries are meant to be ephemeral and not shared to untrusted
  clients, and they are only reachable if the prepared query ID is known. Since
  these IDs are generated using the same random ID scheme as ACL Tokens, it is
  infeasible to guess them. When listing all prepared queries, only a management
  token will be able to see these types, though clients can read instances for
  which they have an ID. An example use for this type is a query built by a
  startup script, tied to a session, and written to a configuration file for a
  process to use via DNS.

* Static queries with a `Name` defined are controlled by the `query` and `query_prefix`
  ACL resources. Clients are required to have an ACL token with permissions on to
  access that query name. Clients can list or read queries for
  which they have "read" access based on their prefix, and similar they can
  update any queries for which they have "write" access. An example use for
  this type is a query with a well-known name (eg. `prod-master-customer-db`)
  that is used and known by many clients to provide geo-failover behavior for
  a database.

* [Template queries](/api/query.html#templates)
  queries work like static queries with a `Name` defined, except that a catch-all
  template with an empty `Name` requires an ACL token that can write to any query
  prefix.

When prepared queries are executed via DNS lookups or HTTP requests, the ACL
checks are run against the service being queried, similar to how ACLs work with
other service lookups. There are several ways the ACL token is selected for this
check:

* If an ACL Token was captured when the prepared query was defined, it will be
  used to perform the service lookup. This allows queries to be executed by
  clients with lesser or even no ACL Token, so this should be used with care.

* If no ACL Token was captured, then the client's ACL Token will be used to
  perform the service lookup.

* If no ACL Token was captured and the client has no ACL Token, then the
  anonymous token will be used to perform the service lookup.

In the common case, the ACL Token of the invoker is used
to test the ability to look up a service. If a `Token` was specified when the
prepared query was created, the behavior changes and now the captured
ACL Token set by the definer of the query is used when looking up a service.

Capturing ACL Tokens is analogous to
[PostgreSQL’s](http://www.postgresql.org/docs/current/static/sql-createfunction.html)
`SECURITY DEFINER` attribute which can be set on functions, and using the client's ACL
Token is similar to the complementary `SECURITY INVOKER` attribute.

Prepared queries were originally introduced in Consul 0.6.0, and ACL behavior remained
unchanged through version 0.6.3, but was then changed to allow better management of the
prepared query namespace.

These differences are outlined in the table below:

<table class="table table-bordered table-striped">
  <tr>
    <th>Operation</th>
    <th>Version <= 0.6.3 </th>
    <th>Version > 0.6.3 </th>
  </tr>
  <tr>
    <td>Create static query without `Name`</td>
    <td>The ACL Token used to create the prepared query is checked to make sure it can access the service being queried. This token is captured as the `Token` to use when executing the prepared query.</td>
    <td>No ACL policies are used as long as no `Name` is defined. No `Token` is captured by default unless specifically supplied by the client when creating the query.</td>
  </tr>
  <tr>
    <td>Create static query with `Name`</td>
    <td>The ACL Token used to create the prepared query is checked to make sure it can access the service being queried. This token is captured as the `Token` to use when executing the prepared query.</td>
    <td>The client token's `query` ACL policy is used to determine if the client is allowed to register a query for the given `Name`. No `Token` is captured by default unless specifically supplied by the client when creating the query.</td>
  </tr>
  <tr>
    <td>Manage static query without `Name`</td>
    <td>The ACL Token used to create the query or a token with management privileges must be supplied in order to perform these operations.</td>
    <td>Any client with the ID of the query can perform these operations.</td>
  </tr>
  <tr>
    <td>Manage static query with a `Name`</td>
    <td>The ACL token used to create the query or a token with management privileges must be supplied in order to perform these operations.</td>
    <td>Similar to create, the client token's `query` ACL policy is used to determine if these operations are allowed.</td>
  </tr>
  <tr>
    <td>List queries</td>
    <td>A token with management privileges is required to list any queries.</td>
    <td>The client token's `query` ACL policy is used to determine which queries they can see. Only tokens with management privileges can see prepared queries without `Name`.</td>
  </tr>
  <tr>
    <td>Execute query</td>
    <td>Since a `Token` is always captured when a query is created, that is used to check access to the service being queried. Any token supplied by the client is ignored.</td>
    <td>The captured token, client's token, or anonymous token is used to filter the results, as described above.</td>
  </tr>
</table>

#### Service Rules

The `service` and `service_prefix` resources control service-level registration and read access to the [Catalog API](/api/catalog.html)
and service discovery with the [Health API](/api/health.html).

Service rules look like this:

```text
service_prefix "" {
  policy = "read"
}
service "app" {
  policy = "write"
}
service "admin" {
  policy = "deny"
}
```

Service rules are segmented by the service name they apply to. In the example above, the rules allow read-only
access to any service name with the empty prefix, allow read-write access to the "app" service, and deny all
access to the "admin" service.

Consul's DNS interface is affected by restrictions on service rules. If the
[`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default) used by the agent does not have "read" access to a
given service, then the DNS interface will return no records when queried for it.

When reading from the catalog or retrieving information from the health endpoints, service rules are
used to filter the results of the query.

Service rules come into play when using the [Agent API](/api/agent.html) to register services or
checks. The agent will check tokens locally as a service or check is registered, and Consul also
performs periodic [anti-entropy](/docs/internals/anti-entropy.html) syncs, which may require an
ACL token to complete. To accommodate this, Consul provides two methods of configuring ACL tokens
to use for registration events:

1. Using the [acl.tokens.default](/docs/agent/options.html#acl_tokens_default) configuration
   directive. This allows a single token to be configured globally and used
   during all service and check registration operations.
2. Providing an ACL token with service and check definitions at registration
   time. This allows for greater flexibility and enables the use of multiple
   tokens on the same agent. Examples of what this looks like are available for
   both [services](/docs/agent/services.html) and
   [checks](/docs/agent/checks.html). Tokens may also be passed to the [HTTP
   API](/api/index.html) for operations that require them. **Note:** all tokens
   passed to an agent are persisted on local disk to allow recovery from
   restarts. See [`-data-dir` flag
   documentation](/docs/agent/options.html#acl_token) for notes on securing
   access.

In addition to ACLs, in Consul 0.9.0 and later, the agent must be configured with
[`enable_script_checks`](/docs/agent/options.html#_enable_script_checks) or
[`enable_local_script_checks`](/docs/agent/options.html#_enable_local_script_checks)
set to `true` in order to enable script checks.


#### Session Rules

The `session` and `session_prefix` resources controls access to [Session API](/api/session.html) operations.

Session rules look like this:

```text
session_prefix "" {
  policy = "read"
}
session "app" {
  policy = "write"
}
session "admin" {
  policy = "deny"
}
```

Session rules are segmented by the node name they apply to. In the example above, the rules allow read-only
access to sessions on node name with the empty prefix, allow creating sessions on the node named "app",
and deny all access to any sessions on the "admin" node.

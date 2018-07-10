---
layout: "docs"
page_title: "ACL System"
sidebar_current: "docs-guides-acl"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL system is a Capability-based system that relies on tokens which can have fine grained rules applied to them. It is very similar to AWS IAM in many ways.
---

# ACL System

Consul provides an optional Access Control List (ACL) system which can be used to control
access to data and APIs. The ACL is
[Capability-based](https://en.wikipedia.org/wiki/Capability-based_security), relying
on tokens to which fine grained rules can be applied. It is very similar to
[AWS IAM](http://aws.amazon.com/iam/) in many ways.

## ACL System Overview

The ACL system is designed to be easy to use, fast to enforce, and flexible to new policies,
all while providing administrative insight.

#### ACL Tokens

The ACL system is based on tokens, which are managed by Consul operators via Consul's
[ACL API](/api/acl.html), or systems like
[HashiCorp's Vault](https://www.vaultproject.io/docs/secrets/consul/index.html).

Every token has an ID, name, type, and rule set. The ID is a randomly generated
UUID, making it infeasible to guess. The name is opaque to Consul and human readable.
The type is either "client" (meaning the token cannot modify ACL rules) or "management"
(meaning the token is allowed to perform all actions).

The token ID is passed along with each RPC request to the servers. Consul's
[HTTP endpoints](/api/index.html) can accept tokens via the `token`
query string parameter, or the `X-Consul-Token` request header. Consul's
[CLI commands](/docs/commands/index.html) can accept tokens via the
`token` argument, or the `CONSUL_HTTP_TOKEN` environment variable.

If no token is provided, the rules associated with a special, configurable anonymous
token are automatically applied. The anonymous token is managed using the
[ACL API](/api/acl.html) like any other ACL token, but using `anonymous` for the ID.

#### ACL Rules and Scope

Tokens are bound to a set of rules that control which Consul resources the token
has access to. Policies can be defined in either a whitelist or blacklist mode
depending on the configuration of
[`acl_default_policy`](/docs/agent/options.html#acl_default_policy). If the default
policy is to "deny" all actions, then token rules can be set to whitelist specific
actions. In the inverse, the "allow" all default behavior is a blacklist where rules
are used to prohibit actions. By default, Consul will allow all actions.

The following table summarizes the ACL policies that are available for constructing
rules:

| Policy                   | Scope |
| ------------------------ | ----- |
| [`agent`](#agent-rules)          | Utility operations in the [Agent API](/api/agent.html), other than service and check registration |
| [`event`](#event-rules)          | Listing and firing events in the [Event API](/api/event.html) |
| [`key`](#key-value-rules)        | Key/value store operations in the [KV Store API](/api/kv.html) |
| [`keyring`](#keyring-rules)      | Keyring operations in the [Keyring API](/api/operator/keyring.html) |
| [`node`](#node-rules)            | Node-level catalog operations in the [Catalog API](/api/catalog.html), [Health API](/api/health.html), [Prepared Query API](/api/query.html), [Network Coordinate API](/api/coordinate.html), and [Agent API](/api/agent.html) |
| [`operator`](#operator-rules)    | Cluster-level operations in the [Operator API](/api/operator.html), other than the [Keyring API](/api/operator/keyring.html) |
| [`query`](#prepared-query-rules) | Prepared query operations in the [Prepared Query API](/api/query.html)
| [`service`](#service-rules)      | Service-level catalog operations in the [Catalog API](/api/catalog.html), [Health API](/api/health.html), [Prepared Query API](/api/query.html), and [Agent API](/api/agent.html) |
| [`session`](#session-rules)      | Session operations in the [Session API](/api/session.html) |

Since Consul snapshots actually contain ACL tokens, the
[Snapshot API](/api/snapshot.html) requires a management token for snapshot operations
and does not use a special policy.

The following resources are not covered by ACL policies:

1. The [Status API](/api/status.html) is used by servers when bootstrapping and exposes
basic IP and port information about the servers, and does not allow modification
of any state.

2. The datacenter listing operation of the
[Catalog API](/api/catalog.html#list-datacenters) similarly exposes the names of known
Consul datacenters, and does not allow modification of any state.

Constructing rules from these policies is covered in detail in the
[Rule Specification](#rule-specification) section below.

#### ACL Datacenter

All nodes (clients and servers) must be configured with an
[`acl_datacenter`](/docs/agent/options.html#acl_datacenter) which enables ACL
enforcement but also specifies the authoritative datacenter. Consul relies on
[RPC forwarding](/docs/internals/architecture.html) to support multi-datacenter
configurations. However, because requests can be made across datacenter boundaries,
ACL tokens must be valid globally. To avoid consistency issues, a single datacenter
is considered authoritative and stores the canonical set of tokens.

When a request is made to an agent in a non-authoritative datacenter, it must be
resolved into the appropriate policy. This is done by reading the token from the
authoritative server and caching the result for a configurable
[`acl_ttl`](/docs/agent/options.html#acl_ttl). The implication of caching is that
the cache TTL is an upper bound on the staleness of policy that is enforced. It is
possible to set a zero TTL, but this has adverse performance impacts, as every
request requires refreshing the policy via an RPC call.

During an outage of the ACL datacenter, or loss of connectivity, the cache will be
used as long as the TTL is valid, or the cache may be extended if the
[`acl_down_policy`](/docs/agent/options.html#acl_down_policy) is set accordingly.
This configuration also allows the ACL system to fail open or closed.
[ACL replication](#replication) is also available to allow for the full set of ACL
tokens to be replicated for use during an outage.

## Configuring ACLs

ACLs are configured using several different configuration options. These are marked
as to whether they are set on servers, clients, or both.

| Configuration Option | Servers | Clients | Purpose |
| -------------------- | ------- | ------- | ------- |
| [`acl_datacenter`](/docs/agent/options.html#acl_datacenter) | `REQUIRED` | `REQUIRED` | Master control that enables ACLs by defining the authoritative Consul datacenter for ACLs |
| [`acl_default_policy`](/docs/agent/options.html#acl_default_policy) | `OPTIONAL` | `N/A` | Determines whitelist or blacklist mode |
| [`acl_down_policy`](/docs/agent/options.html#acl_down_policy) | `OPTIONAL` | `OPTIONAL` | Determines what to do when the ACL datacenter is offline |
| [`acl_ttl`](/docs/agent/options.html#acl_ttl) | `OPTIONAL` | `OPTIONAL` | Determines time-to-live for cached ACLs |

There are some additional configuration items related to [ACL replication](#replication) and
[Version 8 ACL support](#version_8_acls). These are discussed in those respective sections
below.

A number of special tokens can also be configured which allow for bootstrapping the ACL
system, or accessing Consul in special situations:

| Special Token | Servers | Clients | Purpose |
| ------------- | ------- | ------- | ------- |
| [`acl_agent_master_token`](/docs/agent/options.html#acl_agent_master_token) | `OPTIONAL` | `OPTIONAL` | Special token that can be used to access [Agent API](/api/agent.html) when the ACL datacenter isn't available, or servers are offline (for clients); used for setting up the cluster such as doing initial join operations, see the [ACL Agent Master Token](#acl-agent-master-token) section for more details |
| [`acl_agent_token`](/docs/agent/options.html#acl_agent_token) | `OPTIONAL` | `OPTIONAL` | Special token that is used for an agent's internal operations, see the [ACL Agent Token](#acl-agent-token) section for more details |
| [`acl_master_token`](/docs/agent/options.html#acl_master_token) | `REQUIRED` | `N/A` | Special token used to bootstrap the ACL system, see the [Bootstrapping ACLs](#bootstrapping-acls) section for more details |
| [`acl_token`](/docs/agent/options.html#acl_token) | `OPTIONAL` | `OPTIONAL` | Default token to use for client requests where no token is supplied; this is often configured with read-only access to services to enable DNS service discovery on agents |

In Consul 0.9.1 and later, the agent ACL tokens can be introduced or updated via the
[/v1/agent/token API](/api/agent.html#update-acl-tokens).

#### ACL Agent Master Token

Since the [`acl_agent_master_token`](/docs/agent/options.html#acl_agent_master_token) is designed to be used when the Consul servers are not available, its policy is managed locally on the agent and does not need to have a token defined on the Consul servers via the ACL API. Once set, it implicitly has the following policy associated with it (the `node` policy was added in Consul 0.9.0):

```text
agent "<node name of agent>" {
  policy = "write"
}
node "" {
  policy = "read"
}
```

In Consul 0.9.1 and later, the agent ACL tokens can be introduced or updated via the
[/v1/agent/token API](/api/agent.html#update-acl-tokens).

#### ACL Agent Token

The [`acl_agent_token`](/docs/agent/options.html#acl_agent_token) is a special token that is used for an agent's internal operations. It isn't used directly for any user-initiated operations like the [`acl_token`](/docs/agent/options.html#acl_token), though if the `acl_agent_token` isn't configured the `acl_token` will be used. The ACL agent token is used for the following operations by the agent:

1. Updating the agent's node entry using the [Catalog API](/api/catalog.html), including updating its node metadata, tagged addresses, and network coordinates
2. Performing [anti-entropy](/docs/internals/anti-entropy.html) syncing, in particular reading the node metadata and services registered with the catalog
3. Reading and writing the special `_rexec` section of the KV store when executing [`consul exec`](/docs/commands/exec.html) commands

Here's an example policy sufficient to accomplish the above for a node called `mynode`:

```text
node "mynode" {
  policy = "write"
}
service "" {
  policy = "read"
}
key "_rexec" {
  policy = "write"
}
```

The `service` policy needs `read` access for any services that can be registered on the agent. If [remote exec is disabled](/docs/agent/options.html#disable_remote_exec), the default, then the `key` policy can be omitted.

In Consul 0.9.1 and later, the agent ACL tokens can be introduced or updated via the
[/v1/agent/token API](/api/agent.html#update-acl-tokens).

## Bootstrapping ACLs

Bootstrapping ACLs on a new cluster requires a few steps, outlined in the examples in this
section.

#### Enable ACLs on the Consul Servers

The first step for bootstrapping ACLs is to enable ACLs on the Consul servers in the ACL
datacenter. In this example, we are configuring the following:

1. An ACL datacenter of "dc1", which is where these servers are
2. An ACL master token of "b1gs33cr3t"; see below for an alternative using the [/v1/acl/bootstrap API](/api/acl.html#bootstrap-acls)
3. A default policy of "deny" which means we are in whitelist mode
4. A down policy of "extend-cache" which means that we will ignore token TTLs during an
   outage

Here's the corresponding JSON configuration file:

```json
{
  "acl_datacenter": "dc1",
  "acl_master_token": "b1gs33cr3t",
  "acl_default_policy": "deny",
  "acl_down_policy": "extend-cache"
}
```

The servers will need to be restarted to load the new configuration. Please take care
to start the servers one at a time, and ensure each server has joined and is operating
correctly before starting another.

The [`acl_master_token`](/docs/agent/options.html#acl_master_token) will be created
as a "management" type token automatically. The
[`acl_master_token`](/docs/agent/options.html#acl_master_token) is only installed when
a server acquires cluster leadership. If you would like to install or change the
[`acl_master_token`](/docs/agent/options.html#acl_master_token), set the new value for
[`acl_master_token`](/docs/agent/options.html#acl_master_token) in the configuration
for all servers. Once this is done, restart the current leader to force a leader election.

In Consul 0.9.1 and later, you can use the [/v1/acl/bootstrap API](/api/acl.html#bootstrap-acls)
to make the initial master token, so a token never needs to be placed into a configuration
file. To use this approach, omit `acl_master_token` from the above config and then call the API:

```text
$ curl \
    --request PUT \
    http://127.0.0.1:8500/v1/acl/bootstrap

{"ID":"fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"}
```

The returned token is the initial management token, which is randomly generated by Consul.
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
[`acl_agent_token`](/docs/agent/options.html#acl_agent_token) that it can use for its
own internal operations like updating its node information in the catalog and performing
[anti-entropy](/docs/internals/anti-entropy.html) syncing. We can create a token using the
ACL API, and the ACL master token we set in the previous step:

```text
$ curl \
    --request PUT \
    --header "X-Consul-Token: b1gs33cr3t" \
    --data \
'{
  "Name": "Agent Token",
  "Type": "client",
  "Rules": "node \"\" { policy = \"write\" } service \"\" { policy = \"read\" }"
}' http://127.0.0.1:8500/v1/acl/create

{"ID":"fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"}
```

The returned value is the newly-created token. We can now add this to our Consul server
configuration and restart the servers once more to apply it:

```json
{
  "acl_datacenter": "dc1",
  "acl_master_token": "b1gs33cr3t",
  "acl_default_policy": "deny",
  "acl_down_policy": "extend-cache",
  "acl_agent_token": "fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"
}
```

In Consul 0.9.1 and later you can also introduce the agent token using an API,
so it doesn't need to be set in the configuration file:

```text
$ curl \
    --request PUT \
    --header "X-Consul-Token: b1gs33cr3t" \
    --data \
'{
  "Token": "fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"
}' http://127.0.0.1:8500/v1/agent/token/acl_agent_token
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
  "acl_datacenter": "dc1",
  "acl_down_policy": "extend-cache",
  "acl_agent_token": "fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"
}
```

Similar to the previous example, in Consul 0.9.1 and later you can also introduce the
agent token using an API, so it doesn't need to be set in the configuration file:

```text
$ curl \
    --request PUT \
    --header "X-Consul-Token: b1gs33cr3t" \
    --data \
'{
  "Token": "fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1"
}' http://127.0.0.1:8500/v1/agent/token/acl_agent_token
```

We used the same ACL agent token that we created for the servers, which will work since
it was not specific to any node or set of service prefixes. In a more locked-down
environment it is recommended that each client get an ACL agent token with `node` write
privileges for just its own node name prefix, and `service` read privileges for just the
service prefixes expected to be registered on that client.

[Anti-entropy](/docs/internals/anti-entropy.html) syncing requires the ACL agent token
to have `service` read privileges for all services that may be registered with the agent,
so generally an empty `service` prefix can be used, as shown in the example.

Clients will report similar permission denied errors until they are restarted with an ACL
agent token.

#### Set an Anonymous Policy (Optional)

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

```
$ CONSUL_HTTP_TOKEN=fe3b8d40-0ee0-8783-6cc2-ab1aa9bb16c1 consul members
Node    Address         Status  Type    Build     Protocol  DC
node-1  127.0.0.1:8301  alive   server  0.9.0dev  2         dc1
node-2  127.0.0.2:8301  alive   client  0.9.0dev  2         dc1
```

It's pretty common in many environments to allow listing of all nodes, even without a
token. The policies associated with the special anonymous token can be updated to
configure Consul's behavior when no token is supplied. The anonymous token is managed
like any other ACL token, except that `anonymous` is used for the ID. In this example
we will give the anonymous token read privileges for all nodes:

```text
$ curl \
    --request PUT \
    --header "X-Consul-Token: b1gs33cr3t" \
    --data \
'{
  "ID": "anonymous",
  "Type": "client",
  "Rules": "node \"\" { policy = \"read\" }"
}' http://127.0.0.1:8500/v1/acl/update

{"ID":"anonymous"}
```

The anonymous token is implicitly used if no token is supplied, so now we can run
`consul members` without supplying a token and we will be able to see the nodes:

```
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

```text
$ curl \
    --request PUT \
    --header "X-Consul-Token: b1gs33cr3t" \
    --data \
'{
  "ID": "anonymous",
  "Type": "client",
  "Rules": "node \"\" { policy = \"read\" } service \"consul\" { policy = \"read\" }"
}' http://127.0.0.1:8500/v1/acl/update

{"ID":"anonymous"}
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

An alternative to the anonymous token is the [`acl_token`](/docs/agent/options.html#acl_token)
configuration item. When a request is made to a particular Consul agent and no token is
supplied, the [`acl_token`](/docs/agent/options.html#acl_token) will be used for the token,
instead of being left empty which would normally invoke the anonymous token.

In Consul 0.9.1 and later, the agent ACL tokens can be introduced or updated via the
[/v1/agent/token API](/api/agent.html#update-acl-tokens).

This behaves very similarly to the anonymous token, but can be configured differently on each
agent, if desired. For example, this allows more fine grained control of what DNS requests a
given agent can service, or can give the agent read access to some key-value store prefixes by
default.

If using [`acl_token`](/docs/agent/options.html#acl_token), then it's likely the anonymous
token will have a more restrictive policy than shown in the examples here.

#### Create Tokens for UI Use (Optional)

If you utilize the Consul UI with a restrictive ACL policy, as above, the UI will
not function fully using the anonymous ACL token. It is recommended
that a UI-specific ACL token is used, which can be set in the UI during the
web browser session to authenticate the interface.

```text
$ curl \
    --request PUT \
    --header "X-Consul-Token: b1gs33cr3t" \
    --data \
'{
  "Name": "UI Token",
  "Type": "client",
  "Rules": "key \"\" { policy = \"write\" } node \"\" { policy = \"read\" } service \"\" { policy = \"read\" }"
}' http://127.0.0.1:8500/v1/acl/create
{"ID":"d0a9f330-2f9d-0a8c-d2af-1e9ceda354e6"}
```

The token can then be set on the "settings" page of the UI.

#### Next Steps

The examples above configure a basic ACL environment with the ability to see all nodes
by default, and limited access to just the "consul" service. The [ACL API](/api/acl.html)
can be used to create tokens for applications specific to their intended use, and to create
more specific ACL agent tokens for each agent's expected role.

Also see [HashiCorp's Vault](https://www.vaultproject.io/docs/secrets/consul/index.html), which
has an integration with Consul that allows it to generate ACL tokens on the fly and to manage
their lifetimes.

## Rule Specification

A core part of the ACL system is the rule language which is used to describe the policy
that must be enforced. Most of the ACL rules are prefix-based, allowing operators to
define different namespaces within Consul's resource areas like the catalog and key/value
store, in order to delegate responsibility for these namespaces. Policies can have several
dispositions:

* `read`: allow the resource to be read but not modified
* `write`: allow the resource to be read and modified
* `deny`: do not allow the resource to be read or modified

With prefix-based rules, the most specific prefix match determines the action. This
allows for flexible rules like an empty prefix to allow read-only access to all
resources, along with some specific prefixes that allow write access or that are
denied all access.

We make use of the
[HashiCorp Configuration Language (HCL)](https://github.com/hashicorp/hcl/) to specify
rules. This language is human readable and interoperable with JSON making it easy to
machine-generate. Rules can make use of one or more policies.

Specification in the HCL format looks like:

```text
# These control access to the key/value store.
key "" {
  policy = "read"
}
key "foo/" {
  policy = "write"
}
key "foo/private/" {
  policy = "deny"
}

# This controls access to cluster-wide Consul operator information.
operator = "read"
```

This is equivalent to the following JSON input:

```javascript
{
  "key": {
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
  "operator": "read"
}
```

The [ACL API](/api/acl.html) allows either HCL or JSON to be used to define the content
of the rules section.

Here's a sample request using the HCL form:

```text
$ curl \
    --request PUT \
    --data \
'{
  "Name": "my-app-token",
  "Type": "client",
  "Rules": "key \"\" { policy = \"read\" } key \"foo/\" { policy = \"write\" } key \"foo/private/\" { policy = \"deny\" } operator = \"read\""
}' https://consul.rocks/v1/acl/create?token=<management token>
```

Here's an equivalent request using the JSON form:

```text
$ curl \
    --request PUT \
    --data \
'{
  "Name": "my-app-token",
  "Type": "client",
  "Rules": "{\"key\":{\"\":{\"policy\":\"read\"},\"foo/\":{\"policy\":\"write\"},\"foo/private\":{\"policy\":\"deny\"}},\"operator\":\"read\"}"
}' https://consul.rocks/v1/acl/create?token=<management token>
```

On success, the token ID is returned:

```json
{
  "ID": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

This token ID can then be passed into Consul's HTTP APIs via the `token`
query string parameter, or the `X-Consul-Token` request header, or Consul's
CLI commands via the `token` argument, or the `CONSUL_HTTP_TOKEN` environment
variable.

#### Agent Rules

The `agent` policy controls access to the utility operations in the [Agent API](/api/agent.html),
such as join and leave. All of the catalog-related operations are covered by the [`node`](#node-rules)
and [`service`](#service-rules) policies instead.

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
```

Agent rules are keyed by the node name prefix they apply to, using the longest prefix match rule. In
the example above, the rules allow read-only access to any node name with the empty prefix, allow
read-write access to any node name that starts with "foo", and deny all access to any node name that
starts with "bar".

Since [Agent API](/api/agent.html) utility operations may be required before an agent is joined to
a cluster, or during an outage of the Consul servers or ACL datacenter, a special token may be
configured with [`acl_agent_master_token`](/docs/agent/options.html#acl_agent_master_token) to allow
write access to these operations even if no ACL resolution capability is available.

#### Event Rules

The `event` policy controls access to event operations in the [Event API](/api/event.html), such as
firing events and listing events.

Event rules look like this:

```text
event "" {
  policy = "read"
}
event "deploy" {
  policy = "write"
}
```

Event rules are keyed by the event name prefix they apply to, using the longest prefix match rule.
In the example above, the rules allow read-only access to any event, and firing of any event that
starts with "deploy".

The [`consul exec`](/docs/commands/exec.html) command uses events with the "_rexec" prefix during
operation, so to enable this feature in a Consul environment with ACLs enabled, you will need to
give agents a token with access to this event prefix, in addition to configuring
[`disable_remote_exec`](/docs/agent/options.html#disable_remote_exec) to `false`.

#### Key/Value Rules

The `key` policy controls access to key/value store operations in the [KV API](/api/kv.html). Key
rules look like this:

```text
key "" {
  policy = "read"
}
key "foo" {
  policy = "write"
}
key "bar" {
  policy = "deny"
}
```

Key rules are keyed by the key name prefix they apply to, using the longest prefix match rule. In
the example above, the rules allow read-only access to any key name with the empty prefix, allow
read-write access to any key name that starts with "foo", and deny all access to any key name that
starts with "bar".

#### List Policy for Keys

Consul 1.0 introduces a new `list` policy for keys that is only enforced when opted in via the boolean config param "acl_enable_key_list_policy".
`list` controls access to recursively list entries and keys, and enables more fine grained policies. With "acl_enable_key_list_policy",
recursive reads via [the KV API](/api/kv.html#recurse) with an invalid token result in a 403. Example:

```text
key "" {
 policy = "deny"
}

key "bar" {
 policy = "list"
}

key "baz" {
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
      code = " import \"strings\"
               main = rule { strings.has_suffix(value, \"bar\") } "
      enforcementlevel = "hard-mandatory"
  }
}
```

For more detailed documentation, see the [Consul Sentinel Guide](/docs/guides/sentinel.html).

#### Keyring Rules

The `keyring` policy controls access to keyring operations in the
[Keyring API](/api/operator/keyring.html).

Keyring rules look like this:

```text
keyring = "write"
```

There's only one keyring policy allowed per rule set, and its value is set to one of the policy
dispositions. In the example above, the keyring may be read and updated.

#### Node Rules

The `node` policy controls node-level registration and read access to the [Catalog API](/api/catalog.html),
service discovery with the [Health API](/api/health.html), and filters results in [Agent API](/api/agent.html)
operations like fetching the list of cluster members.

Node rules look like this:

```text
node "" {
  policy = "read"
}
node "app" {
  policy = "write"
}
node "admin" {
  policy = "deny"
}
```

Node rules are keyed by the node name prefix they apply to, using the longest prefix match rule. In
the example above, the rules allow read-only access to any node name with the empty prefix, allow
read-write access to any node name that starts with "app", and deny all access to any node name that
starts with "admin".

Agents need to be configured with an [`acl_agent_token`](/docs/agent/options.html#acl_agent_token)
with at least "write" privileges to their own node name in order to register their information with
the catalog, such as node metadata and tagged addresses. If this is configured incorrectly, the agent
will print an error to the console when it tries to sync its state with the catalog.

Consul's DNS interface is also affected by restrictions on node rules. If the
[`acl_token`](/docs/agent/options.html#acl_token) used by the agent does not have "read" access to a
given node, then the DNS interface will return no records when queried for it.

When reading from the catalog or retrieving information from the health endpoints, node rules are
used to filter the results of the query. This allows for configurations where a token has access
to a given service name, but only on an allowed subset of node names.

Node rules come into play when using the [Agent API](/api/agent.html) to register node-level
checks. The agent will check tokens locally as a check is registered, and Consul also performs
periodic [anti-entropy](/docs/internals/anti-entropy.html) syncs, which may require an
ACL token to complete. To accommodate this, Consul provides two methods of configuring ACL tokens
to use for registration events:

1. Using the [acl_token](/docs/agent/options.html#acl_token) configuration
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

The `operator` policy controls access to cluster-level operations in the
[Operator API](/api/operator.html), other than the [Keyring API](/api/operator/keyring.html).

Operator rules look like this:

```text
operator = "read"
```

There's only one operator policy allowed per rule set, and its value is set to one of the policy
dispositions. In the example above, the token could be used to query the operator endpoints for
diagnostic purposes but not make any changes.

#### Prepared Query Rules

The `query` policy controls access to create, update, and delete prepared queries in the
[Prepared Query API](/api/query.html). Executing queries is subject to `node` and `service`
policies, as will be explained below.

Query rules look like this:

```text
query "" {
  policy = "read"
}
query "foo" {
  policy = "write"
}
```

Query rules are keyed by the query name prefix they apply to, using the longest prefix match rule. In
the example above, the rules allow read-only access to any query name with the empty prefix, and allow
read-write access to any query name that starts with "foo". This allows control of the query namespace
to be delegated based on ACLs.

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

* Static queries with a `Name` defined are controlled by the `query` ACL policy.
  Clients are required to have an ACL token with a prefix sufficient to cover
  the name they are trying to manage, with a longest prefix match providing a
  way to define more specific policies. Clients can list or read queries for
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
    <td>The ACL Token used to create the query, or a management token must be supplied in order to perform these operations.</td>
    <td>Any client with the ID of the query can perform these operations.</td>
  </tr>
  <tr>
    <td>Manage static query with a `Name`</td>
    <td>The ACL token used to create the query, or a management token must be supplied in order to perform these operations.</td>
    <td>Similar to create, the client token's `query` ACL policy is used to determine if these operations are allowed.</td>
  </tr>
  <tr>
    <td>List queries</td>
    <td>A management token is required to list any queries.</td>
    <td>The client token's `query` ACL policy is used to determine which queries they can see. Only management tokens can see prepared queries without `Name`.</td>
  </tr>
  <tr>
    <td>Execute query</td>
    <td>Since a `Token` is always captured when a query is created, that is used to check access to the service being queried. Any token supplied by the client is ignored.</td>
    <td>The captured token, client's token, or anonymous token is used to filter the results, as described above.</td>
  </tr>
</table>

#### Service Rules

The `service` policy controls service-level registration and read access to the [Catalog API](/api/catalog.html)
and service discovery with the [Health API](/api/health.html).

Service rules look like this:

```text
service "" {
  policy = "read"
}
service "app" {
  policy = "write"
}
service "admin" {
  policy = "deny"
}
```

Service rules are keyed by the service name prefix they apply to, using the longest prefix match rule. In
the example above, the rules allow read-only access to any service name with the empty prefix, allow
read-write access to any service name that starts with "app", and deny all access to any service name that
starts with "admin".

Consul's DNS interface is affected by restrictions on service rules. If the
[`acl_token`](/docs/agent/options.html#acl_token) used by the agent does not have "read" access to a
given service, then the DNS interface will return no records when queried for it.

When reading from the catalog or retrieving information from the health endpoints, service rules are
used to filter the results of the query.

Service rules come into play when using the [Agent API](/api/agent.html) to register services or
checks. The agent will check tokens locally as a service or check is registered, and Consul also
performs periodic [anti-entropy](/docs/internals/anti-entropy.html) syncs, which may require an
ACL token to complete. To accommodate this, Consul provides two methods of configuring ACL tokens
to use for registration events:

1. Using the [acl_token](/docs/agent/options.html#acl_token) configuration
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
[`enable_script_checks`](/docs/agent/options.html#_enable_script_checks) set to `true` in order to enable
script checks.


#### Session Rules

The `session` policy controls access to [Session API](/api/session.html) operations.

Session rules look like this:

```text
session "" {
  policy = "read"
}
session "app" {
  policy = "write"
}
session "admin" {
  policy = "deny"
}
```

Session rules are keyed by the node name prefix they apply to, using the longest prefix match rule. In
the example above, the rules allow read-only access to sessions on node name with the empty prefix, allow
creating sessions on any node name that starts with "app", and deny all access to any sessions on a node
name that starts with "admin".

## Advanced Topics

<a name="replication"></a>
#### Outages and ACL Replication

The Consul ACL system is designed with flexible rules to accommodate for an outage
of the [`acl_datacenter`](/docs/agent/options.html#acl_datacenter) or networking
issues preventing access to it. In this case, it may be impossible for
agents in non-authoritative datacenters to resolve tokens. Consul provides
a number of configurable [`acl_down_policy`](/docs/agent/options.html#acl_down_policy)
choices to tune behavior. It is possible to deny or permit all actions or to ignore
cache TTLs and enter a fail-safe mode. The default is to ignore cache TTLs
for any previously resolved tokens and to deny any uncached tokens.

Consul 0.7 added an ACL Replication capability that can allow non-authoritative
datacenter agents to resolve even uncached tokens. This is enabled by setting an
[`acl_replication_token`](/docs/agent/options.html#acl_replication_token) in the
configuration on the servers in the non-authoritative datacenters. In Consul
0.9.1 and later you can enable ACL replication using
[`enable_acl_replication`](/docs/agent/options.html#enable_acl_replication) and
then set the token later using the
[agent token API](/api/agent.html#update-acl-tokens) on each server. This can
also be used to rotate the token without restarting the Consul servers.

With replication enabled, the servers will maintain a replica of the authoritative
datacenter's full set of ACLs on the non-authoritative servers. The ACL replication
token needs to be a valid ACL token with management privileges, it can also be the
same as the master ACL token.

Replication occurs with a background process that looks for new ACLs approximately
every 30 seconds. Replicated changes are written at a rate that's throttled to
100 updates/second, so it may take several minutes to perform the initial sync of
a large set of ACLs.

If there's a partition or other outage affecting the authoritative datacenter,
and the [`acl_down_policy`](/docs/agent/options.html#acl_down_policy)
is set to "extend-cache", tokens will be resolved during the outage using the
replicated set of ACLs. An [ACL replication status](/api/acl.html#acl_replication_status)
endpoint is available to monitor the health of the replication process.
Also note that in recent versions of Consul (greater than 1.2.0), using
`acl_down_policy = "async-cache"` refreshes token asynchronously when an ACL is
already cached and is expired while similar semantics than "extend-cache".
It allows to avoid having issues when connectivity with the authoritative is not completely
broken, but very slow.

Locally-resolved ACLs will be cached using the [`acl_ttl`](/docs/agent/options.html#acl_ttl)
setting of the non-authoritative datacenter, so these entries may persist in the
cache for up to the TTL, even after the authoritative datacenter comes back online.

ACL replication can also be used to migrate ACLs from one datacenter to another
using a process like this:

1. Enable ACL replication in all datacenters to allow continuation of service
during the migration, and to populate the target datacenter. Verify replication
is healthy and caught up to the current ACL index in the target datacenter
using the [ACL replication status](/api/acl.html#acl_replication_status)
endpoint.
2. Turn down the old authoritative datacenter servers.
3. Rolling restart the agents in the target datacenter and change the
`acl_datacenter` servers to itself. This will automatically turn off
replication and will enable the datacenter to start acting as the authoritative
datacenter, using its replicated ACLs from before.
3. Rolling restart the agents in other datacenters and change their `acl_datacenter`
configuration to the target datacenter.

<a name="version_8_acls"></a>
#### Complete ACL Coverage in Consul 0.8

Consul 0.8 added many more ACL policy types and brought ACL enforcement to Consul
agents for the first time. To ease the transition to Consul 0.8 for existing ACL
users, there's a configuration option to disable these new features. To disable
support for these new ACLs, set the
[`acl_enforce_version_8`](/docs/agent/options.html#acl_enforce_version_8) configuration
option to `false` on Consul clients and servers.

Here's a summary of the new features:

* Agents now check [`node`](#node-rules) and [`service`](#service-rules) ACL policies
  for catalog-related operations in `/v1/agent` endpoints, such as service and check
  registration and health check updates.
* Agents enforce a new [`agent`](#agent-rules) ACL policy for utility operations in
  `/v1/agent` endpoints, such as joins and leaves.
* A new [`node`](#node-rules) ACL policy is enforced throughout Consul, providing a
  mechanism to restrict registration and discovery of nodes by name. This also applies
  to service discovery, so provides an additional dimension for controlling access to
  services.
* A new [`session`](#session-rules) ACL policy controls the ability to create session
  objects by node name.
* Anonymous prepared queries (non-templates without a `Name`) now require a valid
  session, which ties their creation to the new [`session`](#session-rules) ACL policy.
* The existing [`event`](#event-rules) ACL policy has been applied to the
  `/v1/event/list` endpoint.

Two new configuration options are used once version 8 ACLs are enabled:

* [`acl_agent_master_token`](/docs/agent/options.html#acl_agent_master_token) is used as
  a special access token that has `agent` ACL policy `write` privileges on each agent where
  it is configured, as well as `node` ACL policy `read` privileges for all nodes. This token
  should only be used by operators during outages when Consul servers aren't available to
  resolve ACL tokens. Applications should use regular ACL tokens during normal operation.
* [`acl_agent_token`](/docs/agent/options.html#acl_agent_token) is used internally by
  Consul agents to perform operations to the service catalog when registering themselves
  or sending network coordinates to the servers. This token must at least have `node` ACL
  policy `write` access to the node name it will register as in order to register any
  node-level information like metadata or tagged addresses.

Since clients now resolve ACLs locally, the [`acl_down_policy`](/docs/agent/options.html#acl_down_policy)
now applies to Consul clients as well as Consul servers. This will determine what the
client will do in the event that the servers are down.

Consul clients must have [`acl_datacenter`](/docs/agent/options.html#acl_datacenter) configured
in order to enable agent-level ACL features. If this is set, the agents will contact the Consul
servers to determine if ACLs are enabled at the cluster level. If they detect that ACLs are not
enabled, they will check at most every 2 minutes to see if they have become enabled, and will
start enforcing ACLs automatically. If an agent has an `acl_datacenter` defined, operators will
need to use the [`acl_agent_master_token`](/docs/agent/options.html#acl_agent_master_token) to
perform agent-level operations if the Consul servers aren't present (such as for a manual join
to the cluster), unless the [`acl_down_policy`](/docs/agent/options.html#acl_down_policy) on the
agent is set to "allow".

Non-server agents do not need to have the
[`acl_master_token`](/docs/agent/options.html#acl_master_token) configured; it is not
used by agents in any way.

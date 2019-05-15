---
layout: "docs"
page_title: "ACL Rules"
sidebar_current: "docs-acl-rules"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL system is a Capability-based system that relies on tokens which can have fine grained rules applied to them. It is very similar to AWS IAM in many ways.
---

-> **1.4.0 and later:** This document only applies in Consul versions 1.4.0 and later. The documentation for the legacy ACL system is [here](/docs/acl/acl-legacy.html)

# ACL Rules

Consul provides an optional Access Control List (ACL) system which can be used 
to control access to data and APIs. To learn more about Consul's ACL review the
[ACL system documentation](/docs/acl/acl-system.html)

A core part of the ACL system is the rule language, which is used to describe the policy
that must be enforced. There are two types of rules: prefix based rules and exact matching
rules. 

## Rule Specification

Rules are composed of a resource, a segment (for some resource areas) and a policy
disposition. The general structure of a rule is:

```text
<resource> "<segment>" {
  policy = "<policy disposition>"
}
```

Segmented resource areas allow operators to more finely control access to those resources.
Note that not all resource areas are segmented such as the `keyring`, `operator`, and `acl` resources. For those rules they would look like:

```text
<resource> = "<policy disposition>"
```

Policies can have several control levels:

* `read`: allow the resource to be read but not modified.
* `write`: allow the resource to be read and modified.
* `deny`: do not allow the resource to be read or modified.
* `list`: allows access to all the keys under a segment in the Consul KV. Note, this policy can only be used with the `key_prefix` resource and [`acl.enable_key_list_policy`](/docs/agent/options.html#acl_enable_key_list) must be set to true.

When using prefix-based rules, the most specific prefix match determines the action. This
allows for flexible rules like an empty prefix to allow read-only access to all
resources, along with some specific prefixes that allow write access or that are
denied all access. Exact matching rules will only apply to the exact resource specified.
The order of precedence for matching rules are, DENY has priority over WRITE or READ and 
 WRITE has priority over READ.  

We make use of the
[HashiCorp Configuration Language (HCL)](https://github.com/hashicorp/hcl/) to specify
rules. This language is human readable and interoperable with JSON making it easy to
machine-generate. Rules can make use of one or more policies.

Specification in the HCL format looks like:

```hcl
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
# Or for exact key matches
key "foo/bar/secret" {
  policy = "deny"
}

# This controls access to cluster-wide Consul operator information.
operator = "read"
```

This is equivalent to the following JSON input:

```json
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
  },
  "operator": "read"
}
```

The [ACL API](/api/acl/acl.html) allows either HCL or JSON to be used to define the content
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

The created policy can now be specified either by name or by ID when 
[creating a token](https://learn.hashicorp.com/consul/security-networking/production-acls#create-the-agent-token). This will grant the rules
provided to the [bearer of that token](/api/index.html#authentication).

Below is a breakdown of each rule type. 

#### ACL Resource Rules

The `acl` resource controls access to ACL operations in the
[ACL API](/api/acl/acl.html).

ACL rules look like this:

```hcl
acl = "write"
```

There is only one acl rule allowed per policy and its value is set to one of the [policy dispositions](/docs/acl/acl-rules.html#rule-specification). In the example
above ACLs may be read or written including discovering any token's secret ID. Snapshotting also requires `acl = "write"`
permissions due to the fact that all the token secrets are contained within the snapshot.

#### Agent Rules

The `agent` and `agent_prefix` resources control access to the utility operations in the [Agent API](/api/agent.html),
such as join and leave. All of the catalog-related operations are covered by the [`node` or `node_prefix`](#node-rules)
and [`service` or `service_prefix`](#service-rules) policies instead.

Agent rules look like this:

```hcl
agent_prefix "" {
  policy = "read"
}
agent "foo" {
  policy = "write"
}
agent_prefix "bar" {
  policy = "deny"
}
```

Agent rules are keyed by the node name they apply to. In the example above the rules
allow read-only access to any node name by using the empty prefix, read-write access to 
the node with the _exact_ name `foo`, and denies all access to any noe name that starts
with `bar`. 

Since [Agent API](/api/agent.html) utility operations may be required before an agent is joined to
a cluster, or during an outage of the Consul servers or ACL datacenter, a special token may be
configured with [`acl.tokens.agent_master`](/docs/agent/options.html#acl_tokens_agent_master) to allow
write access to these operations even if no ACL resolution capability is available.

#### Event Rules

The `event` and `event_prefix` resources control access to event operations in the [Event API](/api/event.html), such as
firing events and listing events.

Event rules look like this:

```hcl
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

```hcl
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

```hcl
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

```hcl
key "foo" {
  policy = "write"
  sentinel {
      code = <<EOF
import "strings"
main = rule { strings.has_suffix(value, "bar") }
EOF
      enforcementlevel = "hard-mandatory"
  }
}
```

For more detailed information, see the [Consul Sentinel documentation](/docs/agent/sentinel.html).

#### Keyring Rules

The `keyring` resource controls access to keyring operations in the
[Keyring API](/api/operator/keyring.html).

Keyring rules look like this:

```hcl
keyring = "write"
```

There's only one keyring policy allowed per rule set, and its value is set to one of the policy
dispositions. In the example above, the keyring may be read and updated.

#### Node Rules

The `node` and `node_prefix` resources controls node-level registration and read access to the [Catalog API](/api/catalog.html),
service discovery with the [Health API](/api/health.html), and filters results in [Agent API](/api/agent.html)
operations like fetching the list of cluster members.

Node rules look like this:

```hcl
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

```hcl
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

```hcl
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

```hcl
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

```hcl
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

---
layout: "docs"
page_title: "Bootstrapping ACLs"
sidebar_current: "docs-guides-acl"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL system is a Capability-based system that relies on tokens which can have fine grained rules applied to them. It is very similar to AWS IAM in many ways.
---

# Bootstrapping the ACL System

Consul uses Access Control Lists (ACLs) to secure the UI, API, CLI, service communications, and agent communications. For securing gossip and RPC communication please review [this guide](/docs/guides/agent-encryption.html). When securing your cluster you should configure the ACLs first. 

At the core, ACLs operate by grouping rules into policies, then associating one or more policies with a token.

To complete this guide, you should have an operational Consul 1.4+ cluster. We also recommend reading the [ACL System documentation](/docs/agent/acl-system.html). For securing Consul version 1.3 and older, please read the [legacy ACL documentation](https://www.consul.io/docs/guides/acl-legacy.html).

Bootstrapping the ACL system is a multi-step process, we will cover all the necessary steps in this guide. 

* [Enable ACLs on all the servers](/docs/guides/acl.html#step-1-enable-acls-on-all-the-consul-servers).
* [Create the initial bootstrap token](/docs/guides/acl.html#step-2-create-the-bootstrap-token).
* [Create the agent policy](/docs/guides/acl.html#step-3-create-an-agent-token-policy).
* [Create the agent token](/docs/guides/acl.html#step-4-create-an-agent-token).
* [Apply the new token to the servers](/docs/guides/acl.html#step-5-add-the-agent-token-to-all-the-servers). 
* [Enable ACLs on the clients and apply the agent token](/docs/guides/acl.html#step-6-enable-acls-on-the-consul-clients).

At the end of this guide, there are also several additional and optional steps.

## Step 1: Enable ACLs on all the Consul Servers

The first step for bootstrapping the ACL system is to enable ACLs on the Consul servers in the agent configuration file. In this example, we are configuring the default policy of "deny", which means we are in whitelist mode, and a down policy of "extend-cache", which means that we will ignore token TTLs during an outage.

```json
{
  "acl" : {
    "enabled" : true,
    "default_policy" : "deny",
    "down_policy" : "extend-cache"
  }
}
```

The servers will need to be restarted to load the new configuration. Please take care
to restart the servers one at a time and ensure each server has joined and is operating
correctly before restarting another.

If ACLs are enabled correctly, we will now see the following warnings and info in the leader's logs.

```sh
2018/12/12 01:36:40 [INFO] acl: Created the anonymous token
2018/12/12 01:36:40 [INFO] consul: ACL bootstrap enabled
2018/12/12 01:36:41 [INFO] agent: Synced node info
2018/12/12 01:36:58 [WARN] agent: Coordinate update blocked by ACLs
2018/12/12 01:37:40 [INFO] acl: initializing acls
2018/12/12 01:37:40 [INFO] consul: Created ACL 'global-management' policy
```

If you do not see ACL bootstrap enabled, the anonymous token creation, and the `global-management` policy creation message in the logs, ACLs have not been properly enabled. 

Note, now that we have enabled ACLs, we will need a token to complete any operation. We can't do anything else to the cluster until we bootstrap and generate the first master token. For simplicity we will use the master token created during the bootstrap for the remainder of the guide.

## Step 2: Create the Bootstrap Token

Once ACLs have been enabled we can bootstrap our first token, the bootstrap token. 
The bootstrap token is a management token with unrestricted privileges. It will
be shared with all the servers in the quorum, since it will be added to the 
state store. 

```bash
$ consul acl bootstrap
AccessorID: edcaacda-b6d0-1954-5939-b5aceaca7c9a
SecretID: 4411f091-a4c9-48e6-0884-1fcb092da1c8
Description: Bootstrap Token (Global Management)
Local: false
Create Time: 2018-12-06 18:03:23.742699239 +0000 UTC
Policies:
00000000-0000-0000-0000-000000000001 - global-management
```

On the server where the `bootstrap` command was issued we should see the following log message. 

```sh
2018/12/11 15:30:23 [INFO] consul.acl: ACL bootstrap completed
2018/12/11 15:30:23 [DEBUG] http: Request PUT /v1/acl/bootstrap (2.347965ms) from=127.0.0.1:40566
```

Since ACLs have been enabled, we will need to use it to complete any additional operations.
For example, even checking the member list will require a token.  

```sh
$ consul members -token "4411f091-a4c9-48e6-0884-1fcb092da1c8"
Node  Address            Status  Type    Build  Protocol  DC   Segment
fox   172.20.20.10:8301  alive   server  1.4.0  2         kc  <all>
bear  172.20.20.11:8301  alive   server  1.4.0  2         kc  <all>
wolf  172.20.20.12:8301  alive   server  1.4.0  2         kc  <all>
```

Note using the token on the command line with the `-token` flag is not 
recommended, instead we will set it as an environment variable once.

```sh
$ export CONSUL_HTTP_TOKEN=4411f091-a4c9-48e6-0884-1fcb092da1c8
```

The bootstrap token can also be used in the server configuration file as 
the [`master`](https://www.consul.io/docs/agent/options.html#acl_tokens_master) token.

Note, the bootstrap token can only be created once, bootstrapping will be disabled after the master token was created. Once the ACL system is bootstrapped, ACL tokens can be managed through the
[ACL API](/api/acl/acl.html).

## Step 3: Create an Agent Token Policy

Before we can create a token, we will need to create its associated policy. A policy is a set of rules that can be used to specify granular permissions. To learn more about rules, read the ACL rule specification [documentation](/docs/agent/acl-rules.html).

```bash
# agent-policy.hcl contains the following:
node_prefix "" {
   policy = "write"
}
service_prefix "" {
   policy = "read"
}
```

This policy will allow all nodes to be registered and accessed and any service to be read. 
Note, this simple policy is not recommended in production.
It is best practice to create separate node policies and tokens for each node in the cluster
with an exact-match node rule.

We only need to create one policy and can do this on any of the servers. If you have not set the 
`CONSUL_HTTP_TOKEN` environment variable to the bootstrap token, please refer to the previous step. 

```
$ consul acl policy create -name "agent-token" -description "Agent Token Policy" -rules @agent-policy.hcl
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

The returned value is the newly-created policy that we can now use when creating our agent token. 

## Step 4: Create an Agent Token

Using the newly created policy, we can create an agent token. Again we can complete this process on any of the servers. For this guide, all agents will share the same token. Note, the `SecretID` is the token used to authenticate API and CLI commands. 

```sh
$ consul acl token create -description "Agent Token" -policy-name "agent-token"
AccessorID:   499ab022-27f2-acb8-4e05-5a01fff3b1d1
SecretID:     da666809-98ca-0e94-a99c-893c4bf5f9eb
Description:  Agent Token
Local:        false
Create Time:  2018-10-19 14:23:40.816899 -0400 EDT
Policies:
   fcd68580-c566-2bd2-891f-336eadc02357 - agent-token
```

## Step 5: Add the Agent Token to all the Servers

Our final step for configuring the servers is to assign the token to all of our
Consul servers via the configuration file and reload the Consul service 
on all of the servers, one last time.

```json
{
  "primary_datacenter": "dc1",
  "acl" : {
    "enabled" : true,
    "default_policy" : "deny",
    "down_policy" : "extend-cache",
    "tokens" : {
      "agent" : "da666809-98ca-0e94-a99c-893c4bf5f9eb"
    }
  }
}
```

~> Note: In Consul version 1.4.2 and older any ACL updates
in the agent configuration file will require a full restart of the 
Consul service. 

At this point we should no longer see the coordinate warning in the servers logs, however, we should continue to see that the node information is in sync.

```sh
2018/12/11 15:34:20 [DEBUG] agent: Node info in sync
```

It is important to ensure the servers are configured properly, before enable ACLs 
on the clients. This will reduce any duplicate work and troubleshooting, if there
is a misconfiguration.  

#### Ensure the ACL System is Configured Properly

Before configuring the clients, we should check that the servers are healthy. To do this, let's view the catalog.

```sh
curl http://127.0.0.1:8500/v1/catalog/nodes -H 'x-consul-token: 4411f091-a4c9-48e6-0884-1fcb092da1c8' 
[
    {
        "Address": "172.20.20.10",
        "CreateIndex": 7,
        "Datacenter": "kc",
        "ID": "881cfb69-2bcd-c2a9-d87c-cb79fc454df9",
        "Meta": {
            "consul-network-segment": ""
        },
        "ModifyIndex": 10,
        "Node": "fox",
        "TaggedAddresses": {
            "lan": "172.20.20.10",
            "wan": "172.20.20.10"
        }
    }
]
``` 

All the values should be as expected. Particularly, if `TaggedAddresses` is `null` it is likely we have not configured ACLs correctly. A good place to start debugging is reviewing the Consul logs on all the servers.

If you encounter issues that are unresolvable, or misplace the bootstrap token, you can reset the ACL system by updating the index. First re-run the bootstrap command to get the index number.

```
$ consul acl bootstrap
Failed ACL bootstrapping: Unexpected response code: 403 (Permission denied: ACL bootstrap no longer allowed (reset index: 13))
```

Then write the reset index into the bootstrap reset file: (here the reset index is 13)

```
$ echo 13 >> <data-directory>/acl-bootstrap-reset
```

After reseting the ACL system you can start again at Step 2. 

## Step 6: Enable ACLs on the Consul Clients

Since ACL enforcement also occurs on the Consul clients, we need to also restart them
with a configuration file that enables ACLs. We can use the same ACL agent token that we created for the servers. The same token can be used because we did not specify any node or service prefixes.

```json
{
  "acl" : {
    "enabled" : true,
    "down_policy" : "extend-cache",
    "tokens" : {
      "agent" : "da666809-98ca-0e94-a99c-893c4bf5f9eb"
    }
  }
}
```

To ensure the agent's are configured correctly, we can again use the `/catalog` endpoint. 

## Additional ACL Configuration

Now that the nodes have been configured to use ACLs, we can configure the CLI, UI, and nodes to use specific tokens. All of the following steps are optional examples. In your own environment you will likely need to create more fine grained policies.

#### Configure the Anonymous Token (Optional)

The anonymous token is created during the bootstrap process, `consul acl bootstrap`. It is implicitly used if no token is supplied. In this section we will update the existing token with a newly created policy.

At this point ACLs are bootstrapped with ACL agent tokens configured, but there are no
other policies set up. Even basic operations like `consul members` will be restricted
by the ACL default policy of "deny":

```
$ consul members
```

We will not receive an error, since the ACL has filtered what we see and we are not allowed to
see any nodes by default.

If we supply the token we created above we will be able to see a listing of nodes because
it has write privileges to an empty `node` prefix, meaning it has access to all nodes:

```bash
$ CONSUL_HTTP_TOKEN=4411f091-a4c9-48e6-0884-1fcb092da1c8 consul members
Node  Address            Status  Type    Build  Protocol  DC   Segment
fox   172.20.20.10:8301  alive   server  1.4.0  2         kc  <all>
bear  172.20.20.11:8301  alive   server  1.4.0  2         kc  <all>
wolf  172.20.20.12:8301  alive   server  1.4.0  2         kc  <all>
```

It is common in many environments to allow listing of all nodes, even without a
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
Node  Address            Status  Type    Build  Protocol  DC   Segment
fox   172.20.20.10:8301  alive   server  1.4.0  2         kc  <all>
bear  172.20.20.11:8301  alive   server  1.4.0  2         kc  <all>
wolf  172.20.20.12:8301  alive   server  1.4.0  2         kc  <all>
```

The anonymous token is also used for DNS lookups since there is no way to pass a
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
```

Now we get an `NXDOMAIN` error because the anonymous token doesn't have access to the
"consul" service. Let's update the anonymous token's policy to allow for service reads of the "consul" service.

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
```

The next section shows an alternative to the anonymous token.

#### Set Agent-Specific Default Tokens (Optional)

An alternative to the anonymous token is the [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default)
configuration item. When a request is made to a particular Consul agent and no token is
supplied, the [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default) will be used for the token, instead of being left empty which would normally invoke the anonymous token.

This behaves very similarly to the anonymous token, but can be configured differently on each
agent, if desired. For example, this allows more fine grained control of what DNS requests a
given agent can service or can give the agent read access to some key-value store prefixes by
default.

If using [`acl.tokens.default`](/docs/agent/options.html#acl_tokens_default), then it's likely the anonymous token will have a more restrictive policy than shown in these examples.

#### Create Tokens for UI Use (Optional)

If you utilize the Consul UI with a restrictive ACL policy, as above, the UI will not function fully using the anonymous ACL token. It is recommended that a UI-specific ACL token is used, which can be set in the UI during the web browser session to authenticate the interface.

First create the new policy.

```bash
$ consul acl policy create -name "ui-policy" \
                           -description "Necessary permissions for UI functionality" \
                           -rules 'key_prefix "" { policy = "write" } node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }'
ID:           9cb99b2b-3c20-81d4-a7c0-9ffdc2fbf08a
Name:         ui-policy
Description:  Necessary permissions for UI functionality
Datacenters:
Rules:
key_prefix "" { policy = "write" } node_prefix "" { policy = "read" } service_prefix "" { policy = "read" }
```

With the new policy, create a token.

```sh
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

Note, in this example, we have also given full write access to the KV through the UI.

## Summary

The [ACL API](/api/acl/acl.html) can be used to create tokens for applications specific to their intended use and to create more specific ACL agent tokens for each agent's expected role. 
Now that you have bootstrapped ACLs, learn more about [ACL rules](/docs/agent/acl-rules.html)

### Notes on Security 

In this guide we configured a basic ACL environment with the ability to see all nodes
by default, but with limited access to discover only the "consul" service. If your environment has stricter security requirements we would like to note the following and make some additional recommendations. 

1. In this guide we added the agent token to the configuration file. This means the tokens are now saved on disk. If this is a security concern, tokens can be added to agents using the [Consul CLI](https://www.consul.io/docs/commands/acl/acl-set-agent-token.html). However, this process is more complicated and takes additional care. 

2. It is recommended that each client get an ACL agent token with `node` write privileges for just its own node name, and `service` read privileges for just the service prefixes expected to be registered on that client.

3. [Anti-entropy](/docs/internals/anti-entropy.html) syncing requires the ACL agent token
to have `service:write` privileges for all services that may be registered with the agent.
We recommend providing `service:write` for each separate service via a separate token that 
is used when registering via the API, or provided along with the [registration in the 
configuration file](https://www.consul.io/docs/agent/services.html). Note that `service:write`
is the privilege required to assume the identity of a service and so Consul Connect's
intentions are only enforceable to the extent that each service instance is unable to gain 
`service:write` on any other service name. For more details see the Connect security
[documentation](https://www.consul.io/docs/connect/security.html).



---
layout: "docs"
page_title: "Securing Consul with ACLs"
sidebar_current: "docs-guides-acl-production"
description: |-
  This guide walks though securing your production Consul datacenter with ACLs.
---

The [Bootstrapping the ACL System guide](/advanced/day-1-operations/acl-guide)
walks you through how to set up ACLs on a single datacenter. Because it
introduces the basic concepts and syntax we recommend completing it before
starting this guide. This guide builds on the first guide with recommendations
for production workloads on a single datacenter. 

After [bootstrapping the ACL
system](/advanced/day-1-operations/production-acls#bootstrap-the-acl-system),
you will learn how to create tokens with minimum privileges for:

* [Servers and Clients](/advanced/day-1-operations/production-acls#apply-individual-tokens-to-agents)
* [Services](/advanced/day-1-operations/production-acls#apply-individual-tokens-to-services)
* [DNS](/advanced/day-1-operations/production-acls#token-for-dns) 
* [Consul KV](/advanced/day-1-operations/production-acls#consul-kv-tokens) 
* [Consul UI](/advanced/day-1-operations/production-acls#consul-ui-tokens) 

~> **Important:** For best results, use this guide during the [initial
deployment](/advanced/day-1-operations/deployment-guide) of a Consul (version
1.4.3 or newer) datacenter. Specifically, you should have already installed all
agents and configured initial service definitions, but you should not yet rely
on Consul for any service discovery or service configuration operations.  

## Bootstrap the ACL System

You will bootstrap the ACL system in two steps, enable ACLs and create the
bootstrap token.  

### Enable ACLs on the Agents

To enable ACLs, add the following [ACL
parameters](https://www.consul.io/docs/agent/options.html#configuration-key-reference)
to the agent's configuration file and then restart the Consul service. If you
want to reduce Consul client restarts, you can enable the ACLs 
on them when you apply the token. 

```
# agent.hcl 
{ 
  acl = { 
  	enabled = true, 
  	default_policy = "deny",
    enable_token_persistence = true
  	} 
} 
```

~> Note: Token persistence was introduced in Consul 1.4.3. In older versions
of Consul, you cannot persist tokens when using the HTTP API. 

In this example, you configured the default policy of "deny", which means you
are in whitelist mode. You also enabled token persistence when using the HTTP
API. With persistence enabled, tokens will be persisted to disk and 
reloaded when an agent restarts

~> Note: If you are bootstrapping ACLs on an existing datacenter, enable the
ACLs on the agents first with `default_policy=allow`. Default policy allow will
enable ACLs, but will allow all operations, allowing the cluster to function
normally while you create the tokens and apply them. This will reduce downtime.
You should update the configuration files on all the servers first and then
initiate a rolling restart. 

### Create the Initial Bootstrap Token

To create the initial bootstrap token, use the `acl bootstrap` command on one
of the servers. 

```sh
$ consul acl bootstrap 
```

The output gives you important information about the token, including the
associated policy `global-management` and `SecretID`. 

~> Note: By default, Consul assigns the `global-management` policy to the
bootstrap token, which has unrestricted privileges. It is important to have one
token with unrestricted privileges in case of emergencies; however you should
only give a small number of administrators access to it. The `SecretID` is a
UUID that you will use to identify the token when using the Consul CLI or HTTP
API. 

While you are setting up the ACL system, set the `CONSUL_HTTP_TOKEN`
environment variable to the bootstrap token on one server, for this guide
the example is on server "consul-server-one". This gives you the necessary 
privileges to continue
creating policies and tokens. Set the environment variable temporarily with
`export`, so that it will not persist once you’ve closed the session. 

```sh 
$ export CONSUL_HTTP_TOKEN=<your_token_here> 
```

Now, all of the following commands in this guide can 
be completed on the same server, in this
case server "consul-server-one". 

## Apply Individual Tokens to Agents

Adding tokens to agents is a three step process.
 
1. [Create the agent
policy](/advanced/day-1-operations/production-acls/create-the-agent-policy).
2. [Create the token with the newly created
policy](/advanced/day-1-operations/production-acls/create-the-agent-token).
3. [Add the token to the agent](/advanced/day-1-operations/production-acls/add-the-token-to-the-agent).
 

### Create the Agent Policy

We recommend creating agent policies that have write privileges for node
related actions including registering itself in the catalog, updating node
level health checks, and having write access on its configuration file. The
example below has unrestricted privileges for node related actions for
"consul-server-one" only.
 
```
# consul-server-one-policy.hcl
node "consul-server-one" { 
  policy = "write"
} 
```

When creating agent policies, review the [node rules](
https://www.consul.io/docs/agent/acl-rules.html#node-rules). Now that 
you have
specified the policy, you can initialize it using the Consul
CLI. To create a programmatic process, you could also use
the HTTP API.

```sh 
$ consul acl policy create -name consul-server-one -rules @consul-server-one-policy.hcl 
```

The command output will include the policy information. 

Repeat this process for all servers and clients in the Consul datacenter. Each agent should have its own policy based on the
node name, that grants write privileges to it. 

### Create the Agent Token

After creating the per-agent policies, create individual tokens for all the
agents. You will need to include the policy in the `consul acl token create`
command.

```sh 
$ consul acl token create -description "consul-server-one agent token" -policy-name consul-server-one 
``` 

This command returns the token information, which should include a description
and policy information.

Repeat this process for each agent. It is the responsibility of the operator to
save tokens in a secure location; we recommend
[Vault](https://www.vaultproject.io/).  

### Add the Token to the Agent.

Finally, apply the tokens to the agents using the HTTP API. 
Start with the servers
and ensure they are working correctly before applying the client tokens. Please
review the Bootstrapping the ACL System [guide](/advanced/day-1-operations/acl-guide) for example of setting the token in the agent configuration 
file.

```sh
$ consul acl set-agent-token -token "<your token here>" agent "<agent token here>"
``` 

The data file must contain a valid token. 

```
# consul-server-one-token.json
{
  "Token": "adf4238a-882b-9ddc-4a9d-5b6758e4159e"
}
```

At this point, every agent that has a token can once
again read and write information to Consul, but only for node-related actions.
Actions for individual services are not yet allowed.

~> Note: If you are bootstrapping ACLs on an existing datacenter, remember to
update the default policy to `default_policy = deny` and initiate another
rolling restart. After applying the token. 
  

## Apply Individual Tokens to the Services

The token creation and application process for services is similar to agents. 
Create a policy.  Use that policy to create a token.  Add the token to the
service. Service tokens are necessary for
 agent anti-entropy, registering and de-registering the service, and
 registering and de-registering the service's checks.

Review the [service
rules](https://www.consul.io/docs/agent/acl-rules.html#service-rules) before
getting started.

Below is an example service definition that needs a token after bootstrapping
the ACL system.

```json 
{ 
  "service": { 
    "name": "dashboard", 
    "port": 9002, 
    "check": { 
    	"id": "dashboard-check", 
    	"http": "http://localhost:9002/health", 
    	"method": "GET",
    	"interval": "1s", 
    	"timeout": "1s" 
    	} 
    } 
} 
```

This service definition should be located in the [configuration
directory](https://www.consul.io/docs/agent/options.html#_config_dir) on one of
the clients.  

First, create the policy that will grant write privileges to only the
"dashboard" service. This means the "dashboard" service can register
itself, update it's health checks, and write any of the fields in the [service
definition](https://www.consul.io/docs/agent/services.html).

```sh
# dashboard-policy.hcl
service "dashboard" { 
	policy = "write" 
} 
```

Use the policy definition to initiate the policy.

```sh 
$ consul acl policy create -name "dashboard-service" -rules @dashboard-policy.hcl 
```

Next, create a token with the policy.

```sh 
$ consul acl token create -description "Token for Dashboard Service" -policy-name dashboard-service 
```

The command will return information about the token, which should include a
description and policy information. As usual, save the token to a secure
location.


Finally, add the token to the service definition. 

``` 
{ 
  "service": { 
  	"name": "dashboard", 
  	"port": 9002, 
  	"token": "57c5d69a-5f19-469b-0543-12a487eecc66", 
  	"check": { 
  		"id": "dashboard-check",
  		"http": "http://localhost:9002/health", 
  		"method": "GET", 
  		"interval": "1s",
  		"timeout": "1s" 
  		} 
  	} 
 } 
```

If the service is running, you will need to restart it. Unlike with agent 
tokens, there is no HTTP API endpoint to apply the token directly to the
service. If the service is registered with a configuration file, you must
also set the token in the configuration file. However, if you register a
 service with the HTTP API, you can pass the token in the [header](https://www.consul.io/api/index.html#authentication) with
  `X-Consul-Token` and it will be used by the service.

If you are using a sidecar proxy, it can inherit the token from the service
definition. Alternatively, you can create a separate token. 

## Token for DNS

Depending on your use case, the token used for DNS may need policy rules for
[nodes](https://www.consul.io/docs/agent/acl-rules.html#node-rules),
[services](https://www.consul.io/docs/agent/acl-rules.html#service-rules), and
[prepared queries](https://www.consul.io/docs/agent/acl-rules.html#prepared-query-rules).
You should apply the token to the Consul agent serving DNS requests. When the
DNS server makes a request to Consul, it will include the token in the request.
Consul can either authorize or revoke the request, depending on the token's
privileges. The token creation for DNS is the same three step process you used
for agents and services, create a policy, create a token, apply the
token. 

Below is an example of a policy that provides read privileges for all services,
nodes, and prepared queries. 

```
# dns-request-policy.hcl
node_prefix "" { 
	policy = "read" 
} 
service_prefix "" { 
	policy = "read" 
}
# only needed if using prepared queries
query_prefix "" { 
	policy = "read" 
} 
```

First, create the policy.

```sh 
$ consul acl policy create -name "dns-requests" -rules @dns-request-policy.hcl 
```

Next, create the token.

```sh 
$ consul acl token create -description "Token for DNS Requests" -policy-name dns-requests 
```

Finally, apply the token to the Consul agent serving DNS request in default token ACL
configuration parameter.

```sh
$ consul acl set-agent-token -token "<your token here>" default "<dns token>"
```

The data file must contain a valid token. 

``` 
# dns-token.json
{ 
  "Token":"5467d69a-5f19-469b-0543-12a487eecc66" 
}

``` 

Note, if you have multiple agents serving DNS requests you can use the same
 policy to create individual tokens for all of them if they are using the same rules.


## Consul KV Tokens

The  process of creating tokens for Consul KV follows the same three step
process as nodes and services. First create a policy, then a token, and finally
apply or use the token. However, unlike tokens for nodes and services Consul KV
has many varied use cases. 

- Services may need to access configuration data in the key-value store. 
- You may want to store distributed lock information for sessions.  
- Operators may need access to
update configuration values in the key-value store. . 

The [rules for
KV](https://www.consul.io/docs/agent/acl-rules.html#key-value-rules) have four
policy levels; `deny`, `write`, `read`, and `list`.  Let's review several
examples of `read` and `write`.

Depending on the use case, the token will be applied differently. For services
you will add the token to the HTTP client. For operators use, the
operator will use the token when issuing commands, either with the CLI or API.

### Recursive Reads

``` 
key_prefix "redis/" { 
  policy = "read" 
} 
```

In the above example, we are allowing any key with the prefix `redis/` to be
read. If you issued the command `consul kv get -recurse redis/ -token=<your
token> ` you would get a list of key/values for `redis/`. 

This type of policy is good for allowing operators to recursively read
configuration parameters stored in the KV. Similarly, a "write" policy with the
same prefix would allow you to update any keys that begin with "redis/".

### Write Privileges for One Key

``` 
key "dashboard-app" {
  policy = "write" 
} 
```

In the above example, we are allowing read and write privileges to the
dashboard-app key. This allows for `get`, `delete`,  and `put` operations. 

This type of token would allow an application to update and read a value in the
KV store. It would also be useful for operators who need access to set specific
keys. 

### Read Privileges for One Key

``` 
key "counting-app" { 
  policy = "read" 
} 
```

In the above example, we are setting a read privileges for a single key,
“counting-app”. This allows for only `get` operations. 

This type of token allows an application to simply read from a key to get the
value. This is useful for configuration parameter updates.

## Consul UI Token

Once you have bootstrapped the ACL system, access to the UI will be limited.
The anonymous token grants UI access if no [default
token](https://www.consul.io/docs/agent/options.html#acl_tokens_default) is set
on the agents, and all operations will be denied, including viewing nodes and
services. 

You can re-enable UI features (with flexible levels of access) by creating and
distributing tokens to operators. Once you have a token, you can use it in the
UI by adding it to the "ACL" page:

![Access Controls](/assets/images/guides/access-controls.png "Access Controls")

After saving a new token, you will be able to see your tokens.

![Tokens](/assets/images/guides/tokens.png "Tokens")

The browser stores tokens that you add to the UI. This allows you to distribute
different levels of privileges to operators. Like other tokens, it's up to the
operator to decide the per-token privileges. 

Below is an example of policy that
will allow the operator to have read access to the UI for services, nodes,
key/values, and intentions. You need to have  "acl = read"  to view policies
and tokens. Otherwise you will not be able to access the ACL section of the UI,
not even to view the token you used to access the UI.

```
# operator-ui.hcl
service_prefix "" { 
  policy = "read" 
  } 
key_prefix "" { 
  policy = "read" 
  }
node_prefix "" { 
  policy = "read" 
  }
```

## Summary

In this guide you bootstrapped the ACL system for consul and applied tokens to agents and services. You assigned tokens for DNS, Consul KV, and the Consul UI. 

To learn more about Consul’s security model read the [internals documentation](https://www.consul.io/docs/internals/security.html). You can find commands relating to ACLs in our [reference documentation](https://www.consul.io/docs/commands/acl.html).



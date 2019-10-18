---
name: Troubleshooting the ACL System
content_length: 8
id: acl-troubleshooting
products_used:
  - Consul
description: >-
  In this guide, you will gain familiarity with several Consul CLI commands that can be used to help troubleshoot issues with tokens and policies.
level: Implementation
---

Consul provides a robust set of APIs that you can use to check the health of your datacenter. In this guide, you will learn about several Consul CLI commands that you can use to troubleshoot issues with tokens and policies. Additionally, you will learn about the ACL system reset procedure that can be used encase of an emergency. 

This guide has four sections.

* [consul members](#consul-members)
* [consul catalog](#consul-catalog)
* [consul acl commands](#consul-acl)
* [Reset the ACL System](#reset-the-acl-system)

## Prerequisites 

This guide assumes an existing datacenter running Consul 1.4 or newer. Access to a machine where you can execute Consul CLI commands, either a Consul agent or a local binary configured to control the remote datacenter.

All commands in this guide will need a valid token. For ease, we recommend using the bootstrap token since it has unlimited privileges. 

## Consul Members

You can use the `consul members` [command](https://www.consul.io/docs/commands/members.html) when configuring agents with tokens to check that they have the necessary privileges to join the datacenter. 

If an agent, server or client, is missing from the members list then ACLs have not been configured correctly on that agent or the token does not have the correct privileges. Only agents that have privileges to register themselves in the catalog will be included in the member list. 

```sh
$ consul members
Node       Address         Status    Type    Build  Protocol  DC   Segment
server-1  172.17.0.2:8301  alive     server  1.4.4  2         dc1  <all>
server-2  172.17.0.3:8301  alive     server  1.4.4  2         dc1  <all>
server-3  172.17.0.4:8301  alive     server  1.4.4  2         dc1  <all>
```

Use the `consul acl` commands listed in the following sections to help troubleshoot token privileges. 

## Consul Catalog

The `consul catalog nodes -detailed` [command](https://www.consul.io/docs/commands/catalog/nodes.html) will display node information, including "TaggedAddresses". If "TaggedAddresses" is null for any of the agents, that agent’s ACLs are not configured correctly. You can start debugging by reviewing the Consul logs on all the servers. If ACLs are enabled correctly, you can investigate the agent's token.

```sh
$ consul catalog nodes -detailed
Node      ID                  Address   DC  TaggedAddresses                       
server-1  a82c7db3-fdc3  192.168.1.191  kc  lan=192.168.1.191, wan=192.168.1.190  
server-2  a82c7db3-fdc3  192.168.1.192  kc  lan=192.168.1.192, wan=192.168.1.190  
server-3  a82c7db3-fdc3  192.168.1.193   kc  lan=192.168.1.193, wan=192.168.1.190 
```

Use the `consul acl` commands listed below to help troubleshoot troubleshoot token privileges. 

## Consul ACL 

Once you've confirmed the issue is not due to misconfiguration, you can use the following commands to help troubleshoot token or policy issues.

#### Consul ACL Policy List

The `consul acl policy list` [command](https://www.consul.io/docs/commands/acl/acl-policy.html#list) will output all the available policies. When you're first setting up tokens you should use it to ensure the list of policies and their rules are as expected. In the example output below, there are two policies; the Consul created `global-management` policy and a user created policy named `server-one-policy`.

```shell
$ consul acl policy list
global-management:
   ID:           00000000-0000-0000-0000-000000000001
   Description:  Builtin Policy that grants unlimited access
   Datacenters:
server-one-policy:
   ID:           0bcee22c-6602-9dd6-b147-964958069426
   Description:  policy for server one
   Datacenters:

```

You can use `consul acl policy read -id <policy_id>` to investigate individual policies. In the example output below, the `server-one-policy` policy has node write privileges for node "consul-server-one". 

```shell
consul acl policy read -id 0bcee22c-6602-9dd6-b147-964958069426 
ID:           0bcee22c-6602-9dd6-b147-964958069426
Name:         server-one-policy
Description:  policy for server one
Datacenters:
Rules:
node "consul-server-one" {
  policy = "write"
}
```

### Consul ACL Token List

The `consul acl token list` [command](https://www.consul.io/docs/commands/acl/acl-token.html#list) will list all the tokens. Ensure this list only includes the tokens in use. It is important for the security of your datacenter and you should check it often. Since tokens do not expire, it is up to the operator to [delete](https://www.consul.io/docs/commands/acl/acl-token.html#delete) tokens that are not in use. 

In the example output below, there are three tokens. The first token is created by Consul during the bootstrap process, it is often referred to as the bootstrap token. The second token is user generated and was created with the server-one-policy policy. The third token is also created by Consul, but it has no privileges. 

```shell
$ consul acl token list
AccessorID:       cf827c04-fb7d-ea75-da64-84e1dd2d5dfe
Description:      Master Token
Local:            false
Create Time:      2019-05-20 11:08:27.253096 -0500 CDT
Legacy:           false
Policies:
   00000000-0000-0000-0000-000000000001 - global-management

AccessorID:       5d3c3a03-e627-a749-444c-2984101190c0
Description:      token for server one
Local:            false
Create Time:      2019-10-17 11:46:27.106158 -0500 CDT
Legacy:           false
Policies:
   0bcee22c-6602-9dd6-b147-964958069426 - server-one-policy

AccessorID:       00000000-0000-0000-0000-000000000002
Description:      Anonymous Token
Local:            false
Create Time:      2019-05-20 11:08:27.253959 -0500 CDT
Legacy:           false
```

The `consul acl token read` [command](https://www.consul.io/docs/commands/acl/acl-token.html#read) will provide information about the token specified. Ensure the privileges of the token are expected. This is useful when checking a node or service has the correct privileges to add itself to the catalog. 

In the example output below includes the same information that is returned with the `consul acl token list` command, but is useful when you do not want to view all tokens.

```sh
$ consul acl token read -id 5d3c3a03-e627-a749-444c-2984101190c0
AccessorID:       5d3c3a03-e627-a749-444c-2984101190c0
SecretID:         547a969c-5dff-f9a8-6b84-fb1d23f9a5cb
Description:      token for server one
Local:            false
Create Time:      2019-10-17 11:46:27.106158 -0500 CDT
Policies:
   0bcee22c-6602-9dd6-b147-964958069426 - server-one-policy
```

Use the `consul acl token read` command first if the `consul catalog` or `consul members` commands return unexpected results.

## Reset the ACL System

If you encounter issues that are unresolvable, or misplace the bootstrap token, you can reset the ACL system by updating the index. First re-run the bootstrap command to get the index number.

```sh
$ consul acl bootstrap
Failed ACL bootstrapping: Unexpected response code: 403 (Permission denied: ACL bootstrap no longer allowed (reset index: 13))
```

Then write the reset index into the bootstrap reset file: (here the reset index is 13)

```
$ echo 13 >> <data-directory>/acl-bootstrap-reset
```

After resetting the ACL system, you can recreate the bootstrap token.

### Summary 

In this guide, you learned how to use the Consul CLI to troubleshoot the ACL system. Each command has a corresponding [API](https://www.consul.io/api/index.html). You also learned how to reset the the ACL system encase of emergency. Note, resetting the ACL system will invalidate all tokens generated before the reset. 


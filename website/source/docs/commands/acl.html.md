---
layout: "docs"
page_title: "Commands: ACL"
sidebar_current: "docs-commands-acl"
---

# Consul ACLs

Command: `consul acl`

The `acl` command is used to interact with Consul's ACLs via the command
line. It exposes top-level commands for bootstrapping the ACL system,
managing tokens and policies, translating legacy rules, and setting the
tokens for use by an agent.

ACLs are also accessible via the [HTTP API](/api/acl/acl.html).


Bootstrap Consul's ACLs:

```sh
$ consul acl bootstrap
AccessorID:   4d123dff-f460-73c3-02c4-8dd64d136e01
SecretID:     86cddfb9-2760-d947-358d-a2811156bf31
Description:  Bootstrap Token (Global Management)
Local:        false
Create Time:  2018-10-22 11:27:04.479026 -0400 EDT
Policies:
   00000000-0000-0000-0000-000000000001 - global-management
```

Create a policy:

```sh
$ consul acl policy create -name "acl-replication" -description "Token capable of replicating ACL policies" -rules 'acl = "read"'
ID:           35b8ecb0-707c-ee18-2002-81b238b54b38
Name:         acl-replication
Description:  Token capable of replicating ACL policies
Datacenters:
Rules:
acl = "read"
```

Create a token:

```sh
$ consul acl token create -description "Agent Policy Replication - my-agent" -policy-name "acl-replication"
AccessorID:   c24c11aa-4e08-e25c-1a67-705a2e8d75a4
SecretID:     e7024f9c-f016-02dd-6217-daedbffb86ac
Description:  Agent Policy Replication - my-agent
Local:        false
Create Time:  2018-10-22 11:34:49.960482 -0400 EDT
Policies:
   35b8ecb0-707c-ee18-2002-81b238b54b38 - acl-replication
```

For more examples, ask for subcommand help or view the subcommand documentation
by clicking on one of the links in the sidebar.

## Usage

Usage: `consul acl <subcommand>`

For the exact documentation for your Consul version, run `consul acl -h` to
view the complete list of subcommands.

```text
Usage: consul acl <subcommand> [options] [args]

  This command has subcommands for interacting with Consul's ACLs.
  Here are some simple examples, and more detailed examples are available
  in the subcommands or the documentation.

  Bootstrap ACLs:

      $ consul acl bootstrap

  List all ACL tokens:

      $ consul acl token list

  Create a new ACL policy:

      $ consul acl policy create -name "new-policy" \
                                 -description "This is an example policy" \
                                 -datacenter "dc1" \
                                 -datacenter "dc2" \
                                 -rules @rules.hcl

  Set the default agent token:

      $ consul acl set-agent-token default 0bc6bc46-f25e-4262-b2d9-ffbe1d96be6f

  For more examples, ask for subcommand help or view the documentation.

Subcommands:
    auth-method        Manage Consul's ACL auth methods
    binding-rule       Manage Consul's ACL binding rules
    bootstrap          Bootstrap Consul's ACL system
    policy             Manage Consul's ACL policies
    role               Manage Consul's ACL roles
    set-agent-token    Assign tokens for the Consul Agent's usage
    token              Manage Consul's ACL tokens
    translate-rules    Translate the legacy rule syntax into the current syntax

```

For more information, examples, and usage about a subcommand, click on the name
of the subcommand in the sidebar or one of the links below:

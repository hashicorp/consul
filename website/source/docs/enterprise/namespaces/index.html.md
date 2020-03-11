---
layout: "docs"
page_title: "Consul Enterprise Namespaces"
sidebar_current: "docs-enterprise-namespaces"
description: |-
  Consul Enterprise enables data isolation with Namespaces.
---

# Consul Enterprise Namespaces

With [Consul Enterprise](https://www.hashicorp.com/consul.html) v1.7.0, data for different users or teams
can be isolated from each other with the use of Namespaces. Namespaces help reduce operational challenges 
by removing restrictions around uniqueness of resource names across distinct teams, and enable operators 
to provide self-service through delegation of administrative privileges.

For more information on how to use namespaces with Consul Enterprise please review the following Learn Guides:

- [Register and Discover Services within Namespaces](https://learn.hashicorp.com/consul/namespaces/discovery-namespaces) - Register multiple services within different Namespaces in Consul
- [Setup Secure Namespaces](https://learn.hashicorp.com/consul/namespaces/secure-namespaces) - Secure resources within a namespace and delegate Namespace ACL rights via ACL tokens


## Namespace Definition

Namespaces are managed exclusively through the [HTTP API](/api/namespaces.html) and the [Consul CLI](/docs/commands/namespace.html).
The HTTP API accepts only JSON formatted definitions while the CLI will parse either JSON or HCL.

An example Namespace definition looks like the following:

JSON:

```json
{
   "Name": "team-1",
   "Description": "Namespace for Team 1",
   "ACLs": {
      "PolicyDefaults": [
         {
            "ID": "77117cf6-d976-79b0-d63b-5a36ac69c8f1"
         },
         {
            "Name": "node-read"
         }
      ],
      "RoleDefaults": [
         {
            "ID": "69748856-ae69-d620-3ec4-07844b3c6be7"
         },
         {
            "Name": "ns-team-2-read"
         }
      ]
   },
   "Meta": {
      "foo": "bar"
   }
}
```

HCL:

```hcl
Name = "team-1"
Description = "Namespace for Team 1"
ACLs {
  PolicyDefaults = [
    {
      ID = "77117cf6-d976-79b0-d63b-5a36ac69c8f1"
    },
    {
      Name = "node-read"
    }
  ]
  RoleDefaults = [
    {
      "ID": "69748856-ae69-d620-3ec4-07844b3c6be7"
    },
    {
      "Name": "ns-team-2-read"
    }
  ]
}
Meta {
  foo = "bar"
}
```

### Fields

- `Name` `(string: <required>)` - The Namespaces name must be a valid DNS hostname label.

- `Description` `(string: "")` - This field is intended to be a human readable description of the
  namespace's purpose. It is not used internally.
  
- `ACLs` `(object: <optional>)` - This fields is a nested JSON/HCL object to contain the Namespaces
  ACL configuration. 
  
  - `PolicyDefaults` `(array<ACLLink>)` - A list of default policies to be applied to all tokens
    created in this namespace. The ACLLink object can contain an `ID` and/or `Name` field. When the
    policies ID is omitted Consul will resolve the name to an ID before writing the Namespace
    definition internally. Note that all policies linked in a Namespace definition must be defined
    within the `default namespace.
    
  - `RoleDefaults` `(array<ACLLink>)` - A list of default roles to be applied to all tokens
    created in this namespace. The ACLLink object can contain an `ID` and/or `Name` field. When the
    roles' ID is omitted Consul will resolve the name to an ID before writing the Namespace
    definition internally. Note that all roles linked in a Namespace definition must be defined
    within the `default namespace.
    
- `Meta` `(map<string|string>: <optional>)` - Specifies arbitrary KV metadata to associate with
  this namespace.

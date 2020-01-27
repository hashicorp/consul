---
layout: "docs"
page_title: "ACL Guides"
sidebar_current: "docs-guides-acl-index"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. Select the following guide for your use case.
---

# ACL Documentation and Guides

Consul uses Access Control Lists (ACLs) to secure the UI, API, CLI, service communications, and agent communications. At the core, ACLs operate by grouping rules into policies, then associating one or more policies with a token.

The following documentation and guides will help you understand and implement
ACLs.

## ACL Documentation

### ACL System

Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. The ACL system is a Capability-based system that relies on tokens which can have fine grained rules applied to them. The [ACL System documentation] details the functionality of Consul ACLs.

### ACL Rules

A core part of the ACL system is the rule language, which is used to describe the policy that must be enforced. The [ACL Rules documentation] is useful 
when creating rule specifications.

### ACL Legacy System

The ACL system in Consul 1.3.1 and older is now called legacy. For information on bootstrapping the legacy system, ACL rules, and a general ACL system overview, read the legacy [documentation](/docs/acl/acl-legacy.html).

### ACL Migration

[The migration documentation](/docs/acl/acl-migrate-tokens.html) details how to upgrade
existing legacy tokens after upgrading to 1.4.0. It will briefly describe what changed, and then walk through the high-level migration process options, finally giving some specific examples of migration strategies. The new ACL system has improvements for the security and management of ACL tokens and policies.

## ACL Guides

We have several guides for setting up and configuring Consul's ACL system. They include how to bootstrap the ACL system in Consul version 1.4.0 and newer. Please select one of the following guides to get started.

~> Note: the following are located on HashiCorp Learn. By selecting
one of the guides, you will be directed to a new site.

### Bootstrapping the ACL System 

Learn how to control access to Consul resources with this step-by-step [guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/acl-guide) on bootstrapping the ACL system in Consul 1.4.0 and newer. This guide also includes additional steps for configuring the anonymous token, setting up agent-specific default tokens, and creating tokens for Consul UI use. 

### Securing Consul with ACLs

The _Bootstrapping the ACL System_ guide walks you through how to set up ACLs on a single datacenter. Because it introduces the basic concepts and syntax we recommend completing it before starting this guide. This guide builds on the first guide with recommendations for production workloads on a single datacenter.





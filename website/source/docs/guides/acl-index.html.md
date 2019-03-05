---
layout: "docs"
page_title: "ACL Guides"
sidebar_current: "docs-guides-acl-index"
description: |-
  Consul provides an optional Access Control List (ACL) system which can be used to control access to data and APIs. Select the following guide for your use case.
---

# ACL Guides

We have several guides for setting up and configuring Consul's ACL system. They include how to bootstrap the ACL system in Consul version 1.4.0 and newer, how to bootstrap the ACL system in older versions of Consul, and how to migrate tokens from the legacy system to the new system in Consul 1.4.0.

Please select one of the following guides to get started.

## Bootstrapping Guide

Learn how to control access to Consul resources with this step-by-step [guide](https://learn.hashicorp.com/consul/advanced/day-1-operations/acl-guide) on bootstrapping the ACL system in Consul 1.4.0 and newer. This guide also includes additional steps for configuring the anonymous token, setting up agent-specific default tokens, and creating tokens for Consul UI use. 

## Legacy ACL System

The ACL system in Consul 1.3.1 and older is now called legacy. For information on bootstrapping the legacy system, ACL rules, and a general ACL system overview, read the legacy [guide](/docs/guides/acl-legacy.html).

## Migrating Tokens

[This guide](/docs/guides/acl-migrate-tokens.html) documents how to upgrade
existing legacy tokens after upgrading to 1.4.0. It will briefly describe what changed, and then walk through the high-level migration process options, finally giving some specific examples of migration strategies. The new ACL system has improvements for the security and management of ACL tokens and policies.




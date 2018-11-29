---
layout: "docs"
page_title: "Consul-AWS"
sidebar_current: "docs-guides-consul-aws"
description: |-
  Consul-AWS provides a tool, which syncs Consul's and AWS Cloud Map's service catalog
---

# Consul-AWS

[Consul-AWS](https://github.com/hashicorp/consul-aws/) syncs the services in an AWS Cloud Map namespace to a Consul datacenter. Consul services will be created in AWS Cloud Map and the other way around. This enables native service discovery across Consul and AWS Cloud Map.
This guide will describe how to configure and how to start the sync.

## Authentication

`consul-aws` needs access to Consul and AWS for uni- and bidirectional sync.

For Consul, the process accepts both the standard CLI flags, `-token` and the environment variables `CONSUL_HTTP_TOKEN`. This should be set to a Consul ACL token if ACLs are enabled.

For AWS, `consul-aws` uses the default credential provider chain to find AWS credentials. The default provider chain looks for credentials in the following order:
1. Environment variables.
2. Shared credentials file.
3. If your application is running on an Amazon EC2 instance, IAM role for Amazon EC2.

## Configuration

There are two subcommands available on `consul-aws`:

* version: display version number
* sync-catalog: start syncing the catalogs

The version subcommand doesn’t do anything besides showing the version, so lets focus on sync-catalog. The following flags are available:

* A set of parameters to connect to your Consul Cluster like `-http-addr`, `-token`, `-ca-file`, `-client-cert`, and everything else you might need in order to do that
* `-aws-namespace-id`: The AWS namespace to sync with Consul services.
* `-aws-service-prefix`: A prefix to prepend to all services written to AWS from Consul. If this is not set then services will have no prefix.
* `-consul-service-prefix`: A prefix to prepend to all services written to Consul from AWS. If this is not set then services will have no prefix.
* `-to-aws`: If true, Consul services will be synced to AWS (defaults to false).
* `-to-consul`: If true, AWS services will be synced to Consul (defaults to false).
* `-aws-pull-interval`: The interval between fetching from AWS Cloud Map. Accepts a sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms", "10s", "1.5m" (defaults to 30s).
* `-aws-dns-ttl`: DNS TTL for services created in AWS Cloud Map in seconds (defaults to 60).

Independent of how you want to use `consul-aws` it needs to be able to connect to Consul and AWS. Apart from making sure you setup up authenticated access, `-aws-namespace-id` is mandatory.

## Syncing Consul services to AWS Cloud Map

Assuming authenticated access is set up, there is little left to do before starting the sync. Using `-to-aws` command line flag will start the sync to AWS Cloud Map. If `-aws-service-prefix` is provided, every imported service from Consul will be prefixed. For example:

```shell
$ consul-aws -aws-namespace-id ns-hjrgt3bapp7phzff -to-aws -consul-service-prefix consul_
```

At this point `consul-aws` will start importing services into AWS Cloud Map. A service in Consul named `web` will end up becoming `consul_web` in AWS. The individual service instances from Consul will be created in AWS as well.

Services in AWS Cloud Map that were imported from Consul have the following properties:

* Description:  “Imported from Consul”
* Record types: A and SRV
* DNS routing policy: Multivalue answer routing

## Syncing AWS Cloud Map services to Consul

Similar to the previous chapter, there are two relevant flags: `-to-consul` to turn on the sync and optionally `-consul-service-prefix` to prefix every service imported into Consul. For example:

```shell
$ consul-aws -aws-namespace-id ns-hjrgt3bapp7phzff -to-consul -aws-service-prefix aws_
```

At this point `consul-aws` will start importing services into Consul. A service in AWS named `redis` will end up becoming `aws_redis` in Consul. The individual service instances from AWS will be created in Consul as well.

* Services in Consul that were imported from AWS Cloud Map have the following properties:
* Tag: aws
* Meta-Data: has aws as the source set, as well as the aws-id, the aws-namespace and every custom attribute the instance had in AWS Cloud Map
* Node: the node name is consul-aws

## Syncing both directions

To enable bidirectional sync only put together the previous two sections and provide `-to-consul` and `-to-aws` as well as optionally `-aws-service-prefix` and `-consul-service-prefix`:

```shell
$ consul-aws -aws-namespace-id ns-hjrgt3bapp7phzff -to-consul -aws-service-prefix aws_ -to-aws -consul-service-prefix consul_
```

At this point `consul-aws` will start importing services into Consul from AWS Cloud Map and from AWS Cloud Map to Consul.

## Summary

At this point, either uni- or bidirectional sync is set up and service discovery is available across Consul and AWS seamlessly. If you haven’t enabled [ACL](/docs/guides/acl.html), now is a good time to read about it.

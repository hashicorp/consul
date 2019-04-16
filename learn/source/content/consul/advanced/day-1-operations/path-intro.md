---
name: 'Introduction'
content_length: 5
id: /day-1-operations/path-intro
layout: content_layout
products_used:
  - Consul
description: Deploy your first datacenter with Consul.
level: Advanced
---

This learning path is designed to help you deploy your first datacenter. If you are responsible for setting up and maintaining a healthy datacenter, this path will help you do so successfully. We will cover the following topics:

- Infrastructure recommendations
- Setting up a datacenter
- Backing up the state of the datacenter
- Securing the datacenter
- Ensuring the datacenter is healthy

Below you will find all of the guides that make up this learning path. If you want to skip ahead to any guide, feel free to use them as needed. Each guide has a description along with its objective to help you decide. However, if you are deploying your first production ready datacenter, we recommend completing these guides in order.

## Reference Architecture

By the end of this guide, you will be ready to create a architecture diagram for your environment. You will be able to identify which ports should be open, select hardware sizes that meet your needs, and understand how to implement datacenter design best practices.

[Reference Architecture](/consul/advanced/day-1-operations/reference-architecture)

## Deployment Guide

By the end of the guide, you will install and configure a single Consul datacenter. You will use the examples to create your own custom configuration files for both servers and clients. The custom configuration files will help you join agents, optimize Raft performance, enable the collection of metrics, and configure the web UI. Finally, the guide will detail how to configure Systemd.

[Deployment Guide](/consul/advanced/day-1-operations/deployment-guide)

## Datacenter Backups

By the end of this guide, you will have a backup process outlined. You will also be able to list the server data that is saved. Finally, you will understand the process for restoring from a backup.

[Datacenter Backups](/consul/advanced/day-1-operations/backup)

## Bootstrapping ACL System

By the end of this guide, you will have ACLs configured on the Consul agents, servers and clients.
For each step, you will be able to recognize if the process is not properly executed. Optionally, you can also configure the _anonyomous token_ and token for the UI.

[Bootstrapping ACLs](/consul/advanced/day-1-operations/acl-guide)

## Creating Agent Certificates

By the end of the this guide, you will know how to generate certificates for your cluster. This guide will cover how to create a Certificate Authority(CA), and how to generate server certificates and client certificates.

[Creating Agent Certificates](/consul/advanced/day-1-operations/certificates)

## Gossip and RPC Encryption

By the end of this guide, you will be able to configure gossip and RPC encryption on a your cluster. Encrypting both incoming and outgoing communication is crucial for securing the cluster.

[Gossip and RPC Encryption](/consul/advanced/day-1-operations/agent-encryption)

## Monitor Cluster Health

By the end of the monitoring guide, you will be able to collect agent metrics. You will be able to identify the various methods for collecting metrics and configure them for your use cases; quick collection or to use with monitoring software. You will also be able to interpret the key metrics for a healthy cluster.

[Monitor Cluster Health](/consul/advanced/day-1-operations/monitoring)

## Get Started

Now that we have reviewed the guides that make up the Day 1 Operations learning path,
get started by either hitting the next button at the bottom of the page or select the
guide that you are interested in. If you encounter any technical difficulties while working
through the guides or have any feedback please send an email to the [Consul mailing list](https://groups.google.com/forum/#!forum/consul-tool).

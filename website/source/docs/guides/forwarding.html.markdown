---
layout: "docs"
page_title: "Forwarding"
sidebar_current: "docs-guides-forwarding"
---

# Forwarding DNS queries from Bind To Consul

In order to not have to run Consul as root and bind to port 53 it's best if 
it's paired with Bind. 

In this example, Bind and Consul are running on the same machine

### DNSSEC

First, you have to disable DNSSEC so that Consul and Bind can communicate

    options {
        listen-on port 53 { 127.0.0.1; };
        listen-on-v6 port 53 { ::1; };
        directory       "/var/named";
        dump-file       "/var/named/data/cache_dump.db";
        statistics-file "/var/named/data/named_stats.txt";
        memstatistics-file "/var/named/data/named_mem_stats.txt";
        allow-query     { localhost; };
        recursion yes;

        dnssec-enable no;
        dnssec-validation no;

        /* Path to ISC DLV key */
        bindkeys-file "/etc/named.iscdlv.key";

        managed-keys-directory "/var/named/dynamic";
    };

    include "/etc/named/consul.conf";

Then we set up a zone for our Consul managed records in consul.conf:

    zone "consul" IN {
        type forward;
        forward only;
        forwarders { 127.0.0.1 port 8600; };
    };

We can extend this even further to make separate zones for different data centers / Consul clusters.

    zone "n-california.consul" IN {
        type forward;
        forward only;
        forwarders { 172.16.0.15 port 8600; 172.16.0.16 port 8600; };
    }

    zone "oregon.consul" IN {
        type forward;
        forward only;
        forwarders { 172.24.0.1 port 8600; 172.24.0.1 port 8600; };
    }



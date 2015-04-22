---
layout: "docs"
page_title: "Forwarding"
sidebar_current: "docs-guides-forwarding"
description: |-
  By default, DNS is served from port 53.  On most operating systems, this requires elevated privileges. Instead of running Consul with an administrative or root account, it is possible to instead forward appropriate queries to Consul, running on an unprivileged port, from another DNS server.
---

# Forwarding DNS

By default, DNS is served from port 53.  On most operating systems, this
requires elevated privileges. Instead of running Consul with an administrative
or root account, it is possible to instead forward appropriate queries to Consul,
running on an unprivileged port, from another DNS server.

In this guide, we will demonstrate forwarding from [BIND](https://www.isc.org/downloads/bind/).
For the sake of simplicity, BIND and Consul are running on the same machine in this example,
but this is not required.

### BIND Setup

First, you have to disable DNSSEC so that Consul and BIND can communicate.
Here is an example of such a configuration:

```text
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
```

### Zone File

Then we set up a zone for our Consul managed records in consul.conf:

```text
zone "consul" IN {
  type forward;
  forward only;
  forwarders { 127.0.0.1 port 8600; };
};
```

Here we assume Consul is running with default settings and is serving
DNS on port 8600.

### Testing

First, perform a DNS query against Consul directly to be sure that the record exists:

```text
[root@localhost ~]# dig @localhost -p 8600 master.redis.service.dc-1.consul. A

; <<>> DiG 9.8.2rc1-RedHat-9.8.2-0.23.rc1.32.amzn1 <<>> @localhost master.redis.service.dc-1.consul. A
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 11536
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;master.redis.service.dc-1.consul. IN A

;; ANSWER SECTION:
master.redis.service.dc-1.consul. 0 IN A 172.31.3.234

;; Query time: 4 msec
;; SERVER: 127.0.0.1#53(127.0.0.1)
;; WHEN: Wed Apr  9 17:36:12 2014
;; MSG SIZE  rcvd: 76
```

Then run the same query against your BIND instance and make sure you get a result:

```text
[root@localhost ~]# dig @localhost -p 53 master.redis.service.dc-1.consul. A

; <<>> DiG 9.8.2rc1-RedHat-9.8.2-0.23.rc1.32.amzn1 <<>> @localhost master.redis.service.dc-1.consul. A
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 11536
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;master.redis.service.dc-1.consul. IN A

;; ANSWER SECTION:
master.redis.service.dc-1.consul. 0 IN A 172.31.3.234

;; Query time: 4 msec
;; SERVER: 127.0.0.1#53(127.0.0.1)
;; WHEN: Wed Apr  9 17:36:12 2014
;; MSG SIZE  rcvd: 76
```

### Troubleshooting

If you don't get an answer from BIND but you do get an answer from Consul, your
best bet is to turn on BIND's query log to see what's happening:

```text
[root@localhost ~]# rndc querylog
[root@localhost ~]# tail -f /var/log/messages
```

The log may show errors like this:

```text
error (no valid RRSIG) resolving
error (no valid DS) resolving
```

This indicates that DNSSEC is not disabled properly.

If you see errors about network connections, verify that there are no firewall
or routing problems between the servers running BIND and Consul.

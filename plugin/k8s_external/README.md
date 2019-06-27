# k8s_external

## Name

*k8s_external* - resolve load balancer and external IPs from outside kubernetes clusters.

## Description

This plugin allows an additional zone to resolve the external IP address(es) of a Kubernetes
service. This plugin is only useful if the *kubernetes* plugin is also loaded.

The plugin uses an external zone to resolve in-cluster IP addresses. It only handles queries for A,
AAAA and SRV records, all others result in NODATA responses. To make it a proper DNS zone it handles
SOA and NS queries for the apex of the zone.

By default the apex of the zone will look like (assuming the zone used is `example.org`):

~~~ dns
example.org.	5 IN	SOA ns1.dns.example.org. hostmaster.example.org. (
				12345      ; serial
				14400      ; refresh (4 hours)
				3600       ; retry (1 hour)
				604800     ; expire (1 week)
				5          ; minimum (4 hours)
				)
example.org		5 IN	NS ns1.dns.example.org.

ns1.dns.example.org.  5 IN  A    ....
ns1.dns.example.org.  5 IN  AAAA ....
~~~

Note we use the `dns` subdomain to place the records the DNS needs (see the `apex` directive). Also
note the SOA's serial number is static. The IP addresses of the nameserver records are those of the
CoreDNS service.

The *k8s_external* plugin handles the subdomain `dns` and the apex of the zone by itself, all other
queries are resolved to addresses in the cluster.

## Syntax

~~~
k8s_external [ZONE...]
~~~

* **ZONES** zones *k8s_external* should be authoritative for.

If you want to change the apex domain or use a different TTL for the return records you can use
this extended syntax.

~~~
k8s_external [ZONE...] {
    apex APEX
    ttl TTL
}
~~~

* **APEX** is the name (DNS label) to use the apex records, defaults to `dns`.
* `ttl` allows you to set a custom **TTL** for responses. The default is 5 (seconds).

# Examples

Enable names under `example.org` to be resolved to in cluster DNS addresses.

~~~
. {
   kubernetes cluster.local
   k8s_external example.org
}
~~~

With the Corefile above, the following Service will get an `A` record for `test.default.example.org` with IP address `192.168.200.123`.

~~~
apiVersion: v1
kind: Service
metadata:
 name: test
 namespace: default
spec:
 clusterIP: None
 externalIPs:
 - 192.168.200.123
 type: ClusterIP
~~~


# Also See

For some background see [resolve external IP address](https://github.com/kubernetes/dns/issues/242).
And [A records for services with Load Balancer IP](https://github.com/coredns/coredns/issues/1851).

# Bugs

PTR queries for the reverse zone is not supported.

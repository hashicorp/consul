# dnssec

*dnssec* enables on-the-fly DNSSEC signing of served data.

## Syntax

~~~
dnssec [ZONES...]
~~~

* **ZONES** zones that should be signed. If empty, the zones from the configuration block
    are used.

If keys are not specified (see below), a key is generated and used for all signing operations. The
DNSSEC signing will treat this key a CSK (common signing key), forgoing the ZSK/KSK split. All
signing operations are done online. Authenticated denial of existence is implemented with NSEC black
lies. Using ECDSA as an algorithm is preferred as this leads to smaller signatures (compared to
RSA). NSEC3 is *not* supported.

A single signing key can be specified by using the `key` directive.

NOTE: Key generation has not been implemented yet.

TODO(miek): think about key rollovers, and how to do them automatically.

~~~
dnssec [ZONES... ] {
    key file KEY...
    cache_capacity CAPACITY
}
~~~

* `key file` indicates that key file(s) should be read from disk. When multiple keys are specified, RRsets
  will be signed with all keys. Generating a key can be done with `dnssec-keygen`: `dnssec-keygen -a
  ECDSAP256SHA256 <zonename>`. A key created for zone *A* can be safely used for zone *B*.

* `cache_capacity` indicates the capacity of the LRU cache. The dnssec middleware uses LRU cache to manage
  objects and the default capacity is 10000.

## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metrics are exported:

* coredns_dnssec_cache_size{type} - total elements in the cache, type is "signature".
* coredns_dnssec_cache_capacity{type} - total capacity of the cache, type is "signature".
* coredns_dnssec_cache_hits_total - Counter of cache hits.
* coredns_dnssec_cache_misses_total - Counter of cache misses.

## Examples

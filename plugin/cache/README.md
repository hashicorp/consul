# cache

## Name

*cache* - enables a frontend cache.

## Description

With *cache* enabled, all records except zone transfers and metadata records will be cached for up to
3600s. Caching is mostly useful in a scenario when fetching data from the backend (upstream,
database, etc.) is expensive.

## Syntax

~~~ txt
cache [TTL] [ZONES...]
~~~

* **TTL** max TTL in seconds. If not specified, the maximum TTL will be used, which is 3600 for
    noerror responses and 1800 for denial of existence ones.
    Setting a TTL of 300: `cache 300` would cache records up to 300 seconds.
* **ZONES** zones it should cache for. If empty, the zones from the configuration block are used.

Each element in the cache is cached according to its TTL (with **TTL** as the max).
For the negative cache, the SOA's MinTTL value is used. A cache can contain up to 10,000 items by
default. A TTL of zero is not allowed.

If you want more control:

~~~ txt
cache [TTL] [ZONES...] {
    success CAPACITY [TTL]
    denial CAPACITY [TTL]
    prefetch AMOUNT [[DURATION] [PERCENTAGE%]]
}
~~~

* **TTL**  and **ZONES** as above.
* `success`, override the settings for caching successful responses. **CAPACITY** indicates the maximum
  number of packets we cache before we start evicting (*randomly*). **TTL** overrides the cache maximum TTL.
* `denial`, override the settings for caching denial of existence responses. **CAPACITY** indicates the maximum
  number of packets we cache before we start evicting (LRU). **TTL** overrides the cache maximum TTL.
  There is a third category (`error`) but those responses are never cached.
* `prefetch` will prefetch popular items when they are about to be expunged from the cache.
  Popular means **AMOUNT** queries have been seen with no gaps of **DURATION** or more between them.
  **DURATION** defaults to 1m. Prefetching will happen when the TTL drops below **PERCENTAGE**,
  which defaults to `10%`, or latest 1 second before TTL expiration. Values should be in the range `[10%, 90%]`.
  Note the percent sign is mandatory. **PERCENTAGE** is treated as an `int`.

## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metrics are exported:

* `coredns_cache_size{type}` - Total elements in the cache by cache type.
* `coredns_cache_capacity{type}` - Total capacity of the cache by cache type.
* `coredns_cache_hits_total{type}` - Counter of cache hits by cache type.
* `coredns_cache_misses_total{}` - Counter of cache misses.

Cache types are either "denial" or "success".

## Examples

Enable caching for all zones, but cap everything to a TTL of 10 seconds:

~~~ corefile
. {
    cache 10
    whoami
}
~~~

Proxy to Google Public DNS and only cache responses for example.org (or below).

~~~ corefile
. {
    proxy . 8.8.8.8:53
    cache example.org
}
~~~

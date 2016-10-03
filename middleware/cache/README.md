# cache

`cache` enables a frontend cache.

## Syntax

~~~ txt
cache [ttl] [zones...]
~~~

* `ttl` max TTL in seconds. If not specified, the maximum TTL will be used which is 1 hour for
    positive responses and half an hour for negative ones.
* `zones` zones it should cache for. If empty, the zones from the configuration block are used.

Each element in the cache is cached according to its TTL (with `ttl` as the max).
For the negative cache, the SOA's MinTTL value is used. A cache can contain up to 10,000 items by
default.

Or if you want more control:

~~~ txt
cache [ttl] [zones...] {
    postive capacity [ttl]
    negative capacity [ttl]
}
~~~

* `ttl`  and `zones` as above.
* `positive`, override the settings for caching positive responses, capacity indicates the maximum
  number of packets we cache before we start evicting (LRU). Ttl overrides the cache maximum TTL.
* `negative`, override the settings for caching negative responses, capacity indicates the maximum
  number of packets we cache before we start evicting (LRU). Ttl overrides the cache maximum TTL.

The minimum TTL allowed on resource records is 5 seconds.

If monitoring is enabled (via the `prometheus` directive) then the following extra metrics are added:
* coredns_cache_hit_count_total, and
* coredns_cache_miss_count_total

They both work on a per-zone basis and just count the hit and miss counts for each query.

## Examples

~~~
cache 10
~~~

Enable caching for all zones, but cap everything to a TTL of 10 seconds.

~~~
proxy . 8.8.8.8:53
cache example.org
~~~

Proxy to Google Public DNS and only cache responses for example.org (or below).

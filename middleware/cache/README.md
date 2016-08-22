# cache

`cache` enables a frontend cache.

## Syntax

~~~
cache [ttl] [zones...]
~~~

* `ttl` max TTL in seconds, if not specified the TTL of the reply (SOA minimum or minimum TTL in the
  answer section) will be used.
* `zones` zones it should should cache for. If empty the zones from the configuration block are used.

Each element in the cache is cached according to its TTL, for the negative cache the SOA's MinTTL
value is used.

A cache mostly makes sense with a middleware that is potentially slow, i.e. a proxy that retrieves
answer, or to minimize backend queries for middleware like etcd. Using a cache with the file
middleware essentially doubles the memory load with no conceivable increase of query speed.

The minimum TTL allowed on resource records is 5 seconds.

If monitoring is enabled (`prometheus` directive) then the following extra metrics are added:
* coredns_cache_hit_count_total, and
* coredns_cache_miss_count_total

They both work on a per zone basis and just count the hit and miss counts for each query.

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

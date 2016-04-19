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
middleware essentially doubles the memory load with no concealable increase of query speed.

## Examples

~~~
cache
~~~

Enable caching for all zones.

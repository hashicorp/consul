# loadbalance

*loadbalance* acts as a round-robin DNS loadbalancer by randomizing the order of A and AAAA records
 in the answer. See [Wikipedia](https://en.wikipedia.org/wiki/Round-robin_DNS) about the pros and
 cons on this setup.

It will take care to sort any CNAMEs before any address records, because some stub resolver
implementations (like glibc) are particular about that.

## Syntax

~~~
loadbalance [POLICY]
~~~

* **POLICY** is how to balance, the default is "round_robin"

## Examples

~~~
loadbalance round_robin
~~~

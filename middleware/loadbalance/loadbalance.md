# loadbalance

`loadbalance` acts as a round-robin DNS loadbalancer by randomizing A and AAAA records in the
message. See [Wikipedia](https://en.wikipedia.org/wiki/Round-robin_DNS) about the pros and cons
on this setup.

## Syntax

~~~
loadbalance [policy]
~~~

* policy is how to balance, the default is "round_robin"

## Examples

~~~
loadbalance round_robin
~~~

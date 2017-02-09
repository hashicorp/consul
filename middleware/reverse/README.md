# reverse

The *reverse* middleware allows CoreDNS to respond dynamic to an PTR request and the related A/AAAA request.

## Syntax

~~~
reverse NETWORK.. {
    hostname TEMPLATE
    [ttl TTL]
    [fallthrough]
~~~

* **NETWORK** one or more CIDR formatted networks to respond on.
* `hostname` inject the ip and zone to an template for the hostname. Defaults to "ip-{ip}.{zone[0]}". See below for template.
* `ttl` defaults to 60
* `fallthrough` If zone matches and no record can be generated, pass request to the next middleware.

### Template Syntax
The template for the hostname is used for generating the PTR for an reverse lookup and matching the forward lookup back to an ip.

#### `{ip}`
This symbol is **required** to work.
V4 network replaces the "." with an "-". 10.1.1.1 results in "10-1-1-1"
V6 network removes the ":" and fills the zeros. "ffff::ffff" results in "ffff000000000000000000000000ffff"

#### `{zone[i]}`
This symbol is **optional** to use and can be replaced by a fix zone string.
The zone will be matched by the configured listener on the server block key.
`i` needs to be replaced to the index of the configured listener zones, starting with 0.

`arpa.:53 domain.com.:8053` will resolve `zone{0}` to `arpa.` and `zone{1}` to `domain.com.`

## Examples

~~~
# Serve on port 53
# match arpa. and compute.internal. to resolv reverse and forward lookup 
.arpa.:53 compute.internal.:53 {
    # proxy unmatched requests
    proxy . 8.8.8.8

    # answer requests for IPs in this networks
    # PTR 1.0.32.10.in-addr.arpa. 3600 ip-10-0-32-1.compute.internal.
    # A ip-10-0-32-1.compute.internal. 3600 10.0.32.1
    # v6 is also possible
    # PTR 1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.1.0.d.f.ip6.arpa. 3600 ip-fd010000000000000000000000000001.compute.internal.
    # AAAA ip-fd010000000000000000000000000001.compute.internal. 3600 fd01::1
    reverse 10.32.0.0/16 fd01::/16 {
        # template of the ip injection to hostname, zone resolved to compute.internal.
        hostname ip-{ip}.{zone[1]}

        # set time-to-live of the RR
        ttl 3600

        # forward unanswered or unmatched requests to proxy
        # without this flag, requesting A/AAAA records on compute.internal. will end here
        fallthrough
    }

    # cache with ttl timeout
    cache
}
~~~


~~~
# Serve on port 53
# listen only on the specific network
32.10.in-addr.arpa.arpa.:53 arpa.company.org.:53 {

    reverse 10.32.0.0/16 {
        # template of the ip injection to hostname, zone resolved to arpa.company.org.
        hostname "ip-{ip}.v4.{zone[1]}"

        # set time-to-live of the RR
        ttl 3600

        # fallthrough is not required, v4.arpa.company.org. will be only answered here
    }

    # cidr closer to the ip wins, so we can overwrite the "default"
    reverse 10.32.2.0/24 {
        # its also possible to set fix domain suffix
        hostname ip-{ip}.fix.arpa.company.org.

        # set time-to-live of the RR
        ttl 3600
    }

    # cache with ttl timeout
    cache
}
~~~




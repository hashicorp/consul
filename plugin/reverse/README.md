# reverse

*reverse* allows for dynamic responses to PTR and the related A/AAAA requests.

## Syntax

~~~
reverse NETWORK... {
    hostname TEMPLATE
    [ttl TTL]
    [fallthrough]
    [wildcard]
~~~

* **NETWORK** one or more CIDR formatted networks to respond on.
* `hostname` injects the IP and zone to a template for the hostname. Defaults to "ip-{IP}.{zone[1]}". See below for template.
* `ttl` defaults to 60
* `fallthrough` if zone matches and no record can be generated, pass request to the next plugin.
* `wildcard` allows matches to catch all subdomains as well.

### Template Syntax

The template for the hostname is used for generating the PTR for a reverse lookup and matching the
forward lookup back to an IP.

#### `{ip}`

The `{ip}` symbol is **required** to make reverse work.
For IPv4 lookups the IP is directly extracted
With IPv6 lookups the ":" is removed, and any zero ranged are expanded, e.g.,
"ffff::ffff" results in "ffff000000000000000000000000ffff"

#### `{zone[i]}`

The `{zone[i]}` symbol is **optional** and can be replaced by a fixed (zone) string.
The zone will be matched by the zones listed in *this* configuration stanza.
`i` needs to be replaced with the index of the configured listener zones, starting with 1.

## Examples

~~~ corefile
arpa compute.internal {
    # proxy unmatched requests
    proxy . 8.8.8.8

    # answer requests for IPs in this network
    # PTR 1.0.32.10.in-addr.arpa. 3600 ip-10.0.32.1.compute.internal.
    # A ip-10.0.32.1.compute.internal. 3600 10.0.32.1
    # v6 is also possible
    # PTR 1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.1.0.d.f.ip6.arpa. 3600 ip-fd010000000000000000000000000001.compute.internal.
    # AAAA ip-fd010000000000000000000000000001.compute.internal. 3600 fd01::1
    reverse 10.32.0.0/16 fd01::/16 {
        # template of the ip injection to hostname, zone resolved to compute.internal.
        hostname ip-{ip}.{zone[2]}

        ttl 3600

        # Forward unanswered or unmatched requests to proxy
        # without this flag, requesting A/AAAA records on compute.internal. will end here.
        fallthrough
    }
}
~~~


~~~ corefile
32.10.in-addr.arpa.arpa arpa.company.org {

    reverse 10.32.0.0/16 {
        # template of the ip injection to hostname, zone resolved to arpa.company.org.
        hostname "ip-{ip}.v4.{zone[2]}"

        ttl 3600

        # fallthrough is not required, v4.arpa.company.org. will be only answered here
    }

    # cidr closer to the ip wins, so we can overwrite the "default"
    reverse 10.32.2.0/24 {
        # its also possible to set fix domain suffix
        hostname ip-{ip}.fix.arpa.company.org.

        ttl 3600
    }
}
~~~

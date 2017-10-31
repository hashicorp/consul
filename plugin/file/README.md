# file

*file* enables serving zone data from an RFC 1035-style master file.

The file plugin is used for an "old-style" DNS server. It serves from a preloaded file that exists
on disk. If the zone file contains signatures (i.e. is signed, i.e. DNSSEC) correct DNSSEC answers
are returned. Only NSEC is supported! If you use this setup *you* are responsible for resigning the
zonefile.

## Syntax

~~~
file DBFILE [ZONES...]
~~~

* **DBFILE** the database file to read and parse. If the path is relative the path from the *root*
  directive will be prepended to it.
* **ZONES** zones it should be authoritative for. If empty, the zones from the configuration block
    are used.

If you want to round robin A and AAAA responses look at the *loadbalance* plugin.

~~~
file DBFILE [ZONES... ] {
    transfer to ADDRESS...
    no_reload
    upstream ADDRESS...
}
~~~

* `transfer` enables zone transfers. It may be specified multiples times. `To` or `from` signals
  the direction. **ADDRESS** must be denoted in CIDR notation (127.0.0.1/32 etc.) or just as plain
  addresses. The special wildcard `*` means: the entire internet (only valid for 'transfer to').
  When an address is specified a notify message will be send whenever the zone is reloaded.
* `no_reload` by default CoreDNS will try to reload a zone every minute and reloads if the
  SOA's serial has changed. This option disables that behavior.
* `upstream` defines upstream resolvers to be used resolve external names found (think CNAMEs)
  pointing to external names. This is only really useful when CoreDNS is configured as a proxy, for
  normal authoritative serving you don't need *or* want to use this. **ADDRESS** can be an IP
  address, and IP:port or a string pointing to a file that is structured as /etc/resolv.conf.

## Examples

Load the `example.org` zone from `example.org.signed` and allow transfers to the internet, but send
notifies to 10.240.1.1

~~~ corefile
example.org {
    file example.org.signed {
        transfer to *
        transfer to 10.240.1.1
    }
}
~~~

Or use a single zone file for multiple zones:

~~~
. {
    file example.org.signed example.org example.net {
        transfer to *
        transfer to 10.240.1.1
    }
}
~~~

# file

## Name

*file* - enables serving zone data from an RFC 1035-style master file.

## Description

The file plugin is used for an "old-style" DNS server. It serves from a preloaded file that exists
on disk. If the zone file contains signatures (i.e., is signed using DNSSEC), correct DNSSEC answers
are returned. Only NSEC is supported! If you use this setup *you* are responsible for re-signing the
zonefile.

## Syntax

~~~
file DBFILE [ZONES...]
~~~

* **DBFILE** the database file to read and parse. If the path is relative, the path from the *root*
  directive will be prepended to it.
* **ZONES** zones it should be authoritative for. If empty, the zones from the configuration block
    are used.

If you want to round-robin A and AAAA responses look at the *loadbalance* plugin.

~~~
file DBFILE [ZONES... ] {
    transfer to ADDRESS...
    reload DURATION
    upstream
}
~~~

* `transfer` enables zone transfers. It may be specified multiples times. `To` or `from` signals
  the direction. **ADDRESS** must be denoted in CIDR notation (e.g., 127.0.0.1/32) or just as plain
  addresses. The special wildcard `*` means: the entire internet (only valid for 'transfer to').
  When an address is specified a notify message will be sent whenever the zone is reloaded.
* `reload` interval to perform a reload of the zone if the SOA version changes. Default is one minute.
  Value of `0` means to not scan for changes and reload. For example, `30s` checks the zonefile every 30 seconds
  and reloads the zone when serial changes.
* `upstream` resolve external names found (think CNAMEs) pointing to external names. This is only
  really useful when CoreDNS is configured as a proxy; for normal authoritative serving you don't
  need *or* want to use this. CoreDNS will resolve CNAMEs against itself.

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

Note that if you have a configuration like the following you may run into a problem of the origin
not being correctly recognized:

~~~
. {
    file db.example.org
}
~~~

We omit the origin for the file `db.example.org`, so this references the zone in the server block,
which, in this case, is the root zone. Any contents of `db.example.org` will then read with that
origin set; this may or may not do what you want.
It's better to be explicit here and specify the correct origin. This can be done in two ways:

~~~
. {
    file db.example.org example.org
}
~~~

Or

~~~
example.org {
    file db.example.org
}
~~~

# file

*file* enables serving zone data from an RFC 1035-style master file.

The file middleware is used for an "old-style" DNS server. It serves from a preloaded file that exists
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

If you want to round robin A and AAAA responses look at the *loadbalance* middleware.

TSIG key configuration is TODO; directive format for transfer will probably be extended with
TSIG key information, something like `transfer out [ADDRESS...] key [NAME[:ALG]] [BASE64]`

~~~
file DBFILE [ZONES... ] {
    transfer to ADDRESS...
    no_reload
}
~~~

* `transfer` enables zone transfers. It may be specified multiples times. `To` or `from` signals
  the direction. **ADDRESS** must be denoted in CIDR notation (127.0.0.1/32 etc.) or just as plain
  addresses. The special wildcard `*` means: the entire internet (only valid for 'transfer to').
  When an address is specified a notify message will be send whenever the zone is reloaded.
* `no_reload` by default CoreDNS will reload a zone from disk whenever it detects a change to the
  file. This option disables that behavior.

## Examples

Load the `example.org` zone from `example.org.signed` and allow transfers to the internet, but send
notifies to 10.240.1.1

~~~
file example.org.signed example.org {
    transfer to *
    transfer to 10.240.1.1
}
~~~

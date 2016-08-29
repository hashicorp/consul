# file

`file` enables serving zone data from an RFC 1035-style master file.

The file middleware is used for an "old-style" DNS server. It serves from a preloaded file that exists
on disk. If the zone file contains signatures (i.e. is signed, i.e. DNSSEC) correct DNSSEC answers
are returned. Only NSEC is supported! If you use this setup *you* are responsible for resigning the
zonefile.

## Syntax

~~~
file dbfile [zones...]
~~~

* `dbfile` the database file to read and parse.
* `zones` zones it should be authoritative for. If empty, the zones from the configuration block
    are used.

If you want to round robin A and AAAA responses look at the `loadbalance` middleware.

TSIG key configuration is TODO; directive format for transfer will probably be extended with
TSIG key information, something like `transfer out [address...] key [name] [base64]`

~~~
file dbfile [zones... ] {
    transfer from [address...]
    transfer to [address...]
    no_reload
}
~~~

* `transfer` enables zone transfers. It may be specified multiples times. *To* or *from* signals
    the direction. Addresses must be denoted in CIDR notation (127.0.0.1/32 etc.) or just as plain
    addresses. The special wildcard "*" means: the entire internet (only valid for 'transfer to').
* `no_reload` by default CoreDNS will reload a zone from disk whenever it detects a change to the
  file. This option disables that behavior.

## Examples

Load the `miek.nl` zone from `miek.nl.signed` and allow transfers to the internet.

~~~
file miek.nl.signed miek.nl {
    transfer to *
}
~~~

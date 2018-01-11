# auto

## Name

*auto* - enables serving zone data from an RFC 1035-style master file, which is automatically picked up from disk.

## Description

The *auto* plugin is used for an "old-style" DNS server. It serves from a preloaded file that exists
on disk. If the zone file contains signatures (i.e. is signed, i.e. using DNSSEC) correct DNSSEC answers
are returned. Only NSEC is supported! If you use this setup *you* are responsible for re-signing the
zonefile. New or changed zones are automatically picked up from disk.

## Syntax

~~~
auto [ZONES...] {
    directory DIR [REGEXP ORIGIN_TEMPLATE [TIMEOUT]]
    no_reload
    upstream ADDRESS...
}
~~~

**ZONES** zones it should be authoritative for. If empty, the zones from the configuration block
are used.

* `directory` loads zones from the speficied **DIR**. If a file name matches **REGEXP** it will be
  used to extract the origin. **ORIGIN_TEMPLATE** will be used as a template for the origin. Strings
  like `{<number>}` are replaced with the respective matches in the file name, e.g. `{1}` is the
  first match, `{2}` is the second. The default is: `db\.(.*)  {1}` i.e. from a file with the
  name `db.example.com`, the extracted origin will be `example.com`. **TIMEOUT** specifies how often
  CoreDNS should scan the directory; the default is every 60 seconds. This value is in seconds.
  The minimum value is 1 second.
* `no_reload` by default CoreDNS will try to reload a zone every minute and reloads if the
  SOA's serial has changed. This option disables that behavior.
* `upstream` defines upstream resolvers to be used resolve external names found (think CNAMEs)
  pointing to external names. **ADDRESS** can be an IP address, an IP:port or a string pointing to
  a file that is structured as /etc/resolv.conf.

All directives from the *file* plugin are supported. Note that *auto* will load all zones found,
even though the directive might only receive queries for a specific zone. I.e:

~~~ corefile
. {
    auto example.org {
        directory /etc/coredns/zones
    }
}
~~~
Will happily pick up a zone for `example.COM`, except it will never be queried, because the *auto*
directive only is authoritative for `example.ORG`.

## Examples

Load `org` domains from `/etc/coredns/zones/org` and allow transfers to the internet, but send
notifies to 10.240.1.1

~~~ corefile
. {
    auto org {
        directory /etc/coredns/zones/org
        transfer to *
        transfer to 10.240.1.1
    }
}
~~~

Load `org` domains from `/etc/coredns/zones/org` and looks for file names as `www.db.example.org`,
where `example.org` is the origin. Scan every 45 seconds.

~~~ corefile
org {
    auto {
        directory /etc/coredns/zones/org www\.db\.(.*) {1} 45
    }
}
~~~

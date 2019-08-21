# chaos

## Name

*chaos* - allows for responding to TXT queries in the CH class.

## Description

This is useful for retrieving version or author information from the server by querying a TXT record
for a special domain name in the CH class.

## Syntax

~~~
chaos [VERSION] [AUTHORS...]
~~~

* **VERSION** is the version to return. Defaults to `CoreDNS-<version>`, if not set.
* **AUTHORS** is what authors to return. This defaults to all GitHub handles in the OWNERS files.

Note that you have to make sure that this plugin will get actual queries for the
following zones: `version.bind`, `version.server`, `authors.bind`, `hostname.bind` and
`id.server`.

## Examples

Specify all the zones in full.

~~~ corefile
version.bind version.server authors.bind hostname.bind id.server {
    chaos CoreDNS-001 info@coredns.io
}
~~~

Or just default to `.`:

~~~ corefile
.  {
    chaos CoreDNS-001 info@coredns.io
}
~~~

And test with `dig`:

~~~ txt
% dig @localhost CH TXT version.bind
...
;; ANSWER SECTION:
version.bind.		0	CH	TXT	"CoreDNS-001"
...
~~~

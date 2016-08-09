# chaos

The `chaos` middleware allows CoreDNS to response to TXT queries in CH class.
Useful for retrieving version or author information from the server. If 

## Syntax

~~~
chaos [version] [authors...]
~~~

* `version` the version to return, defaults to CoreDNS.
* `authors` what authors to return. No default.

Note that you have to make sure that this middleware will get actual queries for the
following zones: `version.bind`, `version.server`, `authors.bind`, `hostname.bind` and
`id.server`.

## Examples

~~~
chaos CoreDNS-001 "Miek Gieben" miek@miek.nl
~~~
